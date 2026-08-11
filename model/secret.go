package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
)

const encryptedSecretVersion byte = 1

var (
	secretCipherMu sync.RWMutex
	secretCipher   cipher.AEAD
)

func init() {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err == nil {
		_ = ConfigureSecretEncryption(key)
	}
}

// ConfigureSecretEncryption installs the process-local key used by model hooks.
// The key itself is owned by the Primary data directory and is never persisted
// in the database.
func ConfigureSecretEncryption(key []byte) error {
	if len(key) != 32 {
		return errors.New("business secret key must contain exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	secretCipherMu.Lock()
	secretCipher = aead
	secretCipherMu.Unlock()
	return nil
}

func encryptSecret(plain string) ([]byte, error) {
	if plain == "" {
		return nil, nil
	}
	secretCipherMu.RLock()
	aead := secretCipher
	secretCipherMu.RUnlock()
	if aead == nil {
		return nil, errors.New("business secret encryption is not configured")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	result := make([]byte, 1, 1+len(nonce)+len(plain)+aead.Overhead())
	result[0] = encryptedSecretVersion
	result = append(result, nonce...)
	return aead.Seal(result, nonce, []byte(plain), nil), nil
}

func decryptSecret(value []byte) (string, error) {
	if len(value) == 0 {
		return "", nil
	}
	secretCipherMu.RLock()
	aead := secretCipher
	secretCipherMu.RUnlock()
	if aead == nil {
		return "", errors.New("business secret encryption is not configured")
	}
	if value[0] != encryptedSecretVersion || len(value) < 1+aead.NonceSize()+aead.Overhead() {
		return "", fmt.Errorf("unsupported or malformed encrypted secret")
	}
	nonce := value[1 : 1+aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, value[1+aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("decrypt business secret: authentication failed")
	}
	return string(plain), nil
}
