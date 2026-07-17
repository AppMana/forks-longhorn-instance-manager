//go:build windows

package cmd

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	rpc "github.com/longhorn/types/pkg/generated/imrpc"

	"github.com/longhorn/longhorn-instance-manager/pkg/process"
	"github.com/longhorn/longhorn-instance-manager/pkg/types"
	"github.com/longhorn/longhorn-instance-manager/pkg/util"
)

func StartCmd() cli.Command {
	return cli.Command{
		Name: "daemon",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "listen", Value: "0.0.0.0:8500"},
			cli.StringFlag{Name: "logs-dir", Value: `C:\var\log\longhorn\instances`},
			cli.StringFlag{Name: "port-range", Value: "10000-20000"},
			// Accepted for manifest parity. Windows V1 intentionally does not load SPDK.
			cli.StringFlag{Name: "spdk-port-range", Value: "20001-30000"},
			cli.BoolFlag{Name: "spdk-enabled"},
		},
		Action: func(c *cli.Context) {
			if c.Bool("spdk-enabled") {
				logrus.Fatal("the Windows instance manager supports the V1 data engine only")
			}
			if err := startWindows(c); err != nil {
				logrus.WithError(err).Fatal("Failed to run Windows instance manager")
			}
		},
	}
}

func startWindows(c *cli.Context) error {
	logsDir := c.String("logs-dir")
	if err := util.SetUpLogger(logsDir); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addresses, err := getServiceAddresses(c.String("listen"))
	if err != nil {
		return err
	}

	manager, processServer, processListener, err := setupProcessManagerGRPCServer(ctx, c.String("port-range"), logsDir, addresses[types.ProcessManagerGrpcService])
	if err != nil {
		return err
	}

	var tlsConfig *tls.Config
	if tlsDir := c.GlobalString("tls-dir"); tlsDir != "" {
		tlsDir = util.ResolveContainerMountPath(tlsDir)
		tlsConfig, err = util.LoadServerTLS(
			filepath.Join(tlsDir, "ca.crt"), filepath.Join(tlsDir, "tls.crt"), filepath.Join(tlsDir, "tls.key"),
			"longhorn-backend.longhorn-system")
		if err != nil {
			logrus.WithError(err).Warnf("Failed to add TLS key pair from %v", tlsDir)
		}
	}
	if tlsConfig != nil {
		logrus.Info("Creating Windows proxy gRPC server with mTLS auth")
	} else {
		logrus.Info("Creating Windows proxy gRPC server with no auth")
	}
	proxyServer, proxyListener, err := setupProxyGRPCServer(ctx, logsDir,
		addresses[types.ProxyGRPCService], addresses[types.DiskGrpcService], addresses[types.SpdkGrpcService], tlsConfig)
	if err != nil {
		return err
	}

	servers := map[string]*grpc.Server{
		types.ProcessManagerGrpcService: processServer,
		types.ProxyGRPCService:          proxyServer,
	}
	listeners := map[string]net.Listener{
		types.ProcessManagerGrpcService: processListener,
		types.ProxyGRPCService:          proxyListener,
	}
	return serveWindowsV1Services(ctx, cancel, manager, servers, listeners)
}

func serveWindowsV1Services(ctx context.Context, cancel context.CancelFunc, manager *process.Manager, servers map[string]*grpc.Server, listeners map[string]net.Listener) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		select {
		case <-signals:
			logrus.Info("Windows instance manager received interrupt")
		case <-groupCtx.Done():
		}
		cancel()
		for _, server := range servers {
			server.Stop()
		}
		return nil
	})

	for name, server := range servers {
		name, server := name, server
		group.Go(func() error {
			listener := listeners[name]
			logrus.Infof("%s listening on %s", name, listener.Addr())
			err := server.Serve(listener)
			cancel()
			if name == types.ProcessManagerGrpcService {
				cleanupWindowsProcesses(manager)
			}
			return err
		})
	}

	logrus.Info("Windows V1 instance manager started; shared iSCSI target listening on :3260")
	return group.Wait()
}

func cleanupWindowsProcesses(manager *process.Manager) {
	response, err := manager.ProcessList(context.Background(), &rpc.ProcessListRequest{})
	if err != nil {
		return
	}
	for _, item := range response.Processes {
		_, _ = manager.ProcessDelete(context.Background(), &rpc.ProcessDeleteRequest{Name: item.Spec.Name})
	}
}
