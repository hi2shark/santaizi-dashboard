package pki

import (
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	AgentURIPrefix     = "urn:santaizi:agent:"
	CollectorURIPrefix = "urn:santaizi:collector:"
)

type DeviceKind int

const (
	DeviceUnknown DeviceKind = iota
	DeviceAgent
	DeviceCollector
)

type DeviceIdentity struct {
	Kind          DeviceKind
	NodeUUID      []byte
	CollectorUUID string
}

func EncodeAgentURI(nodeUUID []byte) string {
	return AgentURIPrefix + hex.EncodeToString(nodeUUID)
}

func EncodeCollectorURI(collectorUUID string) string {
	return CollectorURIPrefix + collectorUUID
}

func ParseAgentURI(raw string) ([]byte, error) {
	if !strings.HasPrefix(raw, AgentURIPrefix) {
		return nil, fmt.Errorf("not an agent URI")
	}
	rest := strings.ReplaceAll(strings.TrimPrefix(raw, AgentURIPrefix), "-", "")
	decoded, err := hex.DecodeString(rest)
	if err != nil || len(decoded) != 16 {
		return nil, errors.New("agent URI must contain a 16-byte UUID")
	}
	return decoded, nil
}

func ParseCollectorURI(raw string) (string, error) {
	if !strings.HasPrefix(raw, CollectorURIPrefix) {
		return "", fmt.Errorf("not a collector URI")
	}
	id := strings.TrimSpace(strings.TrimPrefix(raw, CollectorURIPrefix))
	if id == "" {
		return "", errors.New("collector URI is missing collector UUID")
	}
	return id, nil
}

func ParseDeviceIdentityFromCertificate(cert *x509.Certificate) (*DeviceIdentity, error) {
	if cert == nil {
		return nil, errors.New("certificate is required")
	}
	var identity *DeviceIdentity
	for _, uri := range cert.URIs {
		if uri == nil {
			continue
		}
		raw := uri.String()
		switch {
		case strings.HasPrefix(raw, AgentURIPrefix):
			nodeUUID, err := ParseAgentURI(raw)
			if err != nil {
				return nil, err
			}
			if identity != nil {
				return nil, errors.New("certificate has conflicting Santaizi URIs")
			}
			identity = &DeviceIdentity{Kind: DeviceAgent, NodeUUID: nodeUUID}
		case strings.HasPrefix(raw, CollectorURIPrefix):
			collectorUUID, err := ParseCollectorURI(raw)
			if err != nil {
				return nil, err
			}
			if identity != nil {
				return nil, errors.New("certificate has conflicting Santaizi URIs")
			}
			identity = &DeviceIdentity{Kind: DeviceCollector, CollectorUUID: collectorUUID}
		}
	}
	if identity == nil {
		return nil, errors.New("certificate is missing a Santaizi device URI SAN")
	}
	return identity, nil
}

func ParseAgentIdentityFromCertificate(cert *x509.Certificate) ([]byte, error) {
	identity, err := ParseDeviceIdentityFromCertificate(cert)
	if err != nil {
		return nil, err
	}
	if identity.Kind != DeviceAgent {
		return nil, errors.New("certificate is not an agent device certificate")
	}
	return identity.NodeUUID, nil
}

func ParseCollectorIdentityFromCertificate(cert *x509.Certificate) (string, error) {
	identity, err := ParseDeviceIdentityFromCertificate(cert)
	if err != nil {
		return "", err
	}
	if identity.Kind != DeviceCollector {
		return "", errors.New("certificate is not a collector device certificate")
	}
	return identity.CollectorUUID, nil
}

func mustURI(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
