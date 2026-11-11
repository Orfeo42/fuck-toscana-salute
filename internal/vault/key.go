package vault

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const (
	ErrKeyLength   = vaultError("vault: key file must hold base64 of exactly 32 bytes")
	ErrKeyEncoding = vaultError("vault: key file is not valid base64")
	ErrKeyMode     = vaultError("vault: key file must not be accessible by group or others (required mode 0600)")
)

func LoadKey(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("vault: load key: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("vault: load key: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("vault: load key: %w", ErrKeyMode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vault: load key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("vault: load key: %w", ErrKeyEncoding)
	}
	if len(key) != keyLength {
		return nil, fmt.Errorf("vault: load key: %w", ErrKeyLength)
	}
	return key, nil
}
