package jwtclaims

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeToken(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

func TestExpiresAt(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		got, err := ExpiresAt(fakeToken(`{"exp":1700000000}`))
		require.NoError(t, err)
		assert.Equal(t, time.Unix(1700000000, 0).UTC(), got)
	})

	t.Run("padded payload", func(t *testing.T) {
		payload := base64.URLEncoding.EncodeToString([]byte(`{"exp": 1700000000}`))
		require.Contains(t, payload, "=")
		got, err := ExpiresAt("h." + payload + ".sig")
		require.NoError(t, err)
		assert.Equal(t, time.Unix(1700000000, 0).UTC(), got)
	})

	t.Run("two parts", func(t *testing.T) {
		_, err := ExpiresAt("a.b")
		require.ErrorIs(t, err, ErrMalformedToken)
	})

	t.Run("bad base64", func(t *testing.T) {
		_, err := ExpiresAt("a.!!!.c")
		require.ErrorIs(t, err, ErrMalformedToken)
	})

	t.Run("bad json", func(t *testing.T) {
		_, err := ExpiresAt(fakeToken(`not json`))
		require.ErrorIs(t, err, ErrMalformedToken)
	})

	t.Run("missing exp", func(t *testing.T) {
		_, err := ExpiresAt(fakeToken(`{"sub":"x"}`))
		require.ErrorIs(t, err, ErrNoExpiry)
	})
}
