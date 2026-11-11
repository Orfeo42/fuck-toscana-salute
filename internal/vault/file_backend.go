package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	keyLength = 32
	fileMode  = 0o600
	dirMode   = 0o700
)

const (
	ErrInvalidKey  = vaultError("vault: key must be 32 bytes")
	ErrFileCorrupt = vaultError("vault: secrets file is corrupt")
	ErrDecrypt     = vaultError("vault: cannot decrypt entry (wrong key or tampered data)")
)

type FileBackend struct {
	mu   sync.Mutex
	path string
	aead cipher.AEAD
}

var _ Backend = (*FileBackend)(nil)

func NewFileBackend(path string, key []byte) (*FileBackend, error) {
	if len(key) != keyLength {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault: new gcm: %w", err)
	}
	return &FileBackend{path: path, aead: aead}, nil
}

func (b *FileBackend) Get(service, user string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entries, err := b.read()
	if err != nil {
		return "", err
	}
	name := entryName(service, user)
	encoded, ok := entries[name]
	if !ok {
		return "", ErrNotFound
	}
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(sealed) < b.aead.NonceSize() {
		return "", fmt.Errorf("vault: get: %w", ErrFileCorrupt)
	}
	nonce, ciphertext := sealed[:b.aead.NonceSize()], sealed[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return "", fmt.Errorf("vault: get: %w", ErrDecrypt)
	}
	return string(plain), nil
}

func (b *FileBackend) Set(service, user, value string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	entries, err := b.read()
	if err != nil {
		return err
	}
	name := entryName(service, user)
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("vault: set: nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(value), []byte(name))
	entries[name] = base64.StdEncoding.EncodeToString(sealed)
	return b.write(entries)
}

func (b *FileBackend) Delete(service, user string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	entries, err := b.read()
	if err != nil {
		return err
	}
	name := entryName(service, user)
	if _, ok := entries[name]; !ok {
		return ErrNotFound
	}
	delete(entries, name)
	return b.write(entries)
}

func entryName(service, user string) string {
	return service + "/" + user
}

func (b *FileBackend) read() (map[string]string, error) {
	data, err := os.ReadFile(b.path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault: read secrets file: %w", err)
	}
	entries := map[string]string{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("vault: read secrets file: %w", ErrFileCorrupt)
	}
	return entries, nil
}

func (b *FileBackend) write(entries map[string]string) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("vault: write secrets file: marshal: %w", err)
	}
	dir := filepath.Dir(b.path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("vault: write secrets file: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("vault: write secrets file: %w", err)
	}
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("vault: write secrets file: %w", err)
	}
	if err := os.Rename(tmp.Name(), b.path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("vault: write secrets file: %w", err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte) error {
	if err := f.Chmod(fileMode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
