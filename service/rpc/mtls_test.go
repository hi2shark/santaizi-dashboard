package rpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/service/pki"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestAuthorizeDeviceAllowsBootstrapWithoutCert(t *testing.T) {
	ctx := context.Background()
	policy := DeviceAuthPolicy{}
	for _, method := range []string{methodEnroll, methodRegister, methodGetStatus, "/grpc.health.v1.Health/Check"} {
		if err := AuthorizeDevice(ctx, method, policy); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
}

func TestAuthorizeDeviceStrictRejectsMissingCert(t *testing.T) {
	ctx := context.Background()
	policy := DeviceAuthPolicy{RequireAgentMTLS: true, RequireCollectorMTLS: true}
	if err := AuthorizeDevice(ctx, methodControl, policy); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("control=%v", err)
	}
	if err := AuthorizeDevice(ctx, methodIngest, policy); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ingest=%v", err)
	}
	if err := AuthorizeDevice(ctx, methodSync, policy); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sync=%v", err)
	}
	if err := AuthorizeDevice(ctx, methodReplicate, policy); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("replicate=%v", err)
	}
}

func TestAuthorizeDeviceRejectsWrongKind(t *testing.T) {
	bundle, err := pki.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	agentCert := mustSignAgent(t, bundle)
	collectorCert := mustSignCollector(t, bundle, "collector-a")
	policy := DeviceAuthPolicy{}
	if err := AuthorizeDevice(contextWithCert(t, collectorCert), methodControl, policy); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("collector cert on control: %v", err)
	}
	if err := AuthorizeDevice(contextWithCert(t, agentCert), methodSync, policy); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("agent cert on sync: %v", err)
	}
	if err := AuthorizeDevice(contextWithCert(t, agentCert), methodControl, policy); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeDeviceRenewAlwaysRequiresCert(t *testing.T) {
	if err := AuthorizeDevice(context.Background(), methodRenew, DeviceAuthPolicy{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("renew: %v", err)
	}
	if err := AuthorizeDevice(context.Background(), methodRenewCollector, DeviceAuthPolicy{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("renew collector: %v", err)
	}
}

func TestAuthorizeDeviceForceAgentIngest(t *testing.T) {
	if err := AuthorizeDevice(context.Background(), methodIngest, DeviceAuthPolicy{ForceAgentIngest: true}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("force ingest: %v", err)
	}
}

func TestMatchAgentCertificateRequiresMatchingUUID(t *testing.T) {
	bundle, err := pki.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cert := mustSignAgent(t, bundle)
	ctx := contextWithCert(t, cert)
	node := bytes.Repeat([]byte{0x11}, 16)
	if err := matchAgentCertificate(ctx, node, node); err != nil {
		t.Fatal(err)
	}
	other := bytes.Repeat([]byte{0x99}, 16)
	if err := matchAgentCertificate(ctx, other, node); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("hello mismatch: %v", err)
	}
	if err := matchAgentCertificate(ctx, node, other); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("credential mismatch: %v", err)
	}
}

func contextWithCert(t *testing.T, cert *x509.Certificate) context.Context {
	t.Helper()
	return peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{cert}}}},
	})
}

func mustSignAgent(t *testing.T, bundle *pki.Bundle) *x509.Certificate {
	t.Helper()
	key, err := pki.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{0x11}, 16)
	csr, err := pki.CreateCSR(key, pki.EncodeAgentURI(node))
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, _, err := pki.SignAgentCSR(bundle.Agent, csr, node, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cert, err := pki.ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func mustSignCollector(t *testing.T, bundle *pki.Bundle, uuid string) *x509.Certificate {
	t.Helper()
	key, err := pki.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := pki.CreateCSR(key, pki.EncodeCollectorURI(uuid))
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, _, err := pki.SignCollectorCSR(bundle.Collector, csr, uuid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cert, err := pki.ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
