package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"fuck-toscana-salute/internal/config"
	"fuck-toscana-salute/internal/store"
	"fuck-toscana-salute/internal/vault"
)

const dbPassword = "test"

var (
	dbHost string
	dbPort int
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("fts"),
		postgres.WithUsername("fts"),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container:", err)
		return 1
	}
	defer func() { _ = ctr.Terminate(ctx) }()
	dbHost, err = ctr.Host(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "container host:", err)
		return 1
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "container port:", err)
		return 1
	}
	dbPort = int(port.Num())
	return m.Run()
}

func testKey() []byte {
	return bytes.Repeat([]byte{7}, 32)
}

func setEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")
	require.NoError(t, os.WriteFile(keyFile, []byte(base64.StdEncoding.EncodeToString(testKey())), 0o600))
	passwordFile := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(passwordFile, []byte(dbPassword+"\n"), 0o600))
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	t.Setenv("FTS_ADDR", "127.0.0.1:0")
	t.Setenv("FTS_PROFILE_DIR", filepath.Join(dir, "profile"))
	t.Setenv("FTS_DATA_DIR", dataDir)
	t.Setenv("FTS_SECRET_KEY_FILE", keyFile)
	t.Setenv("FTS_DATABASE_HOST", dbHost)
	t.Setenv("FTS_DATABASE_PORT", strconv.Itoa(dbPort))
	t.Setenv("FTS_DATABASE_PASSWORD_FILE", passwordFile)
	return dataDir
}

func startServe(t *testing.T, cfg config.Config) (string, context.CancelFunc, chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", cfg.Addr)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, ln) }()
	return "http://" + ln.Addr().String(), cancel, done
}

func stopServe(t *testing.T, cancel context.CancelFunc, done chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after cancel")
	}
}

func TestRun(t *testing.T) {
	t.Run("stops cleanly on cancel", func(t *testing.T) {
		setEnv(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- run(ctx) }()

		time.Sleep(2 * time.Second)
		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("run did not return after cancel")
		}
	})

	t.Run("rejects non-loopback address", func(t *testing.T) {
		setEnv(t)
		t.Setenv("FTS_ADDR", "0.0.0.0:8787")

		err := run(context.Background())

		require.ErrorIs(t, err, config.ErrNonLoopbackAddr)
	})

	t.Run("port already in use", func(t *testing.T) {
		setEnv(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, ln.Close()) })
		t.Setenv("FTS_ADDR", ln.Addr().String())

		err = run(context.Background())

		require.Error(t, err)
	})
}

func TestServe(t *testing.T) {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	t.Run("routes api and web ui", func(t *testing.T) {
		setEnv(t)
		cfg, err := config.Load()
		require.NoError(t, err)
		base, cancel, done := startServe(t, cfg)

		root, err := client.Get(base + "/")
		require.NoError(t, err)
		require.NoError(t, root.Body.Close())
		session, err := client.Get(base + "/api/session")
		require.NoError(t, err)
		require.NoError(t, session.Body.Close())

		assert.NotEmpty(t, root.Header.Get("Content-Security-Policy"))
		assert.Equal(t, http.StatusOK, session.StatusCode)
		stopServe(t, cancel, done)
	})

	t.Run("mounts google status, settings and callback", func(t *testing.T) {
		setEnv(t)
		cfg, err := config.Load()
		require.NoError(t, err)
		base, cancel, done := startServe(t, cfg)

		status, err := client.Get(base + "/api/google")
		require.NoError(t, err)
		body, err := io.ReadAll(status.Body)
		require.NoError(t, err)
		require.NoError(t, status.Body.Close())
		settings, err := client.Get(base + "/api/google/settings")
		require.NoError(t, err)
		require.NoError(t, settings.Body.Close())
		callback, err := client.Get(base + "/api/google/callback?state=unknown&code=x")
		require.NoError(t, err)
		require.NoError(t, callback.Body.Close())

		assert.Equal(t, http.StatusOK, status.StatusCode)
		assert.Contains(t, string(body), `"connected":false`)
		assert.Equal(t, http.StatusOK, settings.StatusCode)
		assert.Equal(t, http.StatusBadRequest, callback.StatusCode)
		assert.Equal(t, "default-src 'none'", callback.Header.Get("Content-Security-Policy"))
		stopServe(t, cancel, done)
	})

	t.Run("serves reports endpoint behind login", func(t *testing.T) {
		setEnv(t)
		cfg, err := config.Load()
		require.NoError(t, err)
		base, cancel, done := startServe(t, cfg)

		resp, err := client.Get(base + "/api/reports")
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		stopServe(t, cancel, done)
	})

	t.Run("migrates the legacy secrets file on startup", func(t *testing.T) {
		dataDir := setEnv(t)
		legacy := filepath.Join(dataDir, secretsFile)
		backend, err := vault.NewFileBackend(legacy, testKey())
		require.NoError(t, err)
		require.NoError(t, backend.Set(secretService, googleUser, `{"access_token":"x"}`))
		cfg, err := config.Load()
		require.NoError(t, err)
		_, cancel, done := startServe(t, cfg)
		require.Eventually(t, func() bool {
			_, err := os.Stat(legacy + migratedSuffix)
			return err == nil
		}, 10*time.Second, 50*time.Millisecond)
		stopServe(t, cancel, done)

		assert.NoFileExists(t, legacy)
	})

	t.Run("missing key file fails startup", func(t *testing.T) {
		setEnv(t)
		t.Setenv("FTS_SECRET_KEY_FILE", filepath.Join(t.TempDir(), "absent"))
		cfg, err := config.Load()
		require.NoError(t, err)
		ln, err := net.Listen("tcp", cfg.Addr)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })

		require.Error(t, serve(context.Background(), cfg, ln))
	})

	t.Run("unreadable database password fails startup", func(t *testing.T) {
		setEnv(t)
		t.Setenv("FTS_DATABASE_PASSWORD_FILE", filepath.Join(t.TempDir(), "absent"))
		cfg, err := config.Load()
		require.NoError(t, err)
		ln, err := net.Listen("tcp", cfg.Addr)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })

		require.ErrorIs(t, serve(context.Background(), cfg, ln), config.ErrDatabasePassword)
	})
}

type memoryBackend map[string]string

func (m memoryBackend) Get(service, user string) (string, error) {
	value, ok := m[service+"/"+user]
	if !ok {
		return "", vault.ErrNotFound
	}
	return value, nil
}

func (m memoryBackend) Set(service, user, value string) error {
	m[service+"/"+user] = value
	return nil
}

func (m memoryBackend) Delete(service, user string) error {
	delete(m, service+"/"+user)
	return nil
}

func TestMigrateSecrets(t *testing.T) {
	ctx := context.Background()

	t.Run("no legacy file is a no-op", func(t *testing.T) {
		dst := memoryBackend{}

		err := migrateSecrets(ctx, filepath.Join(t.TempDir(), secretsFile), testKey(), dst)

		require.NoError(t, err)
		assert.Empty(t, dst)
	})

	t.Run("copies session and google then renames the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), secretsFile)
		src, err := vault.NewFileBackend(path, testKey())
		require.NoError(t, err)
		require.NoError(t, src.Set(secretService, sessionUser, "s"))
		require.NoError(t, src.Set(secretService, googleUser, "g"))
		dst := memoryBackend{}

		require.NoError(t, migrateSecrets(ctx, path, testKey(), dst))

		assert.Equal(t, memoryBackend{secretService + "/" + sessionUser: "s", secretService + "/" + googleUser: "g"}, dst)
		assert.NoFileExists(t, path)
		assert.FileExists(t, path+migratedSuffix)
	})

	t.Run("existing store entry wins", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), secretsFile)
		src, err := vault.NewFileBackend(path, testKey())
		require.NoError(t, err)
		require.NoError(t, src.Set(secretService, sessionUser, "old"))
		dst := memoryBackend{secretService + "/" + sessionUser: "new"}

		require.NoError(t, migrateSecrets(ctx, path, testKey(), dst))

		assert.Equal(t, "new", dst[secretService+"/"+sessionUser])
	})

	t.Run("wrong key keeps the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), secretsFile)
		src, err := vault.NewFileBackend(path, testKey())
		require.NoError(t, err)
		require.NoError(t, src.Set(secretService, sessionUser, "s"))

		err = migrateSecrets(ctx, path, bytes.Repeat([]byte{9}, 32), memoryBackend{})

		require.Error(t, err)
		assert.FileExists(t, path)
	})

	t.Run("invalid key length", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), secretsFile)
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

		require.ErrorIs(t, migrateSecrets(ctx, path, []byte("short"), memoryBackend{}), vault.ErrInvalidKey)
	})
}

func TestFreshness(t *testing.T) {
	t.Run("reads fetch times from the store", func(t *testing.T) {
		setEnv(t)
		cfg, err := config.Load()
		require.NoError(t, err)
		dsn, err := cfg.DatabaseDSN()
		require.NoError(t, err)
		ctx := context.Background()
		st, err := store.Open(ctx, dsn, testKey())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, st.Close()) })
		at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
		require.NoError(t, st.Put(ctx, store.Record{Source: "freshness.test", Subject: "SUBJ", FetchedAt: at, Status: 200, Payload: []byte("{}")}))

		got, err := freshness{st: st}.FetchedAt(ctx, "freshness.test")

		require.NoError(t, err)
		assert.True(t, at.Equal(got["SUBJ"]))
	})
}
