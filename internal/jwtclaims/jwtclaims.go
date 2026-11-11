package jwtclaims

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtError string

func (e jwtError) Error() string { return string(e) }

const (
	ErrMalformedToken jwtError = "malformed token"
	ErrNoExpiry       jwtError = "token has no exp claim"
)

type claims struct {
	Exp *int64 `json:"exp"`
}

func ExpiresAt(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, ErrMalformedToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, fmt.Errorf("decode payload: %w", ErrMalformedToken)
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return time.Time{}, fmt.Errorf("unmarshal payload: %w", ErrMalformedToken)
	}
	if c.Exp == nil {
		return time.Time{}, ErrNoExpiry
	}

	return time.Unix(*c.Exp, 0).UTC(), nil
}
