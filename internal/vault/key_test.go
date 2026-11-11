package vault

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeKeyFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
	require.NoError(t, os.Chmod(path, mode))
	return path
}

func TestLoadKey(t *testing.T) {
	t.Run("reads base64 key and trims whitespace", func(t *testing.T) {
		want := testKey(7)
		path := writeKeyFile(t, "  "+base64.StdEncoding.EncodeToString(want)+"\n", 0o600)

		got, err := LoadKey(path)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("accepts a read-only owner mode", func(t *testing.T) {
		path := writeKeyFile(t, base64.StdEncoding.EncodeToString(testKey(7)), 0o400)
		_, err := LoadKey(path)
		assert.NoError(t, err)
	})

	t.Run("wrong length fails", func(t *testing.T) {
		path := writeKeyFile(t, base64.StdEncoding.EncodeToString([]byte("short")), 0o600)
		_, err := LoadKey(path)
		assert.ErrorIs(t, err, ErrKeyLength)
	})

	t.Run("invalid base64 fails", func(t *testing.T) {
		path := writeKeyFile(t, "!!!not-base64!!!", 0o600)
		_, err := LoadKey(path)
		assert.ErrorIs(t, err, ErrKeyEncoding)
	})

	t.Run("group readable file is refused", func(t *testing.T) {
		path := writeKeyFile(t, base64.StdEncoding.EncodeToString(testKey(7)), 0o640)
		_, err := LoadKey(path)
		assert.ErrorIs(t, err, ErrKeyMode)
		assert.ErrorContains(t, err, "0600")
	})

	t.Run("world readable file is refused", func(t *testing.T) {
		path := writeKeyFile(t, base64.StdEncoding.EncodeToString(testKey(7)), 0o604)
		_, err := LoadKey(path)
		assert.ErrorIs(t, err, ErrKeyMode)
	})

	t.Run("missing file fails", func(t *testing.T) {
		_, err := LoadKey(filepath.Join(t.TempDir(), "absent"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
