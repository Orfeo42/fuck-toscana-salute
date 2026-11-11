package loginflow

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"time"

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

//go:generate moq -out deps_mock.go . Launcher:MockLauncher Sink:MockSink StatusSource:MockStatusSource
type Launcher interface {
	Login(ctx context.Context, portal session.Portal) (login.Result, error)
	LoginAll(ctx context.Context) ([]login.Result, error)
	Silent(ctx context.Context, portal session.Portal) (login.Result, error)
}

type StatusSource interface {
	Status() session.Status
}

type Sink interface {
	Put(portal session.Portal, tokens session.Tokens)
	SetIdentity(id session.Identity)
}

const silentRetry = 10 * time.Minute

type Flow struct {
	launcher   Launcher
	sink       Sink
	browserMu  sync.Mutex
	mu         sync.Mutex
	states     map[session.Portal]State
	lastSilent map[session.Portal]time.Time
	now        func() time.Time
}

func New(launcher Launcher, sink Sink) *Flow {
	return &Flow{
		launcher: launcher,
		sink:     sink,
		states: map[session.Portal]State{
			session.PortalCUP: {Phase: PhaseIdle},
			session.PortalFSE: {Phase: PhaseIdle},
		},
		lastSilent: map[session.Portal]time.Time{},
		now:        time.Now,
	}
}

func errorKind(err error) string {
	if errors.Is(err, login.ErrLoginTimeout) {
		return "timeout"
	}
	if errors.Is(err, login.ErrWindowClosed) {
		return "window_closed"
	}
	return "error"
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

func (f *Flow) Import(ctx context.Context, res login.Result) error {
	if err := login.Validate(res, f.now()); err != nil {
		return err
	}

	f.sink.Put(res.Portal, res.Tokens)
	f.sink.SetIdentity(res.Identity)
	f.mu.Lock()
	f.states[res.Portal] = State{Phase: PhaseIdle}
	f.mu.Unlock()
	slog.InfoContext(ctx, "login imported", slog.String("portal", string(res.Portal)))
	return nil
}

func (f *Flow) States() map[session.Portal]State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.states)
}

func (f *Flow) StartAll(ctx context.Context) error {
	f.mu.Lock()
	if f.states[session.PortalCUP].Phase == PhaseWaiting || f.states[session.PortalFSE].Phase == PhaseWaiting {
		f.mu.Unlock()
		return ErrLoginInProgress
	}
	f.states[session.PortalCUP] = State{Phase: PhaseWaiting}
	f.states[session.PortalFSE] = State{Phase: PhaseWaiting}
	f.mu.Unlock()

	go func(ctx context.Context) {
		f.browserMu.Lock()
		results, err := f.launcher.LoginAll(ctx)
		f.browserMu.Unlock()

		done := map[session.Portal]bool{}
		for _, res := range results {
			f.sink.Put(res.Portal, res.Tokens)
			f.sink.SetIdentity(res.Identity)
			done[res.Portal] = true
		}

		f.mu.Lock()
		for _, portal := range []session.Portal{session.PortalCUP, session.PortalFSE} {
			if done[portal] {
				f.states[portal] = State{Phase: PhaseIdle}
				continue
			}
			f.states[portal] = State{Phase: PhaseFailed, Error: errorKind(err)}
		}
		f.mu.Unlock()

		if err != nil {
			slog.WarnContext(ctx, "login all failed", slog.Int("completed", len(results)), slog.String("error", err.Error()))
			return
		}
		slog.InfoContext(ctx, "login all completed")
	}(context.WithoutCancel(ctx))

	return nil
}

func (f *Flow) Supervise(ctx context.Context, status StatusSource, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.silentTick(ctx, status.Status())
		}
	}
}

func (f *Flow) silentTick(ctx context.Context, status session.Status) {
	for _, ps := range status.Portals {
		if ps.State != session.StateExpired {
			continue
		}
		f.mu.Lock()
		waiting := f.states[ps.Portal].Phase == PhaseWaiting
		f.mu.Unlock()
		if waiting || f.now().Sub(f.lastSilent[ps.Portal]) < silentRetry {
			continue
		}
		if !f.browserMu.TryLock() {
			return
		}
		f.lastSilent[ps.Portal] = f.now()
		f.mu.Lock()
		f.states[ps.Portal] = State{Phase: PhaseWaiting}
		f.mu.Unlock()
		res, err := f.launcher.Silent(ctx, ps.Portal)
		f.browserMu.Unlock()

		portalAttr := slog.String("portal", string(ps.Portal))
		if errors.Is(err, login.ErrInteractionRequired) {
			f.mu.Lock()
			f.states[ps.Portal] = State{Phase: PhaseFailed, Error: "interaction_required"}
			f.mu.Unlock()
			slog.InfoContext(ctx, "silent login needs user", portalAttr)
			continue
		}
		if err != nil {
			f.mu.Lock()
			f.states[ps.Portal] = State{Phase: PhaseFailed, Error: "error"}
			f.mu.Unlock()
			slog.WarnContext(ctx, "silent login failed", portalAttr, slog.String("error", err.Error()))
			continue
		}
		f.sink.Put(res.Portal, res.Tokens)
		f.sink.SetIdentity(res.Identity)
		f.mu.Lock()
		f.states[ps.Portal] = State{Phase: PhaseIdle}
		f.mu.Unlock()
		slog.InfoContext(ctx, "silent login completed", portalAttr)
	}
}
