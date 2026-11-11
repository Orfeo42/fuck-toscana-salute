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

func okSink() *MockSink {
	return &MockSink{
		PutFunc:         func(portal session.Portal, tokens session.Tokens) {},
		SetIdentityFunc: func(id session.Identity) {},
	}
}

func resultFor(portal session.Portal) login.Result {
	return login.Result{
		Portal:   portal,
		Tokens:   session.Tokens{Access: "access-" + string(portal), Refresh: "refresh-" + string(portal)},
		Identity: session.Identity{CodiceFiscale: "AAABBB00A00A000A", TesseraSanitaria: "123"},
	}
}

func TestStartAll(t *testing.T) {
	t.Run("success puts both results and both go idle", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginAllFunc: func(ctx context.Context) ([]login.Result, error) {
				return []login.Result{resultFor(session.PortalCUP), resultFor(session.PortalFSE)}, nil
			},
		}
		sink := okSink()
		f := New(launcher, sink)

		require.NoError(t, f.StartAll(context.Background()))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP).Phase == PhaseIdle && stateOf(f, session.PortalFSE).Phase == PhaseIdle
		}, time.Second, time.Millisecond)
		assert.Len(t, sink.PutCalls(), 2)
		assert.Len(t, sink.SetIdentityCalls(), 2)
	})

	t.Run("partial result marks missing portal failed with mapped kind", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginAllFunc: func(ctx context.Context) ([]login.Result, error) {
				return []login.Result{resultFor(session.PortalCUP)}, login.ErrLoginTimeout
			},
		}
		sink := okSink()
		f := New(launcher, sink)

		require.NoError(t, f.StartAll(context.Background()))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalFSE) == State{Phase: PhaseFailed, Error: "timeout"}
		}, time.Second, time.Millisecond)
		assert.Equal(t, State{Phase: PhaseIdle}, stateOf(f, session.PortalCUP))
		assert.Len(t, sink.PutCalls(), 1)
	})

	t.Run("error with no results fails both", func(t *testing.T) {
		launcher := &MockLauncher{
			LoginAllFunc: func(ctx context.Context) ([]login.Result, error) {
				return nil, login.ErrWindowClosed
			},
		}
		f := New(launcher, okSink())

		require.NoError(t, f.StartAll(context.Background()))

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP) == State{Phase: PhaseFailed, Error: "window_closed"} &&
				stateOf(f, session.PortalFSE) == State{Phase: PhaseFailed, Error: "window_closed"}
		}, time.Second, time.Millisecond)
	})

	t.Run("start while waiting returns ErrLoginInProgress", func(t *testing.T) {
		release := make(chan struct{})
		launcher := &MockLauncher{
			LoginAllFunc: func(ctx context.Context) ([]login.Result, error) {
				<-release
				return nil, errors.New("boom")
			},
		}
		f := New(launcher, okSink())

		require.NoError(t, f.StartAll(context.Background()))
		require.ErrorIs(t, f.StartAll(context.Background()), ErrLoginInProgress)

		close(release)
		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP).Phase == PhaseFailed
		}, time.Second, time.Millisecond)
	})

	t.Run("cancelled request ctx does not cancel the login", func(t *testing.T) {
		release := make(chan struct{})
		launcher := &MockLauncher{
			LoginAllFunc: func(ctx context.Context) ([]login.Result, error) {
				<-release
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return []login.Result{resultFor(session.PortalCUP), resultFor(session.PortalFSE)}, nil
			},
		}
		f := New(launcher, okSink())
		ctx, cancel := context.WithCancel(context.Background())

		require.NoError(t, f.StartAll(ctx))
		cancel()
		close(release)

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP).Phase == PhaseIdle && stateOf(f, session.PortalFSE).Phase == PhaseIdle
		}, time.Second, time.Millisecond)
	})
}

func expiredStatus(portal session.Portal) *MockStatusSource {
	return &MockStatusSource{
		StatusFunc: func() session.Status {
			return session.Status{Portals: []session.PortalStatus{{Portal: portal, State: session.StateExpired}}}
		},
	}
}

func TestSupervise(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("expired portal triggers silent login and returns to idle", func(t *testing.T) {
		launcher := &MockLauncher{
			SilentFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return resultFor(portal), nil
			},
		}
		sink := okSink()
		f := New(launcher, sink)
		f.now = func() time.Time { return fixedNow }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, expiredStatus(session.PortalCUP), 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return len(sink.PutCalls()) == 1 && stateOf(f, session.PortalCUP).Phase == PhaseIdle
		}, time.Second, time.Millisecond)
		assert.Equal(t, session.PortalCUP, sink.PutCalls()[0].Portal)
		assert.Len(t, sink.SetIdentityCalls(), 1)
	})

	t.Run("interaction required sets failed interaction_required", func(t *testing.T) {
		launcher := &MockLauncher{
			SilentFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, login.ErrInteractionRequired
			},
		}
		f := New(launcher, okSink())
		f.now = func() time.Time { return fixedNow }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, expiredStatus(session.PortalFSE), 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalFSE) == State{Phase: PhaseFailed, Error: "interaction_required"}
		}, time.Second, time.Millisecond)
	})

	t.Run("other silent error sets failed error", func(t *testing.T) {
		launcher := &MockLauncher{
			SilentFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, errors.New("boom")
			},
		}
		f := New(launcher, okSink())
		f.now = func() time.Time { return fixedNow }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, expiredStatus(session.PortalCUP), 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return stateOf(f, session.PortalCUP) == State{Phase: PhaseFailed, Error: "error"}
		}, time.Second, time.Millisecond)
	})

	t.Run("retry is suppressed within silentRetry", func(t *testing.T) {
		launcher := &MockLauncher{
			SilentFunc: func(ctx context.Context, portal session.Portal) (login.Result, error) {
				return login.Result{}, login.ErrInteractionRequired
			},
		}
		f := New(launcher, okSink())
		f.now = func() time.Time { return fixedNow }
		status := expiredStatus(session.PortalCUP)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, status, 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return len(launcher.SilentCalls()) == 1
		}, time.Second, time.Millisecond)
		callsBefore := len(status.StatusCalls())
		require.Eventually(t, func() bool {
			return len(status.StatusCalls()) >= callsBefore+3
		}, time.Second, time.Millisecond)
		assert.Len(t, launcher.SilentCalls(), 1)
	})

	t.Run("logged out portal is skipped", func(t *testing.T) {
		launcher := &MockLauncher{}
		status := &MockStatusSource{
			StatusFunc: func() session.Status {
				return session.Status{Portals: []session.PortalStatus{{Portal: session.PortalCUP, State: session.StateLoggedOut}}}
			},
		}
		f := New(launcher, okSink())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, status, 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return len(status.StatusCalls()) >= 3
		}, time.Second, time.Millisecond)
		assert.Empty(t, launcher.SilentCalls())
	})

	t.Run("busy browser skips the tick", func(t *testing.T) {
		launcher := &MockLauncher{}
		status := expiredStatus(session.PortalCUP)
		f := New(launcher, okSink())
		f.browserMu.Lock()
		defer f.browserMu.Unlock()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go f.Supervise(ctx, status, 5*time.Millisecond)

		require.Eventually(t, func() bool {
			return len(status.StatusCalls()) >= 3
		}, time.Second, time.Millisecond)
		assert.Empty(t, launcher.SilentCalls())
	})

	t.Run("stops on ctx cancel", func(t *testing.T) {
		f := New(&MockLauncher{}, okSink())
		status := &MockStatusSource{
			StatusFunc: func() session.Status { return session.Status{} },
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			f.Supervise(ctx, status, 5*time.Millisecond)
			close(done)
		}()
		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Supervise did not stop")
		}
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
