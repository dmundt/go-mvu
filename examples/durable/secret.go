package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
)

func loadSecret(path string) ([]byte, error) {
	secret, err := os.ReadFile(path)
	if err == nil {
		if len(secret) != 32 {
			return nil, fmt.Errorf("session key %q must contain exactly 32 bytes", path)
		}
		return secret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read session key: %w", err)
	}
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadSecret(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create session key: %w", err)
	}
	if _, err := file.Write(secret); err != nil {
		file.Close()
		return nil, fmt.Errorf("write session key: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("sync session key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close session key: %w", err)
	}
	return secret, nil
}

func deriveSecret(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("go-mvu durable example: " + purpose))
	return mac.Sum(nil)
}
