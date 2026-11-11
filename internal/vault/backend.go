package vault

import (
	"errors"
	"fmt"
)

const ErrNotFound = vaultError("vault: entry not found")

type Backend interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

func Copy(dst, src Backend, service string, users []string) (int, error) {
	copied := 0
	for _, user := range users {
		value, err := src.Get(service, user)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return copied, fmt.Errorf("vault: copy read: %w", err)
		}
		_, err = dst.Get(service, user)
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrNotFound) {
			return copied, fmt.Errorf("vault: copy check: %w", err)
		}
		if err := dst.Set(service, user, value); err != nil {
			return copied, fmt.Errorf("vault: copy write: %w", err)
		}
		copied++
	}
	return copied, nil
}
