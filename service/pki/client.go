package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	clientKeyName          = "client.key"
	clientCertName         = "client.crt"
	clientCAName           = "ca.crt"
	agentCANameOnCollector = "agent-ca.crt"
)

var ErrClientBundleNotFound = errors.New("client certificate bundle not found")

type ClientBundle struct {
	Key     ed25519.PrivateKey
	Cert    *x509.Certificate
	CertPEM []byte
	CAPEM   []byte
}

func GenerateKey() (ed25519.PrivateKey, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	return private, err
}

func CreateCSR(private ed25519.PrivateKey, uri string) ([]byte, error) {
	if len(private) == 0 {
		return nil, errors.New("private key is required")
	}
	template := &x509.CertificateRequest{Subject: pkix.Name{CommonName: uri}}
	if uri != "" {
		template.URIs = []*url.URL{mustURI(uri)}
	}
	return x509.CreateCertificateRequest(rand.Reader, template, private)
}

func MarshalPrivateKeyPEM(private ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func ParsePrivateKeyPEM(pemBytes []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("private key PEM is required")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key must be Ed25519")
	}
	return private, nil
}

func (b *ClientBundle) NeedsRenew(now time.Time, window time.Duration) bool {
	if b == nil || b.Cert == nil {
		return true
	}
	if window <= 0 {
		window = DefaultRenewWindow
	}
	return !now.Before(b.Cert.NotAfter.Add(-window))
}

func (b *ClientBundle) Expired(now time.Time) bool {
	return b == nil || b.Cert == nil || !now.Before(b.Cert.NotAfter)
}

type ClientStore struct {
	mu  sync.RWMutex
	dir string
}

func NewClientStore(dir string) (*ClientStore, error) {
	if dir == "" {
		return nil, errors.New("client pki directory is empty")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &ClientStore{dir: dir}, nil
}

func (s *ClientStore) Dir() string { return s.dir }

func (s *ClientStore) Load() (*ClientBundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadLocked()
}

func (s *ClientStore) loadLocked() (*ClientBundle, error) {
	keyPEM, err := os.ReadFile(filepath.Join(s.dir, clientKeyName)) // #nosec G304 -- local PKI store
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrClientBundleNotFound
	}
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(filepath.Join(s.dir, clientCertName)) // #nosec G304 -- local PKI store
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrClientBundleNotFound
	}
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(filepath.Join(s.dir, clientCAName)) // #nosec G304 -- local PKI store
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, err := ParsePrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, err
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}
	return &ClientBundle{Key: key, Cert: cert, CertPEM: certPEM, CAPEM: caPEM}, nil
}

func (s *ClientStore) Save(bundle *ClientBundle) error {
	if bundle == nil || len(bundle.Key) == 0 || len(bundle.CertPEM) == 0 {
		return errors.New("client certificate bundle is incomplete")
	}
	keyPEM, err := MarshalPrivateKeyPEM(bundle.Key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeFileAtomic(filepath.Join(s.dir, clientKeyName), keyPEM, clientKeyFileMode); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(s.dir, clientCertName), bundle.CertPEM, clientCertFileMode); err != nil {
		return err
	}
	if len(bundle.CAPEM) > 0 {
		if err := writeFileAtomic(filepath.Join(s.dir, clientCAName), bundle.CAPEM, clientCertFileMode); err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientStore) SaveAgentCA(pemBytes []byte) error {
	if len(pemBytes) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeFileAtomic(filepath.Join(s.dir, agentCANameOnCollector), pemBytes, clientCertFileMode)
}

func (s *ClientStore) LoadAgentCA() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pemBytes, err := os.ReadFile(filepath.Join(s.dir, agentCANameOnCollector)) // #nosec G304 -- local PKI store
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return pemBytes, err
}

func (s *ClientStore) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cert, err := tls.LoadX509KeyPair(filepath.Join(s.dir, clientCertName), filepath.Join(s.dir, clientKeyName)) // #nosec G304 -- local PKI store
	if errors.Is(err, os.ErrNotExist) {
		return &tls.Certificate{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func ClientTLSConfig(opts ClientTLSOptions) (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(opts.ExtraCAPEM) > 0 {
		if err := AppendPEMToPool(roots, opts.ExtraCAPEM); err != nil {
			return nil, err
		}
	}
	if opts.CAFile != "" {
		pemBytes, err := os.ReadFile(opts.CAFile) // #nosec G304 -- operator-configured CA file
		if err != nil {
			return nil, fmt.Errorf("read tls ca_file: %w", err)
		}
		if err := AppendPEMToPool(roots, pemBytes); err != nil {
			return nil, err
		}
	}
	cfg := &tls.Config{
		MinVersion:           tls.VersionTLS12,
		RootCAs:              roots,
		InsecureSkipVerify:   opts.InsecureSkipVerify, //nolint:gosec // operator testing flag
		GetClientCertificate: opts.GetClientCertificate,
	}
	if cfg.GetClientCertificate == nil && opts.Bundle != nil {
		certificate, err := tls.X509KeyPair(opts.Bundle.CertPEM, mustKeyPEM(opts.Bundle.Key))
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{certificate}
	}
	return cfg, nil
}

type ClientTLSOptions struct {
	CAFile               string
	ExtraCAPEM           []byte
	InsecureSkipVerify   bool
	Bundle               *ClientBundle
	GetClientCertificate func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
}

func mustKeyPEM(key ed25519.PrivateKey) []byte {
	pemBytes, err := MarshalPrivateKeyPEM(key)
	if err != nil {
		panic(err)
	}
	return pemBytes
}
