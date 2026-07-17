package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"k8s.io/mount-utils"

	spdkhelpertypes "github.com/longhorn/go-spdk-helper/pkg/types"
)

const (
	DefaulCmdTimeout              = time.Minute // one minute by default
	containerSandboxMountPointEnv = "CONTAINER_SANDBOX_MOUNT_POINT"
)

// ResolveContainerMountPath returns the path at which a process can consume a
// Kubernetes volume mount. Windows HostProcess containers on containerd 1.6
// expose mounts beneath CONTAINER_SANDBOX_MOUNT_POINT only. Newer releases
// retain this layout in addition to their direct bind mounts.
func ResolveContainerMountPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}

	sandbox := os.Getenv(containerSandboxMountPointEnv)
	if sandbox == "" {
		return path
	}

	cleanPath := filepath.Clean(path)
	if relative, err := filepath.Rel(sandbox, cleanPath); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return cleanPath
	}

	cleanPath = strings.TrimPrefix(cleanPath, filepath.VolumeName(cleanPath))
	cleanPath = strings.TrimLeft(cleanPath, `/\`)
	return filepath.Join(sandbox, cleanPath)
}

func Execute(binary string, args ...string) (string, error) {
	return ExecuteWithTimeout(DefaulCmdTimeout, binary, args...)
}

func ExecuteWithTimeout(timeout time.Duration, binary string, args ...string) (string, error) {
	var err error
	cmd := exec.Command(binary, args...)
	done := make(chan struct{})

	var output, stderr bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &stderr

	go func() {
		err = cmd.Run()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				logrus.WithError(err).Warnf("Problem killing process pid=%v", cmd.Process.Pid)
			}

		}
		return "", errors.Wrapf(err, "timeout executing: %v %v, output %s, stderr %s",
			binary, args, output.String(), stderr.String())
	}

	if err != nil {
		return "", errors.Wrapf(err, "failed to execute: %v %v, output %s, stderr %s",
			binary, args, output.String(), stderr.String())
	}
	return output.String(), nil
}

func PrintJSON(obj interface{}) error {
	output, err := json.MarshalIndent(obj, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}

func GetURL(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func RemoveFile(file string) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// file doesn't exist
		return nil
	}

	if _, err := Execute("rm", file); err != nil {
		return errors.Wrapf(err, "failed to remove file %v", file)
	}

	return nil
}

func GRPCServiceReadinessProbe(address string) bool {
	// Keep the readiness check in-process so every supported host OS gets the
	// same behavior. In particular, Windows HostProcess images do not contain
	// the Linux /usr/local/bin/grpc_health_probe helper.
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err == nil && response.GetStatus() == healthpb.HealthCheckResponse_SERVING
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func UUID() string {
	return uuid.New().String()
}

func ParsePortRange(portRange string) (int32, int32, error) {
	parts := strings.Split(portRange, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid format for SPDK port range %s", portRange)
	}

	portStart, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}

	portEnd, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}

	return int32(portStart), int32(portEnd), nil
}

// IsSPDKTgtReady checks if SPDK target is ready
func IsSPDKTgtReady(timeout time.Duration) bool {
	for i := 0; i < int(timeout.Seconds()); i++ {
		conn, err := net.DialTimeout(spdkhelpertypes.DefaultJSONServerNetwork, spdkhelpertypes.DefaultUnixDomainSocketPath, 1*time.Second)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				logrus.WithError(closeErr).Warn("Failed to close connection")
			}
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func IsMountPointReadOnly(mp mount.MountPoint) bool {
	for _, opt := range mp.Opts {
		if opt == "ro" {
			return true
		}
	}
	return false
}

func GetVolumeMountPointMap() (map[string]mount.MountPoint, error) {
	volumeMountPointMap := make(map[string]mount.MountPoint)

	mounter := mount.New("")
	mountPoints, err := mounter.List()
	if err != nil {
		return nil, err
	}

	regex := regexp.MustCompile(`.*/globalmount$`)

	for _, mp := range mountPoints {
		if regex.MatchString(mp.Path) {
			volumeNameSHAStr := GetVolumeNameSHAStrFromPath(mp.Path)
			volumeMountPointMap[volumeNameSHAStr] = mp
		}
	}
	return volumeMountPointMap, nil
}

func GetVolumeNameSHAStrFromPath(path string) string {
	// mount path for volume: "/host/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/${VolumeNameSHAStr}/globalmount"
	pathSlices := strings.Split(path, "/")
	volumeNameSHAStr := pathSlices[len(pathSlices)-2]
	return volumeNameSHAStr
}

func ProcessNameToVolumeName(processName string) string {
	// process name: "pvc-e130e369-274d-472d-98d1-f6074d2725e8-e-0"
	nameSlices := strings.Split(processName, "-")
	volumeName := strings.Join(nameSlices[:len(nameSlices)-2], "-")
	return volumeName
}
