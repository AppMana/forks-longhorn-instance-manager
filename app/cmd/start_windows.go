//go:build windows

package cmd

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	grpcHealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	rpc "github.com/longhorn/types/pkg/generated/imrpc"

	"github.com/longhorn/longhorn-instance-manager/pkg/process"
	"github.com/longhorn/longhorn-instance-manager/pkg/util"
)

func StartCmd() cli.Command {
	return cli.Command{
		Name: "daemon",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "listen", Value: "localhost:8500"},
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
	lifecycle, err := newProcessLifecycle(ctx)
	if err != nil {
		return err
	}
	manager, err := process.NewManagerWithLifecycle(ctx, c.String("port-range"), logsDir, lifecycle)
	if err != nil {
		return err
	}

	var serverOptions []grpc.ServerOption
	serverOptions = append(serverOptions, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime: 10 * time.Second, PermitWithoutStream: true,
	}))
	if tlsDir := c.GlobalString("tls-dir"); tlsDir != "" {
		tlsConfig, err := util.LoadServerTLS(
			filepath.Join(tlsDir, "ca.crt"), filepath.Join(tlsDir, "tls.crt"), filepath.Join(tlsDir, "tls.key"),
			"longhorn-backend.longhorn-system")
		if err != nil {
			return errors.Wrap(err, "load instance-manager TLS configuration")
		}
		grpcServer, listener, err := util.NewServer(c.String("listen"), tlsConfig, serverOptions...)
		if err != nil {
			return err
		}
		return serveWindowsProcessManager(ctx, cancel, manager, grpcServer, listener)
	}
	grpcServer, listener, err := util.NewServer(c.String("listen"), nil, serverOptions...)
	if err != nil {
		return err
	}
	return serveWindowsProcessManager(ctx, cancel, manager, grpcServer, listener)
}

func serveWindowsProcessManager(ctx context.Context, cancel context.CancelFunc, manager *process.Manager, server *grpc.Server, listener net.Listener) error {
	rpc.RegisterProcessManagerServiceServer(server, manager)
	healthServer := grpcHealth.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	reflection.Register(server)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		<-signals
		cancel()
		server.GracefulStop()
	}()
	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()
	logrus.Infof("Windows V1 process manager listening on %s; shared iSCSI target listening on :3260", listener.Addr())
	err := server.Serve(listener)
	cleanupWindowsProcesses(manager)
	return err
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
