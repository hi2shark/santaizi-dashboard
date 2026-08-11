package rpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	rpcService "github.com/hi2shark/santaizi-dashboard/service/rpc"
)

func ServeRPC(port uint) {
	server := grpc.NewServer()
	rpcService.SantaiziHandlerSingleton = rpcService.NewSantaiziHandler()
	pb.RegisterSantaiziServiceServer(server, rpcService.SantaiziHandlerSingleton)
	v2Handler, err := rpcService.NewV2Handler()
	if err != nil {
		panic(err)
	}
	pb.RegisterSantaiziTelemetryServiceServer(server, v2Handler)
	pb.RegisterSantaiziControlServiceServer(server, v2Handler)
	pb.RegisterSantaiziNATServiceServer(server, v2Handler)
	collectorHandler, err := rpcService.NewPrimaryCollectorHandler()
	if err != nil {
		panic(err)
	}
	pb.RegisterSantaiziCollectorServiceServer(server, collectorHandler)
	pb.RegisterSantaiziReplicationServiceServer(server, collectorHandler)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	_ = server.Serve(listen)
}

func DispatchMonitor(serviceSentinelDispatchBus <-chan model.Monitor) {
	for monitor := range serviceSentinelDispatchBus {
		rpcService.DispatchMonitor(monitor)
	}
}
