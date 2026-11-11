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
	Iss string `json:"iss"`
	Azp string `json:"azp"`
}

func parse(token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, ErrMalformedToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims{}, fmt.Errorf("decode payload: %w", ErrMalformedToken)
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, fmt.Errorf("unmarshal payload: %w", ErrMalformedToken)
	}
	return c, nil
}

func ExpiresAt(token string) (time.Time, error) {
	c, err := parse(token)
	if err != nil {
		return time.Time{}, err
	}
	if c.Exp == nil {
		return time.Time{}, ErrNoExpiry
	}

	return time.Unix(*c.Exp, 0).UTC(), nil
}

func Issuer(token string) (string, error) {
	c, err := parse(token)
	if err != nil {
		return "", err
	}
	return c.Iss, nil
}

func AuthorizedParty(token string) (string, error) {
	c, err := parse(token)
	if err != nil {
		return "", err
	}
	return c.Azp, nil
}
