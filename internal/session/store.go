package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var portals = []Portal{PortalCUP, PortalFSE}

type Store struct {
	mu        sync.Mutex
	refreshMu sync.Mutex
	refresher Refresher
	margin    time.Duration
	tokens    map[Portal]Tokens
	expired   map[Portal]bool
	identity  Identity
	now       func() time.Time
}

func NewStore(refresher Refresher, refreshMargin time.Duration) *Store {
	return &Store{
		refresher: refresher,
		margin:    refreshMargin,
		tokens:    map[Portal]Tokens{},
		expired:   map[Portal]bool{},
		now:       time.Now,
	}
}

func (s *Store) Put(portal Portal, tokens Tokens) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[portal] = tokens
	delete(s.expired, portal)
}

func (s *Store) SetIdentity(id Identity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id.CodiceFiscale != "" {
		s.identity.CodiceFiscale = id.CodiceFiscale
	}
	if id.TesseraSanitaria != "" {
		s.identity.TesseraSanitaria = id.TesseraSanitaria
	}
}

func (s *Store) Identity() Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.identity
}

func (s *Store) fresh(t Tokens) bool {
	return s.now().Add(s.margin).Before(t.AccessExpiresAt)
}

func (s *Store) AccessToken(ctx context.Context, portal Portal) (string, error) {
	s.mu.Lock()
	t, ok := s.tokens[portal]
	expired := s.expired[portal]
	s.mu.Unlock()

	if !ok && expired {
		return "", ErrSessionExpired
	}
	if !ok {
		return "", ErrNotLoggedIn
	}
	if s.fresh(t) {
		return t.Access, nil
	}

	if err := s.refresh(ctx, portal); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok = s.tokens[portal]
	if !ok {
		return "", ErrNotLoggedIn
	}
	return t.Access, nil
}

func (s *Store) refresh(ctx context.Context, portal Portal) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	s.mu.Lock()
	t, ok := s.tokens[portal]
	s.mu.Unlock()
	if !ok {
		return ErrNotLoggedIn
	}
	if s.fresh(t) {
		return nil
	}
	if !t.RefreshExpiresAt.After(s.now()) {
		s.markExpired(portal)
		return ErrSessionExpired
	}

	next, err := s.refresher.Refresh(ctx, portal, t.Refresh)
	if errors.Is(err, ErrRefreshRejected) {
		s.markExpired(portal)
		return ErrSessionExpired
	}
	if err != nil {
		return fmt.Errorf("refresh %s: %w", portal, err)
	}

	s.Put(portal, next)
	return nil
}

func (s *Store) markExpired(portal Portal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, portal)
	s.expired[portal] = true
}

func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := Status{Portals: make([]PortalStatus, 0, len(portals))}
	for _, p := range portals {
		ps := PortalStatus{Portal: p, State: StateLoggedOut}
		if t, ok := s.tokens[p]; ok {
			ps.State = StateActive
			ps.AccessExpiresAt = t.AccessExpiresAt
			ps.RefreshExpiresAt = t.RefreshExpiresAt
		} else if s.expired[p] {
			ps.State = StateExpired
		}
		status.Portals = append(status.Portals, ps)
	}
	_, cupActive := s.tokens[PortalCUP]
	status.GestioneReady = cupActive && s.identity.TesseraSanitaria != ""
	return status
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = map[Portal]Tokens{}
	s.expired = map[Portal]bool{}
	s.identity = Identity{}
}

func (s *Store) Run(ctx context.Context, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		for _, p := range portals {
			s.mu.Lock()
			t, ok := s.tokens[p]
			s.mu.Unlock()
			if !ok || s.fresh(t) {
				continue
			}

			err := s.refresh(ctx, p)
			if errors.Is(err, ErrSessionExpired) {
				slog.WarnContext(ctx, "session expired", slog.String("portal", string(p)))
				continue
			}
			if err != nil {
				slog.WarnContext(ctx, "session refresh failed", slog.String("portal", string(p)), slog.String("error", err.Error()))
				continue
			}
			slog.InfoContext(ctx, "session refreshed", slog.String("portal", string(p)))
		}
	}
}
