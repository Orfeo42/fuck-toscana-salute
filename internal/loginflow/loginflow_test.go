package loginflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/session"
)

func stateOf(f *Flow, portal session.Portal) State {
	return f.States()[portal]
}

func TestNew(t *testing.T) {
	t.Run("both portals start idle", func(t *testing.T) {
		f := New(&MockLauncher{}, &MockSink{})

		states := f.States()
		assert.Equal(t, State{Phase: PhaseIdle}, states[session.PortalCUP])
		assert.Equal(t, State{Phase: PhaseIdle}, states[session.PortalFSE])
	})
}

func TestStart(t *testing.T) {
	t.Run("success stores tokens and identity and returns to idle", func(t *testing.T) {
		res := login.Result{
			Portal:   session.PortalCUP,
			Tokens:   session.Tokens{Access: "access-1", Refresh: "refresh-1"},
			Identity: session.Identity{CodiceFiscale: "AAABBB00A00A000A", TesseraSanitaria: "123"},
		}
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return res, nil
			},
		}
		sink := &MockSink{
			PutFunc:         func(portal session.Portal, tokens session.Tokens) {},
			SetIdentityFunc: func(id session.Identity) {},
		}
		f := New(launcher, sink)

		require.NoError(t, f.Start(context.Background(), session.PortalCUP))

		require.Eventually(t, func() bool {
			return len(sink.SetIdentityCalls()) == 1 && stateOf(f, session.PortalCUP).Phase == PhaseIdle
		}, time.Second, time.Millisecond)
		require.Len(t, sink.PutCalls(), 1)
		assert.Equal(t, session.PortalCUP, sink.PutCalls()[0].Portal)
		assert.Equal(t, res.Tokens, sink.PutCalls()[0].Tokens)
		assert.Equal(t, res.Identity, sink.SetIdentityCalls()[0].ID)
	})

	t.Run("timeout error sets failed timeout", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, login.ErrLoginTimeout
			},
		}
		f := New(launcher, &MockSink{})

		require.NoError(t, f.Start(context.Background(), session.PortalFSE))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalFSE) == State{Phase: PhaseFailed, Error: "timeout"}
		}, time.Second, time.Millisecond)
	})

	t.Run("window closed error sets failed window_closed", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, login.ErrWindowClosed
			},
		}
		f := New(launcher, &MockSink{})

		require.NoError(t, f.Start(context.Background(), session.PortalFSE))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalFSE) == State{Phase: PhaseFailed, Error: "window_closed"}
		}, time.Second, time.Millisecond)
	})

	t.Run("generic error sets failed error", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, errors.New("boom")
			},
		}
		f := New(launcher, &MockSink{})

		require.NoError(t, f.Start(context.Background(), session.PortalCUP))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP) == State{Phase: PhaseFailed, Error: "error"}
		}, time.Second, time.Millisecond)
	})

	t.Run("second start while waiting returns ErrLoginInProgress", func(t *testing.T) {
		release := make(chan struct{})
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				<-release
				return login.Result{}, login.ErrLoginTimeout
			},
		}
		f := New(launcher, &MockSink{})

		require.NoError(t, f.Start(context.Background(), session.PortalCUP))
		err := f.Start(context.Background(), session.PortalCUP)
		require.ErrorIs(t, err, ErrLoginInProgress)

		close(release)
		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP).Phase == PhaseFailed
		}, time.Second, time.Millisecond)
	})

	t.Run("cancelled request ctx does not cancel the login", func(t *testing.T) {
		release := make(chan struct{})
		launcher := &MockLauncher{
			LoginFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				<-release
				return login.Result{}, ctx.Err()
			},
		}
		sink := &MockSink{
			PutFunc:         func(portal session.Portal, tokens session.Tokens) {},
			SetIdentityFunc: func(id session.Identity) {},
		}
		f := New(launcher, sink)
		ctx, cancel := context.WithCancel(context.Background())

		require.NoError(t, f.Start(ctx, session.PortalCUP))
		cancel()
		close(release)

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP).Phase == PhaseIdle
		}, time.Second, time.Millisecond)
		assert.Len(t, sink.PutCalls(), 1)
	})
}

func TestStates(t *testing.T) {
	t.Run("returned map is a copy", func(t *testing.T) {
		f := New(&MockLauncher{}, &MockSink{})

		states := f.States()
		states[session.PortalCUP] = State{Phase: PhaseFailed, Error: "error"}

		assert.Equal(t, State{Phase: PhaseIdle}, f.States()[session.PortalCUP])
	})
}
