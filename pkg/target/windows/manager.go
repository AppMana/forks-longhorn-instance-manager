//go:build windows

package windows

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gostor/gotgt/pkg/api"
	"github.com/gostor/gotgt/pkg/config"
	"github.com/gostor/gotgt/pkg/port/iscsit"
	"github.com/gostor/gotgt/pkg/scsi"
	_ "github.com/gostor/gotgt/pkg/scsi/backingstore/remote"
	"github.com/sirupsen/logrus"

	lhLonghorn "github.com/longhorn/go-common-libs/longhorn"
	"github.com/longhorn/longhorn-engine/pkg/dataconn"
	enginetypes "github.com/longhorn/longhorn-engine/pkg/types"
	engineutil "github.com/longhorn/longhorn-engine/pkg/util"
	"github.com/longhorn/longhorn-instance-manager/pkg/process"
)

const engineDataPortOffset = 1

type Manager struct {
	mu      sync.Mutex
	driver  *iscsit.ISCSITargetDriver
	portal  string
	targets map[string]*target
}

type target struct {
	name    string
	device  uint64
	backend *switchableBackingStore
}

type switchableBackingStore struct {
	mu     sync.RWMutex
	active *engineBackend
}

type engineBackend struct {
	client *dataconn.Client
}

func NewManager(ctx context.Context, portal string) (*Manager, error) {
	listener, err := net.Listen("tcp", portal)
	if err != nil {
		return nil, fmt.Errorf("listen for shared Windows iSCSI target on %s: %w", portal, err)
	}
	service := scsi.NewSCSITargetService()
	driverInterface, err := scsi.NewTargetDriver("iscsi", service)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	driver := driverInterface.(*iscsit.ISCSITargetDriver)
	m := &Manager{driver: driver, portal: listener.Addr().String(), targets: map[string]*target{}}
	go func() {
		if err := driver.RunListener(listener); err != nil {
			logrus.WithError(err).Error("Windows iSCSI target service stopped")
		}
	}()
	go func() {
		<-ctx.Done()
		_ = driver.Close()
	}()
	return m, nil
}

func (m *Manager) Started(p *process.Process) error {
	if !isISCSIEngine(p) {
		return nil
	}
	info, err := parseEngine(p)
	if err != nil {
		return err
	}
	backend, err := dialEngine(p.PortStart + engineDataPortOffset)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing := m.targets[p.Name]; existing != nil {
		// Replacement processes emit the same running update before ProcessReplace
		// reaches its ordered switchover point. Never switch from this observer;
		// only SwitchOver is allowed to move a live target.
		backend.Close()
		return nil
	}
	store := &switchableBackingStore{active: backend}
	storage := config.BackendStorage{
		DeviceID:         info.deviceID,
		Path:             "RemBs:" + info.targetName,
		Online:           true,
		ThinProvisioning: true,
		BlockShift:       info.blockShift,
		BackendType:      "RemBs",
		DeviceSize:       uint64(info.size),
	}
	if err := scsi.InitSCSILUMapEx(&storage, info.targetName, 0, store); err != nil {
		backend.Close()
		return err
	}
	configuration := &config.Config{
		ISCSIPortals: []config.ISCSIPortalInfo{{ID: 0, Portal: m.portal}},
		ISCSITargets: map[string]config.ISCSITarget{
			info.targetName: {TPGTs: map[string][]uint64{"1": {0}}, LUNs: map[string]uint64{"0": info.deviceID}},
		},
	}
	if err := m.driver.NewTarget(info.targetName, configuration); err != nil {
		backend.Close()
		scsi.DelTargetLUNMap(info.targetName)
		return err
	}
	m.targets[p.Name] = &target{name: info.targetName, device: info.deviceID, backend: store}
	logrus.Infof("Windows iSCSI target %s now serves engine %s on shared portal %s", info.targetName, p.Name, m.portal)
	return nil
}

func (m *Manager) SwitchOver(oldProcess, newProcess *process.Process) error {
	if !isISCSIEngine(newProcess) {
		return nil
	}
	backend, err := dialEngine(newProcess.PortStart + engineDataPortOffset)
	if err != nil {
		return err
	}
	m.mu.Lock()
	t := m.targets[oldProcess.Name]
	m.mu.Unlock()
	if t == nil {
		backend.Close()
		return fmt.Errorf("Windows iSCSI target for engine %s is not active", oldProcess.Name)
	}
	// switchTo takes an exclusive lock after the new backend has answered Ping.
	// Consequently all old-backend I/O drains before ProcessReplace can signal
	// the old engine, while the initiator session and target object stay intact.
	t.backend.switchTo(backend)
	return nil
}

func (m *Manager) Deleted(p *process.Process) error {
	if !isISCSIEngine(p) {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.targets[p.Name]
	if t == nil {
		return nil
	}
	// CSI disconnects the initiator before engine teardown. Refuse deletion if
	// a session is still present so we never close a backing store under live I/O.
	if err := m.driver.DeleteTarget(t.name, false); err != nil {
		return err
	}
	t.backend.Close()
	delete(m.targets, p.Name)
	return nil
}

func (s *switchableBackingStore) switchTo(next *engineBackend) {
	s.mu.Lock()
	previous := s.active
	s.active = next
	s.mu.Unlock()
	if previous != nil {
		previous.Close()
	}
}

func (s *switchableBackingStore) Close() {
	s.mu.Lock()
	active := s.active
	s.active = nil
	s.mu.Unlock()
	if active != nil {
		active.Close()
	}
}

func (s *switchableBackingStore) ReadAt(p []byte, off int64) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return 0, fmt.Errorf("engine backend is unavailable")
	}
	return s.active.client.ReadAt(p, off)
}

func (s *switchableBackingStore) WriteAt(p []byte, off int64) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return 0, fmt.Errorf("engine backend is unavailable")
	}
	return s.active.client.WriteAt(p, off)
}

func (s *switchableBackingStore) Sync() (int, error) { return 0, nil }

func (s *switchableBackingStore) Unmap(offset, length int64) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return 0, fmt.Errorf("engine backend is unavailable")
	}
	total := 0
	for length > 0 {
		chunk := length
		if chunk > math.MaxUint32 {
			chunk = math.MaxUint32
		}
		n, err := s.active.client.UnmapAt(uint32(chunk), offset)
		total += n
		if err != nil {
			return total, err
		}
		offset += chunk
		length -= chunk
	}
	return total, nil
}

func (b *engineBackend) Close() { b.client.Close() }

func dialEngine(port int32) (*engineBackend, error) {
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect replacement engine data endpoint %s: %w", address, err)
	}
	client := dataconn.NewClient([]net.Conn{conn}, engineutil.NewSharedTimeouts(8*time.Second, 16*time.Second))
	if err := client.Ping(); err != nil {
		client.Close()
		return nil, fmt.Errorf("probe replacement engine data endpoint %s: %w", address, err)
	}
	return &engineBackend{client: client}, nil
}

type engineInfo struct {
	targetName string
	deviceID   uint64
	size       int64
	blockShift uint
}

func parseEngine(p *process.Process) (*engineInfo, error) {
	volume, sizeText := "", ""
	for i, arg := range p.Args {
		if arg == "controller" && i+1 < len(p.Args) {
			volume = p.Args[i+1]
		}
		if arg == "--size" && i+1 < len(p.Args) {
			sizeText = p.Args[i+1]
		}
	}
	if volume == "" || sizeText == "" {
		return nil, fmt.Errorf("engine %s arguments do not contain controller volume and size", p.Name)
	}
	size, err := strconv.ParseInt(sizeText, 10, 64)
	if err != nil || size <= 0 {
		return nil, fmt.Errorf("engine %s has invalid size %q", p.Name, sizeText)
	}
	targetName := "iqn.2019-10.io.longhorn:" + engineutil.Volume2ISCSIName(volume)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(targetName))
	return &engineInfo{targetName: targetName, deviceID: hash.Sum64(), size: size, blockShift: 9}, nil
}

func isISCSIEngine(p *process.Process) bool {
	if !lhLonghorn.IsEngineProcess(p.Name) {
		return false
	}
	for i, arg := range p.Args {
		if arg == "--frontend" && i+1 < len(p.Args) {
			return p.Args[i+1] == enginetypes.EngineFrontendISCSI
		}
	}
	return false
}

var _ process.Lifecycle = (*Manager)(nil)
var _ api.RemoteBackingStore = (*switchableBackingStore)(nil)
