package util

import (
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestGRPCServiceReadinessProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	if GRPCServiceReadinessProbe(listener.Addr().String()) {
		t.Fatal("non-serving gRPC endpoint reported ready")
	}

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if !GRPCServiceReadinessProbe(listener.Addr().String()) {
		t.Fatal("serving gRPC endpoint did not report ready")
	}
}
