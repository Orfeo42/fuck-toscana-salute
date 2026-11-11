package loginflow

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"

	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/session"
)

type Phase string

const (
	PhaseIdle    Phase = "idle"
	PhaseWaiting Phase = "waiting"
	PhaseFailed  Phase = "failed"
)

type State struct {
	Phase Phase
	Error string
}

type flowError string

func (e flowError) Error() string { return string(e) }

const ErrLoginInProgress = flowError("loginflow: login already in progress")

//go:generate moq -out deps_mock.go . Launcher:MockLauncher Sink:MockSink
type Launcher interface {
	Login(ctx context.Context, portal session.Portal) (login.Result, error)
}

type Sink interface {
	Put(portal session.Portal, tokens session.Tokens)
	SetIdentity(id session.Identity)
}

type Flow struct {
	launcher Launcher
	sink     Sink
	mu       sync.Mutex
	states   map[session.Portal]State
}

func New(launcher Launcher, sink Sink) *Flow {
	return &Flow{
		launcher: launcher,
		sink:     sink,
		states: map[session.Portal]State{
			session.PortalCUP: {Phase: PhaseIdle},
			session.PortalFSE: {Phase: PhaseIdle},
		},
	}
}

func (f *Flow) Start(ctx context.Context, portal session.Portal) error {
	f.mu.Lock()
	if f.states[portal].Phase == PhaseWaiting {
		f.mu.Unlock()
		return ErrLoginInProgress
	}
	f.states[portal] = State{Phase: PhaseWaiting}
	f.mu.Unlock()

	go func(ctx context.Context) {
		res, err := f.launcher.Login(ctx, portal)
		if err != nil {
			kind := "error"
			if errors.Is(err, login.ErrLoginTimeout) {
				kind = "timeout"
			}
			if errors.Is(err, login.ErrWindowClosed) {
				kind = "window_closed"
			}
			f.mu.Lock()
			f.states[portal] = State{Phase: PhaseFailed, Error: kind}
			f.mu.Unlock()
			slog.WarnContext(ctx, "login failed", slog.String("portal", string(portal)), slog.String("error", err.Error()))
			return
		}
		f.sink.Put(res.Portal, res.Tokens)
		f.sink.SetIdentity(res.Identity)
		f.mu.Lock()
		f.states[portal] = State{Phase: PhaseIdle}
		f.mu.Unlock()
		slog.InfoContext(ctx, "login completed", slog.String("portal", string(portal)))
	}(context.WithoutCancel(ctx))

	return nil
}

func (f *Flow) States() map[session.Portal]State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.states)
}
