package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

type TokenVault struct {
	service string
	user    string
}

func NewTokenVault(service, user string) *TokenVault {
	return &TokenVault{service: service, user: user}
}

func (v *TokenVault) Load(ctx context.Context) (*oauth2.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("vault: load token: %w", err)
	}

	raw, err := keyring.Get(v.service, v.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault: load token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return nil, fmt.Errorf("vault: load token: %w", ErrCorrupt)
	}
	return &token, nil
}

func (v *TokenVault) Save(ctx context.Context, token *oauth2.Token) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("vault: save token: %w", err)
	}

	data, err := json.Marshal(token)
	if err != nil {
		return errors.New("vault: save token: marshal failed")
	}
	if err := keyring.Set(v.service, v.user, string(data)); err != nil {
		return fmt.Errorf("vault: save token: %w", err)
	}
	return nil
}

func (v *TokenVault) Delete(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("vault: delete token: %w", err)
	}

	err := keyring.Delete(v.service, v.user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("vault: delete token: %w", err)
	}
	return nil
}
