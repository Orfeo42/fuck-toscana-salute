package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("FTS_SECRET_KEY_FILE", "/run/secrets/fts_key")
	t.Setenv("FTS_DATABASE_HOST", "db")
	t.Setenv("FTS_DATABASE_PASSWORD_FILE", "/run/secrets/fts_db_password")
}

func unsetEnv(t *testing.T, name string) {
	t.Helper()
	old, ok := os.LookupEnv(name)
	require.NoError(t, os.Unsetenv(name))
	t.Cleanup(func() {
		if ok {
			require.NoError(t, os.Setenv(name, old))
		}
	})
}

func TestLoad(t *testing.T) {
	setRequired(t)

	t.Run("defaults", func(t *testing.T) {
		dataHome := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dataHome)
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, Config{
			Addr:           "127.0.0.1:8787",
			BrowserChannel: "chrome",
			LoginTimeout:   10 * time.Minute,
			RefreshMargin:  60 * time.Second,
			RefreshTick:    15 * time.Second,
			ProfileDir:     filepath.Join(dataHome, "fuck-toscana-salute", "chrome-profile"),
			DataDir:        filepath.Join(dataHome, "fuck-toscana-salute"),
			LoginMode:      LoginModeWindow,
			SecretKeyFile:  "/run/secrets/fts_key",

			DatabaseHost:         "db",
			DatabasePort:         5432,
			DatabaseName:         "fts",
			DatabaseUser:         "fts",
			DatabasePasswordFile: "/run/secrets/fts_db_password",
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

	t.Run("database overrides", func(t *testing.T) {
		t.Setenv("FTS_DATABASE_PORT", "6543")
		t.Setenv("FTS_DATABASE_NAME", "other")
		t.Setenv("FTS_DATABASE_USER", "me")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, 6543, got.DatabasePort)
		assert.Equal(t, "other", got.DatabaseName)
		assert.Equal(t, "me", got.DatabaseUser)
	})

	t.Run("missing database host rejected", func(t *testing.T) {
		unsetEnv(t, "FTS_DATABASE_HOST")
		_, err := Load()
		require.Error(t, err)
	})

	t.Run("missing database password file rejected", func(t *testing.T) {
		unsetEnv(t, "FTS_DATABASE_PASSWORD_FILE")
		_, err := Load()
		require.Error(t, err)
	})

	t.Run("missing secret key file rejected", func(t *testing.T) {
		unsetEnv(t, "FTS_SECRET_KEY_FILE")
		_, err := Load()
		require.Error(t, err)
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

	t.Run("unspecified address allowed with listen any", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "0.0.0.0:8787")
		t.Setenv("FTS_LISTEN_ANY", "true")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "0.0.0.0:8787", got.Addr)
	})

	t.Run("bookmarklet login mode", func(t *testing.T) {
		t.Setenv("FTS_LOGIN_MODE", "bookmarklet")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, LoginModeBookmarklet, got.LoginMode)
	})

	t.Run("unknown login mode rejected", func(t *testing.T) {
		t.Setenv("FTS_LOGIN_MODE", "popup")
		_, err := Load()
		require.ErrorIs(t, err, ErrInvalidLoginMode)
	})

	t.Run("explicit data dir and secret key file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("FTS_DATA_DIR", dir)
		t.Setenv("FTS_SECRET_KEY_FILE", "/run/secrets/fts-key")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, dir, got.DataDir)
		assert.Equal(t, filepath.Join(dir, "chrome-profile"), got.ProfileDir)
		assert.Equal(t, "/run/secrets/fts-key", got.SecretKeyFile)
	})

	t.Run("relative data dir rejected", func(t *testing.T) {
		t.Setenv("FTS_DATA_DIR", "data")
		_, err := Load()
		require.ErrorIs(t, err, ErrInvalidDataDir)
	})

	t.Run("loopback viewer address", func(t *testing.T) {
		t.Setenv("FTS_VIEWER_ADDR", "127.0.0.1:6080")
		got, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6080", got.ViewerAddr)
	})

	t.Run("non-loopback viewer address rejected", func(t *testing.T) {
		t.Setenv("FTS_VIEWER_ADDR", "10.0.0.2:6080")
		_, err := Load()
		require.ErrorIs(t, err, ErrNonLoopbackViewer)
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

func TestDatabaseDSN(t *testing.T) {
	cfg := func(t *testing.T, password string) Config {
		t.Helper()
		file := filepath.Join(t.TempDir(), "password")
		require.NoError(t, os.WriteFile(file, []byte(password), 0o600))
		return Config{DatabaseHost: "db", DatabasePort: 5432, DatabaseName: "fts", DatabaseUser: "fts", DatabasePasswordFile: file}
	}

	t.Run("builds dsn and trims whitespace", func(t *testing.T) {
		got, err := cfg(t, "secret\n").DatabaseDSN()
		require.NoError(t, err)
		assert.Equal(t, "postgres://fts:secret@db:5432/fts?sslmode=disable", got)
	})

	t.Run("escapes the password", func(t *testing.T) {
		got, err := cfg(t, "p@ss/w:rd").DatabaseDSN()
		require.NoError(t, err)
		assert.Equal(t, "postgres://fts:p%40ss%2Fw%3Ard@db:5432/fts?sslmode=disable", got)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := Config{DatabasePasswordFile: filepath.Join(t.TempDir(), "absent")}.DatabaseDSN()
		require.ErrorIs(t, err, ErrDatabasePassword)
	})

	t.Run("empty file", func(t *testing.T) {
		_, err := cfg(t, " \n").DatabaseDSN()
		require.ErrorIs(t, err, ErrDatabasePasswordEmpty)
	})
}
