package telemetry

import (
	"bytes"
	"testing"
	"time"
)

func TestCredentialSignVerifyAndGrace(t *testing.T) {
	signer, err := LoadOrCreateSigner(t.TempDir() + "/signing.key")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	node := bytes.Repeat([]byte{1}, 16)
	credential, err := signer.Sign(node, 7, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := VerifyCredential(signer.PublicKey(), signer.KeyID(), credential, now.Add(30*time.Minute), time.Hour, false)
	if err != nil || verification.InGrace || !bytes.Equal(verification.Claims.GetNodeUuid(), node) {
		t.Fatalf("verification=%#v err=%v", verification, err)
	}
	verification, err = VerifyCredential(signer.PublicKey(), signer.KeyID(), credential, now.Add(90*time.Minute), time.Hour, true)
	if err != nil || !verification.InGrace {
		t.Fatalf("grace verification=%#v err=%v", verification, err)
	}
	if _, err := VerifyCredential(signer.PublicKey(), signer.KeyID(), credential, now.Add(90*time.Minute), time.Hour, false); err == nil {
		t.Fatal("expired credential was accepted without prior authorization")
	}
}

func TestRegistrationTokenHash(t *testing.T) {
	plain, hash, err := NewRegistrationToken()
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || len(hash) != 32 || !RegistrationTokenMatches(plain, hash) {
		t.Fatal("registration token did not verify")
	}
	if RegistrationTokenMatches(plain+"x", hash) {
		t.Fatal("modified token verified")
	}
}
