package vault

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
)

const (
	testService = "fts-test"
	testUser    = "tester"
)

func sampleSnapshot() session.Snapshot {
	base := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	return session.Snapshot{
		Tokens: map[session.Portal]session.Tokens{
			session.PortalCUP: {
				Access:           "cup-access",
				Refresh:          "cup-refresh",
				AccessExpiresAt:  base.Add(time.Hour),
				RefreshExpiresAt: base.Add(24 * time.Hour),
				GestioneVerified: true,
				StartedAt:        base,
			},
			session.PortalFSE: {
				Access:           "fse-access",
				Refresh:          "fse-refresh",
				AccessExpiresAt:  base.Add(2 * time.Hour),
				RefreshExpiresAt: base.Add(48 * time.Hour),
				StartedAt:        base.Add(time.Minute),
			},
		},
		Identity: session.Identity{
			CodiceFiscale:    "AAABBB00A00A000A",
			TesseraSanitaria: "80000000000000000000",
		},
	}
}

func TestNewWithBackend(t *testing.T) {
	t.Run("stores service and user", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)
		assert.Equal(t, testService, v.service)
		assert.Equal(t, testUser, v.user)
	})
}

func TestSave(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)
		want := sampleSnapshot()

		require.NoError(t, v.Save(context.Background(), want))
		got, err := v.Load(context.Background())
		require.NoError(t, err)

		assert.Equal(t, want.Identity, got.Identity)
		require.Len(t, got.Tokens, 2)
		assert.Equal(t, want.Tokens[session.PortalCUP].Access, got.Tokens[session.PortalCUP].Access)
		assert.Equal(t, want.Tokens[session.PortalCUP].Refresh, got.Tokens[session.PortalCUP].Refresh)
		assert.True(t, want.Tokens[session.PortalCUP].AccessExpiresAt.Equal(got.Tokens[session.PortalCUP].AccessExpiresAt))
		assert.True(t, want.Tokens[session.PortalCUP].RefreshExpiresAt.Equal(got.Tokens[session.PortalCUP].RefreshExpiresAt))
		assert.True(t, got.Tokens[session.PortalCUP].GestioneVerified)
		assert.True(t, want.Tokens[session.PortalCUP].StartedAt.Equal(got.Tokens[session.PortalCUP].StartedAt))
		assert.Equal(t, want.Tokens[session.PortalFSE].Access, got.Tokens[session.PortalFSE].Access)
		assert.Equal(t, want.Tokens[session.PortalFSE].Refresh, got.Tokens[session.PortalFSE].Refresh)
		assert.True(t, want.Tokens[session.PortalFSE].AccessExpiresAt.Equal(got.Tokens[session.PortalFSE].AccessExpiresAt))
		assert.True(t, want.Tokens[session.PortalFSE].RefreshExpiresAt.Equal(got.Tokens[session.PortalFSE].RefreshExpiresAt))
		assert.False(t, got.Tokens[session.PortalFSE].GestioneVerified)
		assert.True(t, want.Tokens[session.PortalFSE].StartedAt.Equal(got.Tokens[session.PortalFSE].StartedAt))
	})

	t.Run("empty snapshot deletes stored blob", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)
		require.NoError(t, v.Save(context.Background(), sampleSnapshot()))

		require.NoError(t, v.Save(context.Background(), session.Snapshot{}))
		_, err := v.Load(context.Background())
		assert.ErrorIs(t, err, session.ErrNoSnapshot)
	})

	t.Run("empty snapshot with nothing stored is nil", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)

		assert.NoError(t, v.Save(context.Background(), session.Snapshot{}))
	})

	t.Run("cancelled context", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := v.Save(ctx, sampleSnapshot())
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestLoad(t *testing.T) {
	t.Run("nothing stored", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)

		_, err := v.Load(context.Background())
		assert.ErrorIs(t, err, session.ErrNoSnapshot)
	})

	t.Run("malformed blob", func(t *testing.T) {
		b := newMemoryBackend()
		require.NoError(t, b.Set(testService, testUser, "{not json"))
		v := NewWithBackend(b, testService, testUser)

		_, err := v.Load(context.Background())
		assert.ErrorIs(t, err, ErrCorrupt)
	})

	t.Run("wrong version", func(t *testing.T) {
		b := newMemoryBackend()
		require.NoError(t, b.Set(testService, testUser, `{"version":99,"tokens":{},"identity":{}}`))
		v := NewWithBackend(b, testService, testUser)

		_, err := v.Load(context.Background())
		assert.ErrorIs(t, err, ErrCorrupt)
	})

	t.Run("unknown portal skipped", func(t *testing.T) {
		blob := `{"version":1,"tokens":{"cup":{"access":"a"},"zzz":{"access":"b"}},"identity":{"codiceFiscale":"X"}}`
		b := newMemoryBackend()
		require.NoError(t, b.Set(testService, testUser, blob))
		v := NewWithBackend(b, testService, testUser)

		got, err := v.Load(context.Background())
		require.NoError(t, err)
		require.Len(t, got.Tokens, 1)
		assert.Equal(t, "a", got.Tokens[session.PortalCUP].Access)
		assert.Equal(t, "X", got.Identity.CodiceFiscale)
	})

	t.Run("cancelled context", func(t *testing.T) {
		v := NewWithBackend(newMemoryBackend(), testService, testUser)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := v.Load(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
