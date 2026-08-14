package pki

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type ServerTLSOptions struct {
	CertFile          string
	KeyFile           string
	ClientCAFile      string
	AgentCA           *x509.Certificate
	CollectorCA       *x509.Certificate
	ClientCAsProvider func() *x509.CertPool
}

func GRPCServerTLS(opts ServerTLSOptions) (credentials.TransportCredentials, error) {
	if opts.CertFile == "" || opts.KeyFile == "" {
		return nil, errors.New("grpc_tls.enabled requires cert_file and key_file")
	}
	certificate, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile) // #nosec G304 -- operator-configured server certificate
	if err != nil {
		return nil, fmt.Errorf("load gRPC server certificate: %w", err)
	}
	staticPool, err := buildClientCAPool(opts)
	if err != nil {
		return nil, err
	}
	base := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    staticPool,
	}
	if opts.ClientCAsProvider != nil {
		base.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
			pool := opts.ClientCAsProvider()
			if pool == nil {
				pool = staticPool
			}
			return &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{certificate},
				ClientAuth:   tls.VerifyClientCertIfGiven,
				ClientCAs:    pool,
			}, nil
		}
	}
	return credentials.NewTLS(base), nil
}

func buildClientCAPool(opts ServerTLSOptions) (*x509.CertPool, error) {
	pool := CertPool(opts.AgentCA, opts.CollectorCA)
	if opts.ClientCAFile == "" {
		return pool, nil
	}
	pemBytes, err := os.ReadFile(opts.ClientCAFile) // #nosec G304 -- operator-configured extra client CA bundle
	if err != nil {
		return nil, fmt.Errorf("read grpc_tls.client_ca_file: %w", err)
	}
	if err := AppendPEMToPool(pool, pemBytes); err != nil {
		return nil, fmt.Errorf("grpc_tls.client_ca_file: %w", err)
	}
	return pool, nil
}

func PeerDeviceIdentityFromContext(ctx context.Context) (*DeviceIdentity, bool, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return nil, false, nil
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, false, nil
	}
	if len(info.State.VerifiedChains) == 0 || len(info.State.VerifiedChains[0]) == 0 {
		return nil, false, nil
	}
	identity, err := ParseDeviceIdentityFromCertificate(info.State.VerifiedChains[0][0])
	if err != nil {
		return nil, true, err
	}
	return identity, true, nil
}

func ConnectionIsTLS(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return false
	}
	_, ok = p.AuthInfo.(credentials.TLSInfo)
	return ok
}

// NewSelfSignedServerCertificate issues a server certificate that is independent
// of the device CAs. Tests and local fixtures must not sign server certs with
// Agent CA or Collector CA.
func NewSelfSignedServerCertificate(dnsNames []string, now time.Time, validity time.Duration) (tls.Certificate, []byte, []byte, error) {
	if validity <= 0 {
		validity = DefaultCertificateValidity
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	if len(dnsNames) == 0 {
		dnsNames = []string{"localhost"}
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsNames[0], Organization: []string{"Santaizi"}},
		NotBefore:    now.Add(-NotBeforeSkew),
		NotAfter:     now.Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	return certificate, certPEM, keyPEM, nil
}
