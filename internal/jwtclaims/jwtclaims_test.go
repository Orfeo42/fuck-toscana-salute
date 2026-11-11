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

func TestIssuer(t *testing.T) {
	t.Run("returns iss", func(t *testing.T) {
		got, err := Issuer(fakeToken(`{"iss":"https://auth.example/realms/rt"}`))
		require.NoError(t, err)
		assert.Equal(t, "https://auth.example/realms/rt", got)
	})

	t.Run("missing iss is empty", func(t *testing.T) {
		got, err := Issuer(fakeToken(`{"exp":1}`))
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := Issuer("a.b")
		require.ErrorIs(t, err, ErrMalformedToken)
	})
}

func TestAuthorizedParty(t *testing.T) {
	t.Run("returns azp", func(t *testing.T) {
		got, err := AuthorizedParty(fakeToken(`{"azp":"client-x"}`))
		require.NoError(t, err)
		assert.Equal(t, "client-x", got)
	})

	t.Run("missing azp is empty", func(t *testing.T) {
		got, err := AuthorizedParty(fakeToken(`{"exp":1}`))
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := AuthorizedParty("a.!!!.c")
		require.ErrorIs(t, err, ErrMalformedToken)
	})
}
