package telemetry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
)

const TelemetryCapability = "telemetry"

type Signer struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	keyID   []byte
}

func LoadOrCreateSigner(path string) (*Signer, error) {
	if path == "" {
		return nil, errors.New("signing key path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	seed, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		seed = make([]byte, ed25519.SeedSize)
		if _, err := io.ReadFull(rand.Reader, seed); err != nil {
			return nil, err
		}
		if err := writeSecretAtomic(path, seed); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("signing key has invalid length %d", len(seed))
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	return &Signer{private: private, public: public, keyID: append([]byte(nil), sum[:16]...)}, nil
}

func (s *Signer) PublicKey() []byte {
	return append([]byte(nil), s.public...)
}

func (s *Signer) KeyID() []byte {
	return append([]byte(nil), s.keyID...)
}

func (s *Signer) Sign(nodeUUID []byte, configVersion uint64, now time.Time, validity time.Duration) (*pb.SignedAgentCredential, error) {
	if len(nodeUUID) != 16 {
		return nil, errors.New("node UUID must be 16 bytes")
	}
	claims := &pb.AgentCredentialClaims{
		NodeUuid:      append([]byte(nil), nodeUUID...),
		IssuedAtUnix:  now.Unix(),
		ExpiresAtUnix: now.Add(validity).Unix(),
		Capability:    TelemetryCapability,
		ConfigVersion: configVersion,
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(claims)
	if err != nil {
		return nil, err
	}
	return &pb.SignedAgentCredential{
		Claims:    encoded,
		Signature: ed25519.Sign(s.private, encoded),
		KeyId:     s.KeyID(),
	}, nil
}

type Verification struct {
	Claims  *pb.AgentCredentialClaims
	InGrace bool
}

func VerifyCredential(publicKey, expectedKeyID []byte, credential *pb.SignedAgentCredential, now time.Time, grace time.Duration, previouslyAuthorized bool) (*Verification, error) {
	if len(publicKey) != ed25519.PublicKeySize || credential == nil {
		return nil, errors.New("invalid telemetry credential")
	}
	if len(expectedKeyID) > 0 && subtle.ConstantTimeCompare(expectedKeyID, credential.GetKeyId()) != 1 {
		return nil, errors.New("telemetry credential key ID mismatch")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), credential.GetClaims(), credential.GetSignature()) {
		return nil, errors.New("telemetry credential signature is invalid")
	}
	claims := new(pb.AgentCredentialClaims)
	if err := proto.Unmarshal(credential.GetClaims(), claims); err != nil {
		return nil, err
	}
	if len(claims.GetNodeUuid()) != 16 || claims.GetCapability() != TelemetryCapability {
		return nil, errors.New("telemetry credential claims are invalid")
	}
	if now.Unix() < claims.GetIssuedAtUnix()-300 {
		return nil, errors.New("telemetry credential is not active")
	}
	if now.Unix() <= claims.GetExpiresAtUnix() {
		return &Verification{Claims: claims}, nil
	}
	if previouslyAuthorized && now.Unix() <= claims.GetExpiresAtUnix()+int64(grace.Seconds()) {
		return &Verification{Claims: claims, InGrace: true}, nil
	}
	return nil, errors.New("telemetry credential expired")
}

func NewRegistrationToken() (plain string, hash []byte, err error) {
	token := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, token); err != nil {
		return "", nil, err
	}
	plain = fmt.Sprintf("stc_%x", token)
	digest := sha256.Sum256([]byte(plain))
	return plain, digest[:], nil
}

func RegistrationTokenMatches(plain string, expectedHash []byte) bool {
	digest := sha256.Sum256([]byte(plain))
	return len(expectedHash) == len(digest) && subtle.ConstantTimeCompare(digest[:], expectedHash) == 1
}

func writeSecretAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".signing-key-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
