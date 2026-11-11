package session

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func newPersistStore(refresher Refresher, persister Persister) *Store {
	s := NewStore(refresher, persister, time.Minute)
	s.now = func() time.Time { return testNow }
	return s
}

func newTestStore(refresher Refresher) *Store {
	return newPersistStore(refresher, &MockPersister{
		SaveFunc: func(ctx context.Context, snapshot Snapshot) error { return nil },
	})
}

func newRecordingPersister() *MockPersister {
	return &MockPersister{
		SaveFunc: func(ctx context.Context, snapshot Snapshot) error { return nil },
	}
}

func validTokens(access string) Tokens {
	return Tokens{
		Access:           access,
		Refresh:          "refresh-" + access,
		AccessExpiresAt:  testNow.Add(time.Hour),
		RefreshExpiresAt: testNow.Add(24 * time.Hour),
	}
}

func staleTokens(access string) Tokens {
	t := validTokens(access)
	t.AccessExpiresAt = testNow.Add(30 * time.Second)
	return t
}

func TestPut(t *testing.T) {
	t.Run("stores tokens", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalCUP, validTokens("access-1"))

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-1", got)
	})

	t.Run("clears expired flag", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.expired[PortalCUP] = true
		s.Put(PortalCUP, validTokens("access-1"))

		assert.Equal(t, StateActive, s.Status().Portals[0].State)
	})

	t.Run("sets StartedAt and keeps GestioneVerified", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		tokens := validTokens("access-1")
		tokens.GestioneVerified = true
		tokens.StartedAt = testNow.Add(-48 * time.Hour)
		s.Put(PortalCUP, tokens)

		assert.Equal(t, testNow, s.tokens[PortalCUP].StartedAt)
		assert.True(t, s.tokens[PortalCUP].GestioneVerified)
	})

	t.Run("persists snapshot with tokens", func(t *testing.T) {
		persister := newRecordingPersister()
		s := newPersistStore(&MockRefresher{}, persister)
		s.Put(PortalCUP, validTokens("access-1"))

		require.Len(t, persister.SaveCalls(), 1)
		saved := persister.SaveCalls()[0].Snapshot
		assert.Equal(t, "access-1", saved.Tokens[PortalCUP].Access)
		assert.Equal(t, testNow, saved.Tokens[PortalCUP].StartedAt)
	})

	t.Run("persist error does not fail the operation", func(t *testing.T) {
		persister := &MockPersister{
			SaveFunc: func(ctx context.Context, snapshot Snapshot) error { return errors.New("disk full") },
		}
		s := newPersistStore(&MockRefresher{}, persister)
		s.Put(PortalCUP, validTokens("access-1"))

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-1", got)
		assert.Len(t, persister.SaveCalls(), 1)
	})
}

func TestSetIdentity(t *testing.T) {
	t.Run("sets fields", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"})

		assert.Equal(t, Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"}, s.Identity())
	})

	t.Run("empty fields keep previous values", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U"})
		s.SetIdentity(Identity{TesseraSanitaria: "80380000000000000000"})

		assert.Equal(t, Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"}, s.Identity())
	})

	t.Run("non-empty field overwrites", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U"})
		s.SetIdentity(Identity{CodiceFiscale: "VRDLGI85B02F205X"})

		assert.Equal(t, "VRDLGI85B02F205X", s.Identity().CodiceFiscale)
	})

	t.Run("persists identity", func(t *testing.T) {
		persister := newRecordingPersister()
		s := newPersistStore(&MockRefresher{}, persister)
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"})

		require.Len(t, persister.SaveCalls(), 1)
		assert.Equal(t, Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"}, persister.SaveCalls()[0].Snapshot.Identity)
	})
}

func TestAccessToken(t *testing.T) {
	t.Run("not logged in", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})

		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("fresh token returned without refresh", func(t *testing.T) {
		mock := &MockRefresher{}
		s := newTestStore(mock)
		s.Put(PortalCUP, validTokens("access-1"))

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-1", got)
		assert.Empty(t, mock.RefreshCalls())
	})

	t.Run("refresh success", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return validTokens("access-2"), nil
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-2", got)
		require.Len(t, mock.RefreshCalls(), 1)
		assert.Equal(t, PortalCUP, mock.RefreshCalls()[0].Portal)
		assert.Equal(t, "refresh-access-1", mock.RefreshCalls()[0].RefreshToken)
	})

	t.Run("refresh rejected marks session expired", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return Tokens{}, ErrRefreshRejected
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))

		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrSessionExpired)

		_, err = s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrSessionExpired)
		assert.Equal(t, StateExpired, s.Status().Portals[0].State)
	})

	t.Run("transient error keeps tokens", func(t *testing.T) {
		errBoom := errors.New("boom")
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return Tokens{}, errBoom
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))

		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, errBoom)
		assert.NotErrorIs(t, err, ErrSessionExpired)
		assert.Equal(t, StateActive, s.Status().Portals[0].State)
	})

	t.Run("refresh token already expired", func(t *testing.T) {
		mock := &MockRefresher{}
		s := newTestStore(mock)
		tokens := staleTokens("access-1")
		tokens.RefreshExpiresAt = testNow.Add(-time.Second)
		s.Put(PortalCUP, tokens)

		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrSessionExpired)
		assert.Empty(t, mock.RefreshCalls())
		assert.Equal(t, StateExpired, s.Status().Portals[0].State)
	})

	t.Run("refresh carries GestioneVerified and StartedAt and persists", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return validTokens("access-2"), nil
			},
		}
		persister := newRecordingPersister()
		s := newPersistStore(mock, persister)
		old := staleTokens("access-1")
		old.GestioneVerified = true
		s.Put(PortalCUP, old)
		started := s.tokens[PortalCUP].StartedAt
		s.now = func() time.Time { return testNow.Add(10 * time.Minute) }

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-2", got)
		assert.True(t, s.tokens[PortalCUP].GestioneVerified)
		assert.Equal(t, started, s.tokens[PortalCUP].StartedAt)
		require.Len(t, persister.SaveCalls(), 2)
		last := persister.SaveCalls()[1].Snapshot
		assert.Equal(t, "access-2", last.Tokens[PortalCUP].Access)
		assert.True(t, last.Tokens[PortalCUP].GestioneVerified)
		assert.Equal(t, started, last.Tokens[PortalCUP].StartedAt)
	})

	t.Run("expiry persists snapshot without the portal", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return Tokens{}, ErrRefreshRejected
			},
		}
		persister := newRecordingPersister()
		s := newPersistStore(mock, persister)
		s.Put(PortalCUP, staleTokens("access-1"))

		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrSessionExpired)

		require.Len(t, persister.SaveCalls(), 2)
		assert.NotContains(t, persister.SaveCalls()[1].Snapshot.Tokens, PortalCUP)
	})

	t.Run("concurrent callers trigger one refresh", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				time.Sleep(50 * time.Millisecond)
				return validTokens("access-2"), nil
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))

		var wg sync.WaitGroup
		results := make([]string, 2)
		errs := make([]error, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = s.AccessToken(context.Background(), PortalCUP)
			}()
		}
		wg.Wait()

		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		assert.Equal(t, "access-2", results[0])
		assert.Equal(t, "access-2", results[1])
		assert.Len(t, mock.RefreshCalls(), 1)
	})
}

func TestStatus(t *testing.T) {
	t.Run("lists both portals in order as logged out", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})

		status := s.Status()
		require.Len(t, status.Portals, 2)
		assert.Equal(t, PortalCUP, status.Portals[0].Portal)
		assert.Equal(t, PortalFSE, status.Portals[1].Portal)
		assert.Equal(t, StateLoggedOut, status.Portals[0].State)
		assert.Equal(t, StateLoggedOut, status.Portals[1].State)
		assert.False(t, status.GestioneReady)
	})

	t.Run("active fills expiry times", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		tokens := validTokens("access-1")
		s.Put(PortalFSE, tokens)

		ps := s.Status().Portals[1]
		assert.Equal(t, StateActive, ps.State)
		assert.Equal(t, tokens.AccessExpiresAt, ps.AccessExpiresAt)
		assert.Equal(t, tokens.RefreshExpiresAt, ps.RefreshExpiresAt)
	})

	t.Run("expired has no expiry times", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.expired[PortalFSE] = true

		ps := s.Status().Portals[1]
		assert.Equal(t, StateExpired, ps.State)
		assert.True(t, ps.AccessExpiresAt.IsZero())
		assert.True(t, ps.RefreshExpiresAt.IsZero())
	})

	t.Run("gestione not ready without identity", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		verified := validTokens("access-1")
		verified.GestioneVerified = true
		s.Put(PortalCUP, verified)

		assert.False(t, s.Status().GestioneReady)
	})

	t.Run("gestione not ready without CUP", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalFSE, validTokens("access-1"))
		s.SetIdentity(Identity{TesseraSanitaria: "80380000000000000000"})

		assert.False(t, s.Status().GestioneReady)
	})

	t.Run("gestione not ready when flag is false even with tessera", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalCUP, validTokens("access-1"))
		s.SetIdentity(Identity{TesseraSanitaria: "80380000000000000000"})

		assert.False(t, s.Status().GestioneReady)
	})

	t.Run("gestione ready with CUP, flag and tessera", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		verified := validTokens("access-1")
		verified.GestioneVerified = true
		s.Put(PortalCUP, verified)
		s.SetIdentity(Identity{TesseraSanitaria: "80380000000000000000"})

		assert.True(t, s.Status().GestioneReady)
	})
}

func TestClear(t *testing.T) {
	t.Run("wipes tokens, expired flags and identity", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalCUP, validTokens("access-1"))
		s.expired[PortalFSE] = true
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"})

		s.Clear()

		status := s.Status()
		assert.Equal(t, StateLoggedOut, status.Portals[0].State)
		assert.Equal(t, StateLoggedOut, status.Portals[1].State)
		assert.Equal(t, Identity{}, s.Identity())
		_, err := s.AccessToken(context.Background(), PortalCUP)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("saves empty snapshot", func(t *testing.T) {
		persister := newRecordingPersister()
		s := newPersistStore(&MockRefresher{}, persister)
		s.Put(PortalCUP, validTokens("access-1"))
		s.SetIdentity(Identity{CodiceFiscale: "RSSMRA80A01H501U"})

		s.Clear()

		calls := persister.SaveCalls()
		require.Len(t, calls, 3)
		assert.Empty(t, calls[2].Snapshot.Tokens)
		assert.Equal(t, Identity{}, calls[2].Snapshot.Identity)
	})
}

func TestRestore(t *testing.T) {
	t.Run("loads valid tokens and identity", func(t *testing.T) {
		started := testNow.Add(-2 * time.Hour)
		tokens := validTokens("access-1")
		tokens.GestioneVerified = true
		tokens.StartedAt = started
		persister := &MockPersister{
			LoadFunc: func(ctx context.Context) (Snapshot, error) {
				return Snapshot{
					Tokens:   map[Portal]Tokens{PortalCUP: tokens},
					Identity: Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"},
				}, nil
			},
		}
		s := newPersistStore(&MockRefresher{}, persister)

		require.NoError(t, s.Restore(context.Background()))

		assert.Equal(t, tokens, s.tokens[PortalCUP])
		assert.Equal(t, started, s.tokens[PortalCUP].StartedAt)
		assert.Equal(t, Identity{CodiceFiscale: "RSSMRA80A01H501U", TesseraSanitaria: "80380000000000000000"}, s.Identity())
		assert.True(t, s.Status().GestioneReady)
	})

	t.Run("skips portals with expired refresh token", func(t *testing.T) {
		expired := validTokens("access-1")
		expired.RefreshExpiresAt = testNow.Add(-time.Second)
		persister := &MockPersister{
			LoadFunc: func(ctx context.Context) (Snapshot, error) {
				return Snapshot{Tokens: map[Portal]Tokens{PortalCUP: expired, PortalFSE: validTokens("access-2")}}, nil
			},
		}
		s := newPersistStore(&MockRefresher{}, persister)

		require.NoError(t, s.Restore(context.Background()))

		assert.Equal(t, StateLoggedOut, s.Status().Portals[0].State)
		assert.Equal(t, StateActive, s.Status().Portals[1].State)
	})

	t.Run("no snapshot returns nil", func(t *testing.T) {
		persister := &MockPersister{
			LoadFunc: func(ctx context.Context) (Snapshot, error) { return Snapshot{}, ErrNoSnapshot },
		}
		s := newPersistStore(&MockRefresher{}, persister)

		require.NoError(t, s.Restore(context.Background()))
		assert.Equal(t, StateLoggedOut, s.Status().Portals[0].State)
	})

	t.Run("other error is returned", func(t *testing.T) {
		errBoom := errors.New("boom")
		persister := &MockPersister{
			LoadFunc: func(ctx context.Context) (Snapshot, error) { return Snapshot{}, errBoom },
		}
		s := newPersistStore(&MockRefresher{}, persister)

		require.ErrorIs(t, s.Restore(context.Background()), errBoom)
	})

	t.Run("does not save", func(t *testing.T) {
		persister := &MockPersister{
			LoadFunc: func(ctx context.Context) (Snapshot, error) {
				return Snapshot{Tokens: map[Portal]Tokens{PortalCUP: validTokens("access-1")}}, nil
			},
			SaveFunc: func(ctx context.Context, snapshot Snapshot) error { return nil },
		}
		s := newPersistStore(&MockRefresher{}, persister)

		require.NoError(t, s.Restore(context.Background()))
		assert.Empty(t, persister.SaveCalls())
	})
}

func TestRun(t *testing.T) {
	t.Run("refreshed log line contains chain_age", func(t *testing.T) {
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return validTokens("access-2"), nil
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))
		s.now = func() time.Time { return testNow.Add(90 * time.Minute) }

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		s.Run(ctx, 5*time.Millisecond)

		assert.Contains(t, buf.String(), "session refreshed")
		assert.Contains(t, buf.String(), "chain_age=1h30m0s")
	})

	t.Run("refreshes near-expiry token and stops on cancel", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return validTokens("access-2"), nil
			},
		}
		s := newTestStore(mock)
		s.Put(PortalCUP, staleTokens("access-1"))

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			s.Run(ctx, 5*time.Millisecond)
			close(done)
		}()

		require.Eventually(t, func() bool { return len(mock.RefreshCalls()) == 1 }, time.Second, 5*time.Millisecond)
		cancel()
		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, time.Second, 5*time.Millisecond)

		got, err := s.AccessToken(context.Background(), PortalCUP)
		require.NoError(t, err)
		assert.Equal(t, "access-2", got)
	})

	t.Run("does not refresh fresh tokens", func(t *testing.T) {
		mock := &MockRefresher{}
		s := newTestStore(mock)
		s.Put(PortalCUP, validTokens("access-1"))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		s.Run(ctx, 5*time.Millisecond)

		assert.Empty(t, mock.RefreshCalls())
	})

	t.Run("marks session expired when refresh is rejected", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return Tokens{}, ErrRefreshRejected
			},
		}
		s := newTestStore(mock)
		s.Put(PortalFSE, staleTokens("access-1"))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		s.Run(ctx, 5*time.Millisecond)

		assert.Equal(t, StateExpired, s.Status().Portals[1].State)
	})

	t.Run("transient error keeps tokens", func(t *testing.T) {
		mock := &MockRefresher{
			RefreshFunc: func(ctx context.Context, portal Portal, refreshToken string) (Tokens, error) {
				return Tokens{}, errors.New("boom")
			},
		}
		s := newTestStore(mock)
		s.Put(PortalFSE, staleTokens("access-1"))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		s.Run(ctx, 5*time.Millisecond)

		assert.Equal(t, StateActive, s.Status().Portals[1].State)
	})
}
