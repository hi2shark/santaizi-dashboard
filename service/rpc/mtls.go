package rpc

import (
	"context"
	"strings"

	"github.com/hi2shark/santaizi-dashboard/service/pki"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	methodEnroll         = "/proto.SantaiziEnrollmentService/Enroll"
	methodRenew          = "/proto.SantaiziEnrollmentService/Renew"
	methodRegister       = "/proto.SantaiziCollectorService/Register"
	methodSync           = "/proto.SantaiziCollectorService/Sync"
	methodGetStatus      = "/proto.SantaiziCollectorService/GetStatus"
	methodRenewCollector = "/proto.SantaiziCollectorService/RenewCollector"
	methodControl        = "/proto.SantaiziControlService/Control"
	methodIngest         = "/proto.SantaiziTelemetryService/Ingest"
	methodNAT            = "/proto.SantaiziNATService/NATStream"
	methodReplicate      = "/proto.SantaiziReplicationService/Replicate"
)

type DeviceAuthPolicy struct {
	RequireAgentMTLS     bool
	RequireCollectorMTLS bool
	ForceAgentIngest     bool
}

func UnaryDeviceAuth(policy DeviceAuthPolicy) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := AuthorizeDevice(ctx, info.FullMethod, policy); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func StreamDeviceAuth(policy DeviceAuthPolicy) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := AuthorizeDevice(ss.Context(), info.FullMethod, policy); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func AuthorizeDevice(ctx context.Context, fullMethod string, policy DeviceAuthPolicy) error {
	if strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/") {
		return nil
	}
	switch fullMethod {
	case methodEnroll, methodRegister, methodGetStatus:
		return nil
	}

	ident, hasCert, err := pki.PeerDeviceIdentityFromContext(ctx)
	if hasCert && err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	needAgent := fullMethod == methodRenew ||
		(policy.RequireAgentMTLS && isAgentMethod(fullMethod)) ||
		(policy.ForceAgentIngest && fullMethod == methodIngest)
	needCollector := fullMethod == methodRenewCollector ||
		(policy.RequireCollectorMTLS && isCollectorMethod(fullMethod))

	if needAgent {
		if !hasCert {
			return status.Error(codes.Unauthenticated, "agent certificate is required")
		}
		if ident.Kind != pki.DeviceAgent {
			return status.Error(codes.PermissionDenied, "agent certificate is required")
		}
		return nil
	}
	if needCollector {
		if !hasCert {
			return status.Error(codes.Unauthenticated, "collector certificate is required")
		}
		if ident.Kind != pki.DeviceCollector {
			return status.Error(codes.PermissionDenied, "collector certificate is required")
		}
		return nil
	}

	if hasCert {
		if isAgentMethod(fullMethod) && ident.Kind != pki.DeviceAgent {
			return status.Error(codes.PermissionDenied, "agent certificate is required")
		}
		if isCollectorMethod(fullMethod) && ident.Kind != pki.DeviceCollector {
			return status.Error(codes.PermissionDenied, "collector certificate is required")
		}
	}
	return nil
}

func isAgentMethod(fullMethod string) bool {
	switch fullMethod {
	case methodControl, methodIngest, methodNAT, methodRenew:
		return true
	default:
		return false
	}
}

func isCollectorMethod(fullMethod string) bool {
	switch fullMethod {
	case methodSync, methodReplicate, methodRenewCollector:
		return true
	default:
		return false
	}
}
