package vault

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestTokenVault(t *testing.T) {
	ctx := context.Background()

	t.Run("load returns nil when nothing is stored", func(t *testing.T) {
		got, err := NewTokenVaultWithBackend(newMemoryBackend(), testService, "google").Load(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("save then load round trips", func(t *testing.T) {
		v := NewTokenVaultWithBackend(newMemoryBackend(), testService, "google")
		expiry := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		require.NoError(t, v.Save(ctx, &oauth2.Token{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", Expiry: expiry}))

		got, err := v.Load(ctx)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "at", got.AccessToken)
		assert.Equal(t, "rt", got.RefreshToken)
		assert.True(t, expiry.Equal(got.Expiry))
	})

	t.Run("delete removes the token and tolerates absence", func(t *testing.T) {
		v := NewTokenVaultWithBackend(newMemoryBackend(), testService, "google")
		require.NoError(t, v.Save(ctx, &oauth2.Token{AccessToken: "at"}))

		require.NoError(t, v.Delete(ctx))
		got, err := v.Load(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
		require.NoError(t, v.Delete(ctx))
	})

	t.Run("corrupt payload is reported", func(t *testing.T) {
		b := newMemoryBackend()
		require.NoError(t, b.Set(testService, "google", "not-json"))

		_, err := NewTokenVaultWithBackend(b, testService, "google").Load(ctx)
		require.ErrorIs(t, err, ErrCorrupt)
	})

	t.Run("does not touch the session entry", func(t *testing.T) {
		b := newMemoryBackend()
		require.NoError(t, b.Set(testService, testUser, "session-data"))
		require.NoError(t, NewTokenVaultWithBackend(b, testService, "google").Delete(ctx))

		got, err := b.Get(testService, testUser)
		require.NoError(t, err)
		assert.Equal(t, "session-data", got)
	})

	t.Run("cancelled context is rejected", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		v := NewTokenVaultWithBackend(newMemoryBackend(), testService, "google")

		_, err := v.Load(cancelled)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, v.Save(cancelled, &oauth2.Token{}), context.Canceled)
		require.ErrorIs(t, v.Delete(cancelled), context.Canceled)
	})
}
