package cmd

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	rpc "github.com/longhorn/types/pkg/generated/imrpc"

	"github.com/longhorn/longhorn-instance-manager/pkg/health"
	"github.com/longhorn/longhorn-instance-manager/pkg/process"
	"github.com/longhorn/longhorn-instance-manager/pkg/proxy"
	"github.com/longhorn/longhorn-instance-manager/pkg/types"
	"github.com/longhorn/longhorn-instance-manager/pkg/util"
)

func getServiceAddresses(listen string) (map[string]string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil, err
	}

	basePort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		types.ProcessManagerGrpcService: net.JoinHostPort(host, strconv.Itoa(basePort)),
		types.ProxyGRPCService:          net.JoinHostPort(host, strconv.Itoa(basePort+1)),
		types.DiskGrpcService:           net.JoinHostPort(host, strconv.Itoa(basePort+2)),
		types.InstanceGrpcService:       net.JoinHostPort(host, strconv.Itoa(basePort+3)),
		types.SpdkGrpcService:           net.JoinHostPort(host, strconv.Itoa(basePort+4)),
	}, nil
}

func setupProxyGRPCServer(ctx context.Context, logsDir, listen, diskServiceAddress, spdkServiceAddress string, tlsConfig *tls.Config) (*grpc.Server, net.Listener, error) {
	srv, err := proxy.NewProxy(ctx, logsDir, diskServiceAddress, spdkServiceAddress)
	if err != nil {
		return nil, nil, err
	}
	hc := health.NewProxyHealthCheckServer(srv)

	server, listener, err := util.NewServer(listen, tlsConfig,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to setup %s", types.ProxyGRPCService)
	}

	rpc.RegisterProxyEngineServiceServer(server, srv)
	healthpb.RegisterHealthServer(server, hc)
	reflection.Register(server)
	return server, listener, nil
}

func setupProcessManagerGRPCServer(ctx context.Context, portRange, logsDir, listen string) (*process.Manager, *grpc.Server, net.Listener, error) {
	lifecycle, err := newProcessLifecycle(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	srv, err := process.NewManagerWithLifecycle(ctx, portRange, logsDir, lifecycle)
	if err != nil {
		return nil, nil, nil, err
	}
	hc := health.NewHealthCheckServer(srv)

	server, listener, err := util.NewServer(listen, nil,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to setup %s", types.ProcessManagerGrpcService)
	}

	rpc.RegisterProcessManagerServiceServer(server, srv)
	healthpb.RegisterHealthServer(server, hc)
	reflection.Register(server)
	return srv, server, listener, nil
}
