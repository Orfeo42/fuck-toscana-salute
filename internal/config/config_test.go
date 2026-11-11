package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		dataHome := t.TempDir()
		configHome := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dataHome)
		t.Setenv("XDG_CONFIG_HOME", configHome)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, Config{
			Addr:           "127.0.0.1:8787",
			BrowserChannel: "chrome",
			LoginTimeout:   10 * time.Minute,
			RefreshMargin:  60 * time.Second,
			RefreshTick:    15 * time.Second,
			ProfileDir:     filepath.Join(dataHome, "fuck-toscana-salute", "chrome-profile"),

			GoogleClientFile: filepath.Join(configHome, "fuck-toscana-salute", "google-client.json"),
		}, got)
	})

	t.Run("profile dir from home when XDG_DATA_HOME empty", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", home)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".local", "share", "fuck-toscana-salute", "chrome-profile"), got.ProfileDir)
	})

	t.Run("relative XDG_DATA_HOME ignored", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "relative/data")
		t.Setenv("HOME", home)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".local", "share", "fuck-toscana-salute", "chrome-profile"), got.ProfileDir)
	})

	t.Run("explicit profile dir", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("FTS_PROFILE_DIR", dir)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, dir, got.ProfileDir)
	})

	t.Run("google client file from env", func(t *testing.T) {
		t.Setenv("FTS_GOOGLE_CLIENT_FILE", "/tmp/client.json")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "/tmp/client.json", got.GoogleClientFile)
	})

	t.Run("google client file default under user config dir", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(configHome, "fuck-toscana-salute", "google-client.json"), got.GoogleClientFile)
	})

	t.Run("relative profile dir rejected", func(t *testing.T) {
		t.Setenv("FTS_PROFILE_DIR", "profile")
		_, err := Load()
		require.ErrorIs(t, err, ErrInvalidProfileDir)
	})

	t.Run("custom ipv6 loopback", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "[::1]:9000")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "[::1]:9000", got.Addr)
	})

	t.Run("localhost", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "localhost:1")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "localhost:1", got.Addr)
	})

	t.Run("unspecified address rejected", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "0.0.0.0:8787")
		_, err := Load()
		require.ErrorIs(t, err, ErrNonLoopbackAddr)
	})

	t.Run("lan address rejected", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "192.168.1.5:80")
		_, err := Load()
		require.ErrorIs(t, err, ErrNonLoopbackAddr)
	})

	t.Run("empty host rejected", func(t *testing.T) {
		t.Setenv("FTS_ADDR", ":8787")
		_, err := Load()
		require.ErrorIs(t, err, ErrNonLoopbackAddr)
	})

	t.Run("address without port rejected", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "127.0.0.1")
		_, err := Load()
		require.ErrorIs(t, err, ErrNonLoopbackAddr)
	})

	t.Run("zero login timeout", func(t *testing.T) {
		t.Setenv("FTS_LOGIN_TIMEOUT", "0s")
		_, err := Load()
		require.ErrorIs(t, err, ErrInvalidDuration)
	})

	t.Run("negative refresh margin", func(t *testing.T) {
		t.Setenv("FTS_REFRESH_MARGIN", "-1s")
		_, err := Load()
		require.ErrorIs(t, err, ErrInvalidDuration)
	})

	t.Run("unparsable duration", func(t *testing.T) {
		t.Setenv("FTS_REFRESH_TICK", "soon")
		_, err := Load()
		require.Error(t, err)
	})
}
