package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func newTestStore(refresher Refresher) *Store {
	s := NewStore(refresher, time.Minute)
	s.now = func() time.Time { return testNow }
	return s
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
		s.Put(PortalCUP, validTokens("access-1"))

		assert.False(t, s.Status().GestioneReady)
	})

	t.Run("gestione not ready without CUP", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalFSE, validTokens("access-1"))
		s.SetIdentity(Identity{TesseraSanitaria: "80380000000000000000"})

		assert.False(t, s.Status().GestioneReady)
	})

	t.Run("gestione ready with CUP and tessera", func(t *testing.T) {
		s := newTestStore(&MockRefresher{})
		s.Put(PortalCUP, validTokens("access-1"))
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
}

func TestRun(t *testing.T) {
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
