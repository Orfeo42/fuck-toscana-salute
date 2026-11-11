package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"fuck-toscana-salute/internal/session"
)

func testKey(fill byte) []byte {
	return bytes.Repeat([]byte{fill}, keyLength)
}

func newTestFileBackend(t *testing.T) (*FileBackend, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.json")
	b, err := NewFileBackend(path, testKey(1))
	require.NoError(t, err)
	return b, path
}

func TestNewFileBackend(t *testing.T) {
	t.Run("rejects a short key", func(t *testing.T) {
		_, err := NewFileBackend(filepath.Join(t.TempDir(), "s.json"), []byte("short"))
		assert.ErrorIs(t, err, ErrInvalidKey)
	})

	t.Run("rejects a long key", func(t *testing.T) {
		_, err := NewFileBackend(filepath.Join(t.TempDir(), "s.json"), bytes.Repeat([]byte{1}, 33))
		assert.ErrorIs(t, err, ErrInvalidKey)
	})

	t.Run("accepts a 32 byte key", func(t *testing.T) {
		_, err := NewFileBackend(filepath.Join(t.TempDir(), "s.json"), testKey(1))
		assert.NoError(t, err)
	})
}

func TestFileBackendSetGet(t *testing.T) {
	t.Run("round trips", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "secret"))

		got, err := b.Get(testService, testUser)
		require.NoError(t, err)
		assert.Equal(t, "secret", got)
	})

	t.Run("overwrite replaces the value", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "one"))
		require.NoError(t, b.Set(testService, testUser, "two"))

		got, err := b.Get(testService, testUser)
		require.NoError(t, err)
		assert.Equal(t, "two", got)
	})

	t.Run("missing entry returns ErrNotFound", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		_, err := b.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("plaintext is not stored", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "very-secret-value"))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "very-secret-value")
	})

	t.Run("file mode is 0600 and parent dir is created 0700", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "secrets.json")
		b, err := NewFileBackend(path, testKey(1))
		require.NoError(t, err)
		require.NoError(t, b.Set(testService, testUser, "secret"))

		fileInfo, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())

		dirInfo, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	})

	t.Run("no temp files are left behind", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "secret"))

		entries, err := os.ReadDir(filepath.Dir(path))
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("wrong key fails to decrypt", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "secret"))

		other, err := NewFileBackend(path, testKey(2))
		require.NoError(t, err)
		_, err = other.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("value copied to another entry name fails", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "secret"))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		entries := map[string]string{}
		require.NoError(t, json.Unmarshal(data, &entries))
		entries[entryName(testService, "other")] = entries[entryName(testService, testUser)]
		swapped, err := json.Marshal(entries)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, swapped, 0o600))

		_, err = b.Get(testService, "other")
		assert.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("corrupt json returns ErrFileCorrupt", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

		_, err := b.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrFileCorrupt)
		assert.ErrorIs(t, b.Set(testService, testUser, "x"), ErrFileCorrupt)
	})

	t.Run("invalid base64 value returns ErrFileCorrupt", func(t *testing.T) {
		b, path := newTestFileBackend(t)
		raw, err := json.Marshal(map[string]string{entryName(testService, testUser): "!!!"})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, raw, 0o600))

		_, err = b.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrFileCorrupt)
	})
}

func TestFileBackendDelete(t *testing.T) {
	t.Run("removes the entry", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, testUser, "secret"))
		require.NoError(t, b.Delete(testService, testUser))

		_, err := b.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("keeps other entries", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		require.NoError(t, b.Set(testService, "a", "one"))
		require.NoError(t, b.Set(testService, "b", "two"))
		require.NoError(t, b.Delete(testService, "a"))

		got, err := b.Get(testService, "b")
		require.NoError(t, err)
		assert.Equal(t, "two", got)
	})

	t.Run("missing entry returns ErrNotFound", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		assert.ErrorIs(t, b.Delete(testService, testUser), ErrNotFound)
	})
}

func TestVaultsOverFileBackend(t *testing.T) {
	ctx := context.Background()

	t.Run("Vault saves loads and clears a snapshot", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		v := NewWithBackend(b, testService, testUser)
		snapshot := sampleSnapshot()
		require.NoError(t, v.Save(ctx, snapshot))

		got, err := v.Load(ctx)
		require.NoError(t, err)
		assert.Equal(t, snapshot.Identity, got.Identity)
		assert.Len(t, got.Tokens, 2)

		require.NoError(t, v.Save(ctx, session.Snapshot{}))
		_, err = v.Load(ctx)
		assert.ErrorIs(t, err, session.ErrNoSnapshot)
	})

	t.Run("TokenVault saves loads and deletes a token", func(t *testing.T) {
		b, _ := newTestFileBackend(t)
		v := NewTokenVaultWithBackend(b, testService, "google")
		require.NoError(t, v.Save(ctx, &oauth2.Token{AccessToken: "at", RefreshToken: "rt"}))

		got, err := v.Load(ctx)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "rt", got.RefreshToken)

		require.NoError(t, v.Delete(ctx))
		require.NoError(t, v.Delete(ctx))
		got, err = v.Load(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
