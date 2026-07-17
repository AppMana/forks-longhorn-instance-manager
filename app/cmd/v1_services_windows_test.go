//go:build windows

package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/longhorn/longhorn-instance-manager/pkg/types"
)

func TestWindowsV1ProxyService(t *testing.T) {
	listen := reserveAdjacentPorts(t)
	addresses, err := getServiceAddresses(listen)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proxyServer, proxyListener, err := setupProxyGRPCServer(ctx, t.TempDir(), addresses[types.ProxyGRPCService], addresses[types.DiskGrpcService], addresses[types.SpdkGrpcService], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer proxyServer.Stop()

	serveErrors := make(chan error, 1)
	go func() { serveErrors <- proxyServer.Serve(proxyListener) }()

	checkHealth(t, addresses[types.ProxyGRPCService])
}

func reserveAdjacentPorts(t *testing.T) string {
	t.Helper()
	for attempt := 0; attempt < 50; attempt++ {
		baseListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		basePort := baseListener.Addr().(*net.TCPAddr).Port
		_ = baseListener.Close()
		if basePort >= 65535 {
			continue
		}
		proxyListener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(basePort+1)))
		if err != nil {
			continue
		}
		_ = proxyListener.Close()
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(basePort))
	}
	t.Fatal("failed to reserve adjacent process and proxy ports")
	return ""
}

func checkHealth(t *testing.T, address string) {
	t.Helper()
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check for %s failed: %v", address, err)
	}
	if response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatal(fmt.Errorf("service %s reported %s", address, response.Status))
	}
}
