package vault

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const ErrNotFound = vaultError("vault: entry not found")

type Backend interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

type KeyringBackend struct{}

var _ Backend = KeyringBackend{}

func (KeyringBackend) Get(service, user string) (string, error) {
	value, err := keyring.Get(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("keyring get: %w", err)
	}
	return value, nil
}

func (KeyringBackend) Set(service, user, value string) error {
	if err := keyring.Set(service, user, value); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}
	return nil
}

func (KeyringBackend) Delete(service, user string) error {
	err := keyring.Delete(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("keyring delete: %w", err)
	}
	return nil
}
