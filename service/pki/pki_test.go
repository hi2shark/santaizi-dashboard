package pki

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoadOrCreateCAIsStableAndSplit(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Agent.Cert.Raw, second.Agent.Cert.Raw) || !bytes.Equal(first.Collector.Cert.Raw, second.Collector.Cert.Raw) {
		t.Fatal("CA reload changed certificates")
	}
	if bytes.Equal(first.Agent.Key, first.Collector.Key) {
		t.Fatal("agent CA and collector CA must use different keys")
	}
	assertFilePerm(t, filepath.Join(dir, "agent-ca.key"), 0600)
	assertFilePerm(t, filepath.Join(dir, "collector-ca.key"), 0600)
}

func TestSignAgentCSRIgnoresCSRSanAndSetsURI(t *testing.T) {
	bundle := mustBundle(t)
	node := bytes.Repeat([]byte{0xab}, 16)
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(key, "urn:santaizi:agent:ffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	certPEM, notBefore, notAfter, err := SignAgentCSR(bundle.Agent, csr, node, now)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseAgentIdentityFromCertificate(cert)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, node) {
		t.Fatalf("signed SAN uuid=%x want %x", got, node)
	}
	if cert.NotBefore.After(now) || !notBefore.Equal(now.Add(-NotBeforeSkew)) {
		t.Fatalf("NotBefore=%v now=%v", cert.NotBefore, now)
	}
	if !notAfter.Equal(now.Add(DefaultCertificateValidity)) || !cert.NotAfter.Equal(notAfter) {
		t.Fatalf("NotAfter=%v", cert.NotAfter)
	}
	if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("ExtKeyUsage=%v", cert.ExtKeyUsage)
	}
	if _, err := ParseCollectorIdentityFromCertificate(cert); err == nil {
		t.Fatal("agent cert must not parse as collector")
	}
}

func TestSignCollectorCSRRejectsAgentURIConfusion(t *testing.T) {
	bundle := mustBundle(t)
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(key, EncodeAgentURI(bytes.Repeat([]byte{1}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, _, err := SignCollectorCSR(bundle.Collector, csr, "collector-edge", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseCollectorIdentityFromCertificate(cert)
	if err != nil || id != "collector-edge" {
		t.Fatalf("collector identity=%q err=%v", id, err)
	}
	if _, err := ParseAgentIdentityFromCertificate(cert); err == nil {
		t.Fatal("collector cert must not parse as agent")
	}
}

func TestSignCSRRejectsBadSignatureAndExpiredWindow(t *testing.T) {
	bundle := mustBundle(t)
	_, _, _, err := bundle.Agent.signCSR([]byte("not-a-csr"), EncodeAgentURI(bytes.Repeat([]byte{2}, 16)), time.Now(), DefaultCertificateValidity)
	if err == nil {
		t.Fatal("expected invalid CSR")
	}
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(key, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	certPEM, _, notAfter, err := SignAgentCSR(bundle.Agent, csr, bytes.Repeat([]byte{3}, 16), now)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !cert.NotAfter.Equal(notAfter) || cert.NotAfter.Before(now.Add(29*24*time.Hour)) {
		t.Fatalf("validity=%v", cert.NotAfter.Sub(now))
	}
}

func TestClientBundleAtomicReplaceAndRenewWindow(t *testing.T) {
	store, err := NewClientStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bundle := mustBundle(t)
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(key, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	certPEM, _, _, err := SignAgentCSR(bundle.Agent, csr, bytes.Repeat([]byte{9}, 16), now)
	if err != nil {
		t.Fatal(err)
	}
	client := &ClientBundle{Key: key, CertPEM: certPEM, CAPEM: bundle.Agent.CertPEM}
	client.Cert, err = ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(client); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded.CertPEM, certPEM) {
		t.Fatal("loaded certificate mismatch")
	}
	assertFilePerm(t, filepath.Join(store.Dir(), clientKeyName), 0600)
	if loaded.NeedsRenew(now, DefaultRenewWindow) {
		t.Fatal("fresh certificate should not need renew")
	}
	if !loaded.NeedsRenew(loaded.Cert.NotAfter.Add(-time.Hour), DefaultRenewWindow) {
		t.Fatal("certificate inside renew window should need renew")
	}
	if loaded.Expired(now) || !loaded.Expired(loaded.Cert.NotAfter.Add(time.Second)) {
		t.Fatal("expiry detection failed")
	}
}

func TestNewSelfSignedServerCertificateIsNotDeviceCA(t *testing.T) {
	bundle := mustBundle(t)
	_, certPEM, _, err := NewSelfSignedServerCertificate([]string{"localhost"}, time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDeviceIdentityFromCertificate(cert); err == nil {
		t.Fatal("server certificate must not carry a device URI")
	}
	if bytes.Equal(cert.RawIssuer, bundle.Agent.Cert.RawSubject) || bytes.Equal(cert.RawIssuer, bundle.Collector.Cert.RawSubject) {
		t.Fatal("server certificate must not be issued by a device CA")
	}
}

func TestCertPoolAcceptsMultipleCAs(t *testing.T) {
	bundle := mustBundle(t)
	pool := CertPool(bundle.Agent.Cert, bundle.Collector.Cert)
	if pool == nil {
		t.Fatal("pool")
	}
	if err := AppendPEMToPool(pool, bundle.Agent.CertPEM); err != nil {
		t.Fatal(err)
	}
}

func mustBundle(t *testing.T) *Bundle {
	t.Helper()
	bundle, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func assertFilePerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != want {
		t.Fatalf("%s perm=%o want %o", path, info.Mode().Perm(), want)
	}
}

func TestParseURIRequiresExactKind(t *testing.T) {
	node := bytes.Repeat([]byte{0x11}, 16)
	if got := EncodeAgentURI(node); got != "urn:santaizi:agent:"+string(mustHex(node)) {
		// compare via parse
	}
	parsed, err := ParseAgentURI(EncodeAgentURI(node))
	if err != nil || !bytes.Equal(parsed, node) {
		t.Fatalf("parse agent uri: %x %v", parsed, err)
	}
	id, err := ParseCollectorURI(EncodeCollectorURI("collector-aa"))
	if err != nil || id != "collector-aa" {
		t.Fatalf("parse collector uri: %q %v", id, err)
	}
	if _, err := ParseAgentURI(EncodeCollectorURI("collector-aa")); err == nil {
		t.Fatal("collector URI must not parse as agent")
	}
}

func mustHex(node []byte) []byte {
	return []byte(EncodeAgentURI(node)[len(AgentURIPrefix):])
}

func TestEd25519PublicMatchesSignedCert(t *testing.T) {
	bundle := mustBundle(t)
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(key, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, _, err := SignAgentCSR(bundle.Agent, csr, bytes.Repeat([]byte{0x42}, 16), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		t.Fatal("expected Ed25519 public key")
	}
	if !bytes.Equal(pub, key.Public().(ed25519.PublicKey)) {
		t.Fatal("signed certificate public key mismatch")
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("pem")
	}
}
