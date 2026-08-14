package rpc

import (
	"bytes"
	"context"
	"time"

	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/pki"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EnrollmentHandler struct {
	pb.UnimplementedSantaiziEnrollmentServiceServer
	auth    *authHandler
	agentCA *pki.Authority
}

func NewEnrollmentHandler(agentCA *pki.Authority) *EnrollmentHandler {
	return &EnrollmentHandler{auth: &authHandler{}, agentCA: agentCA}
}

func (h *EnrollmentHandler) Enroll(ctx context.Context, request *pb.AgentEnrollRequest) (*pb.AgentEnrollResponse, error) {
	if !pki.ConnectionIsTLS(ctx) {
		return nil, status.Error(codes.Unauthenticated, "enrollment requires TLS")
	}
	serverID, err := h.auth.Check(ctx)
	if err != nil {
		return nil, err
	}
	return h.issueAgentCertificate(ctx, serverID, request.GetNodeUuid(), request.GetCsrDer(), true)
}

func (h *EnrollmentHandler) Renew(ctx context.Context, request *pb.AgentRenewRequest) (*pb.AgentEnrollResponse, error) {
	ident, hasCert, err := pki.PeerDeviceIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if !hasCert || ident.Kind != pki.DeviceAgent {
		return nil, status.Error(codes.Unauthenticated, "agent certificate is required")
	}
	if !bytes.Equal(ident.NodeUUID, request.GetNodeUuid()) {
		return nil, status.Error(codes.PermissionDenied, "certificate UUID does not match request")
	}
	serverID, err := singleton.ServerIDFromNodeUUID(ident.NodeUUID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "node is not bound to a server")
	}
	return h.issueAgentCertificate(ctx, serverID, request.GetNodeUuid(), request.GetCsrDer(), false)
}

func (h *EnrollmentHandler) issueAgentCertificate(ctx context.Context, serverID uint64, nodeUUID, csrDER []byte, bind bool) (*pb.AgentEnrollResponse, error) {
	if len(nodeUUID) != 16 {
		return nil, status.Error(codes.InvalidArgument, "node UUID must be 16 bytes")
	}
	if len(csrDER) == 0 {
		return nil, status.Error(codes.InvalidArgument, "csr_der is required")
	}
	if bind {
		if err := singleton.EnsureServerNodeAvailableForEnroll(serverID, nodeUUID); err != nil {
			if singleton.IsServerBoundToOtherNode(err) {
				return nil, status.Error(codes.FailedPrecondition, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if _, err := singleton.BindServerNodeForProtocol(serverID, nodeUUID, time.Now(), pb.SourceProtocol_SOURCE_PROTOCOL_SANTAIZI_V2); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	certPEM, notBefore, notAfter, err := pki.SignAgentCSR(h.agentCA, csrDER, nodeUUID, time.Now())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &pb.AgentEnrollResponse{
		CertificatePem:   string(certPEM),
		CaCertificatePem: string(h.agentCA.CertPEM),
		NotBeforeUnix:    notBefore.Unix(),
		ExpiresAtUnix:    notAfter.Unix(),
	}, nil
}
