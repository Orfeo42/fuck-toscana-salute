package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/config"
)

func TestRun(t *testing.T) {
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
