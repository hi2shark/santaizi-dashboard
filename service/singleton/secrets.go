package singleton

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hi2shark/santaizi-dashboard/model"
)

const businessSecretKeySize = 32

func initBusinessSecretEncryption(path string) error {
	if path == "" {
		return errors.New("business secret key path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create secret key directory: %w", err)
	}
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, businessSecretKeySize)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return fmt.Errorf("generate business secret key: %w", err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("create business secret key: %w", err)
		}
		if _, err = file.Write(key); err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			return fmt.Errorf("write business secret key: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("close business secret key: %w", closeErr)
		}
	} else if err != nil {
		return fmt.Errorf("read business secret key: %w", err)
	}
	if len(key) != businessSecretKeySize {
		return fmt.Errorf("business secret key must contain exactly %d bytes", businessSecretKeySize)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("secure business secret key permissions: %w", err)
	}
	return model.ConfigureSecretEncryption(key)
}
