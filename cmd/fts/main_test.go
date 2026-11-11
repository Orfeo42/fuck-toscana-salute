package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"fuck-toscana-salute/internal/config"
)

func TestRun(t *testing.T) {
	keyring.MockInit()

	t.Run("stops cleanly on cancel", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "127.0.0.1:0")
		t.Setenv("FTS_PROFILE_DIR", t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- run(ctx) }()

		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("run did not return after cancel")
		}
	})

	t.Run("rejects non-loopback address", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "0.0.0.0:8787")

		err := run(context.Background())

		require.ErrorIs(t, err, config.ErrNonLoopbackAddr)
	})

	t.Run("port already in use", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, ln.Close()) })
		t.Setenv("FTS_ADDR", ln.Addr().String())

		err = run(context.Background())

		require.Error(t, err)
	})
}

func TestServe(t *testing.T) {
	keyring.MockInit()

	t.Run("routes api and web ui", func(t *testing.T) {
		t.Setenv("FTS_ADDR", "127.0.0.1:0")
		t.Setenv("FTS_PROFILE_DIR", t.TempDir())
		cfg, err := config.Load()
		require.NoError(t, err)
		ln, err := net.Listen("tcp", cfg.Addr)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- serve(ctx, cfg, ln) }()
		base := "http://" + ln.Addr().String()

		client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
		root, err := client.Get(base + "/")
		require.NoError(t, err)
		require.NoError(t, root.Body.Close())
		session, err := client.Get(base + "/api/session")
		require.NoError(t, err)
		require.NoError(t, session.Body.Close())

		assert.NotEmpty(t, root.Header.Get("Content-Security-Policy"))
		assert.Equal(t, http.StatusOK, session.StatusCode)
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("serve did not return after cancel")
		}
	})

	t.Run("mounts google status and callback", func(t *testing.T) {
		clientFile := filepath.Join(t.TempDir(), "client.json")
		data := `{"installed":{"client_id":"id","client_secret":"secret","auth_uri":"https://accounts.example.test/auth","token_uri":"https://accounts.example.test/token","redirect_uris":["http://localhost"]}}`
		require.NoError(t, os.WriteFile(clientFile, []byte(data), 0o600))
		t.Setenv("FTS_ADDR", "127.0.0.1:0")
		t.Setenv("FTS_PROFILE_DIR", t.TempDir())
		t.Setenv("FTS_GOOGLE_CLIENT_FILE", clientFile)
		cfg, err := config.Load()
		require.NoError(t, err)
		ln, err := net.Listen("tcp", cfg.Addr)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- serve(ctx, cfg, ln) }()
		base := "http://" + ln.Addr().String()

		client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
		status, err := client.Get(base + "/api/google")
		require.NoError(t, err)
		body, err := io.ReadAll(status.Body)
		require.NoError(t, err)
		require.NoError(t, status.Body.Close())
		callback, err := client.Get(base + "/api/google/callback?state=unknown&code=x")
		require.NoError(t, err)
		require.NoError(t, callback.Body.Close())

		assert.Equal(t, http.StatusOK, status.StatusCode)
		assert.Contains(t, string(body), `"configured":true`)
		assert.Contains(t, string(body), `"connected":false`)
		assert.Equal(t, http.StatusBadRequest, callback.StatusCode)
		assert.Equal(t, "default-src 'none'", callback.Header.Get("Content-Security-Policy"))
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("serve did not return after cancel")
		}
	})

	t.Run("invalid google client file fails startup", func(t *testing.T) {
		clientFile := filepath.Join(t.TempDir(), "client.json")
		require.NoError(t, os.WriteFile(clientFile, []byte("{}"), 0o600))
		t.Setenv("FTS_ADDR", "127.0.0.1:0")
		t.Setenv("FTS_PROFILE_DIR", t.TempDir())
		t.Setenv("FTS_GOOGLE_CLIENT_FILE", clientFile)
		cfg, err := config.Load()
		require.NoError(t, err)
		ln, err := net.Listen("tcp", cfg.Addr)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })

		require.Error(t, serve(context.Background(), cfg, ln))
	})
}
