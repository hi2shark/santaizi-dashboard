package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultCertificateValidity = 30 * 24 * time.Hour
	DefaultCAValidity          = 10 * 365 * 24 * time.Hour
	NotBeforeSkew              = 5 * time.Minute
	DefaultRenewWindow         = 7 * 24 * time.Hour

	agentCAName        = "agent-ca"
	collectorCAName    = "collector-ca"
	agentCACN          = "Santaizi Agent CA"
	collectorCACN      = "Santaizi Collector CA"
	caKeyFileMode      = 0600
	caCertFileMode     = 0644
	clientKeyFileMode  = 0600
	clientCertFileMode = 0644
)

type Authority struct {
	Name    string
	Cert    *x509.Certificate
	Key     ed25519.PrivateKey
	CertPEM []byte
}

type Bundle struct {
	Dir       string
	Agent     *Authority
	Collector *Authority
}

func DefaultDir(dataDir string) string {
	if dataDir == "" {
		dataDir = "/var/lib/santaizi-dashboard"
	}
	return filepath.Join(dataDir, "pki")
}

func LoadOrCreate(dir string) (*Bundle, error) {
	if dir == "" {
		return nil, errors.New("pki directory is empty")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	agent, err := LoadOrCreateAgentCA(dir)
	if err != nil {
		return nil, err
	}
	collector, err := LoadOrCreateCollectorCA(dir)
	if err != nil {
		return nil, err
	}
	return &Bundle{Dir: dir, Agent: agent, Collector: collector}, nil
}

func LoadOrCreateAgentCA(dir string) (*Authority, error) {
	return loadOrCreateCA(dir, agentCAName, agentCACN)
}

func LoadOrCreateCollectorCA(dir string) (*Authority, error) {
	return loadOrCreateCA(dir, collectorCAName, collectorCACN)
}

func loadOrCreateCA(dir, name, commonName string) (*Authority, error) {
	keyPath := filepath.Join(dir, name+".key")
	certPath := filepath.Join(dir, name+".crt")
	keyPEM, keyErr := os.ReadFile(keyPath)    // #nosec G304 -- operator PKI path
	certPEM, certErr := os.ReadFile(certPath) // #nosec G304 -- operator PKI path
	switch {
	case errors.Is(keyErr, os.ErrNotExist) && errors.Is(certErr, os.ErrNotExist):
		return createCA(keyPath, certPath, commonName)
	case keyErr != nil:
		return nil, fmt.Errorf("read %s key: %w", name, keyErr)
	case certErr != nil:
		return nil, fmt.Errorf("read %s certificate: %w", name, certErr)
	}
	return parseCA(name, keyPEM, certPEM)
}

func createCA(keyPath, certPath, commonName string) (*Authority, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"Santaizi"}},
		NotBefore:             now.Add(-NotBeforeSkew),
		NotAfter:              now.Add(DefaultCAValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := writeFileAtomic(keyPath, keyPEM, caKeyFileMode); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(certPath, certPEM, caCertFileMode); err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Authority{Name: commonName, Cert: cert, Key: private, CertPEM: certPEM}, nil
}

func parseCA(name string, keyPEM, certPEM []byte) (*Authority, error) {
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("%s key is not PEM", name)
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s key: %w", name, err)
	}
	private, ok := parsedKey.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s key must be Ed25519", name)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("parse %s certificate: %w", name, err)
	}
	return &Authority{Name: name, Cert: cert, Key: private, CertPEM: append([]byte(nil), certPEM...)}, nil
}

func ParseCertificatePEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("certificate PEM is required")
	}
	return x509.ParseCertificate(block.Bytes)
}

func CertPool(certs ...*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		if cert != nil {
			pool.AddCert(cert)
		}
	}
	return pool
}

func AppendPEMToPool(pool *x509.CertPool, pemBytes []byte) error {
	if pool == nil {
		return errors.New("certificate pool is required")
	}
	if len(pemBytes) == 0 {
		return nil
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return errors.New("client CA PEM contains no certificates")
	}
	return nil
}

func SignAgentCSR(ca *Authority, csrDER, nodeUUID []byte, now time.Time) (certPEM []byte, notBefore, notAfter time.Time, err error) {
	if len(nodeUUID) != 16 {
		return nil, time.Time{}, time.Time{}, errors.New("node UUID must be 16 bytes")
	}
	return ca.signCSR(csrDER, EncodeAgentURI(nodeUUID), now, DefaultCertificateValidity)
}

func SignCollectorCSR(ca *Authority, csrDER []byte, collectorUUID string, now time.Time) (certPEM []byte, notBefore, notAfter time.Time, err error) {
	if collectorUUID == "" {
		return nil, time.Time{}, time.Time{}, errors.New("collector UUID is required")
	}
	return ca.signCSR(csrDER, EncodeCollectorURI(collectorUUID), now, DefaultCertificateValidity)
}

func (a *Authority) signCSR(csrDER []byte, uri string, now time.Time, validity time.Duration) ([]byte, time.Time, time.Time, error) {
	if a == nil || a.Cert == nil || len(a.Key) == 0 {
		return nil, time.Time{}, time.Time{}, errors.New("certificate authority is not loaded")
	}
	if len(csrDER) == 0 {
		return nil, time.Time{}, time.Time{}, errors.New("CSR is required")
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("CSR signature is invalid: %w", err)
	}
	if validity <= 0 {
		validity = DefaultCertificateValidity
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	notBefore := now.Add(-NotBeforeSkew)
	notAfter := now.Add(validity)
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: uri, Organization: []string{"Santaizi"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{mustURI(uri)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.Cert, csr.PublicKey, a.Key)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), notBefore, notAfter, nil
}

func randomSerial() (*big.Int, error) {
	serial := make([]byte, 16)
	if _, err := rand.Read(serial); err != nil {
		return nil, err
	}
	serial[0] &= 0x7f
	if serial[0] == 0 {
		serial[0] = 1
	}
	return new(big.Int).SetBytes(serial), nil
}
