package rpc

import (
	"context"
	"fmt"
	"net"

	pb "github.com/hi2shark/santaizi-dashboard/proto"
	collectorservice "github.com/hi2shark/santaizi-dashboard/service/collector"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func ServeCollectorRPC(ctx context.Context, port uint, runtime *collectorservice.Runtime) error {
	server := grpc.NewServer()
	pb.RegisterSantaiziTelemetryServiceServer(server, runtime)
	pb.RegisterSantaiziCollectorServiceServer(server, runtime)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(pb.SantaiziTelemetryService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			healthServer.Shutdown()
			server.GracefulStop()
		case <-done:
		}
	}()
	err = server.Serve(listener)
	close(done)
	return err
}
