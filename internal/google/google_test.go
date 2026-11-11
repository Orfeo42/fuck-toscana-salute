package google

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"fuck-toscana-salute/internal/vault"
)

const testRedirect = "http://127.0.0.1:8787/api/google/callback"

type tokenServer struct {
	srv   *httptest.Server
	mu    sync.Mutex
	forms []url.Values
}

func newTokenServer(t *testing.T, status int, body string) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		ts.mu.Lock()
		ts.forms = append(ts.forms, r.PostForm)
		ts.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, err := w.Write([]byte(body))
		assert.NoError(t, err)
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

func testSettings() Settings {
	return Settings{ClientID: "client-id" + clientIDSuffix, ClientSecret: "client-secret", CalendarID: "primary"}
}

func storedSettings(t *testing.T, s Settings) *MockSettingsStore {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	return &MockSettingsStore{
		GetFunc: func(string, string) (string, error) { return string(data), nil },
		SetFunc: func(string, string, string) error { return nil },
	}
}

func emptySettings() *MockSettingsStore {
	return &MockSettingsStore{
		GetFunc: func(string, string) (string, error) { return "", vault.ErrNotFound },
		SetFunc: func(string, string, string) error { return nil },
	}
}

func newTestClient(settings SettingsStore, tokens TokenStore) *Client {
	c := New(settings, tokens, http.DefaultClient)
	c.endpoint = oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: "http://unused.test"}
	return c
}

func stateFrom(t *testing.T, authURL string) string {
	t.Helper()
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	return parsed.Query().Get("state")
}

func TestStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		got, err := newTestClient(emptySettings(), &MockTokenStore{}).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{}, got)
	})

	t.Run("configured without token", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		got, err := newTestClient(storedSettings(t, testSettings()), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true}, got)
	})

	t.Run("connected when a token is stored", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "at"}, nil
		}}
		got, err := newTestClient(storedSettings(t, testSettings()), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true, Connected: true}, got)
	})

	t.Run("store failure is returned", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, errors.New("keyring down") }}
		_, err := newTestClient(storedSettings(t, testSettings()), store).Status(ctx)
		require.Error(t, err)
	})
}

func TestAuthURL(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		_, err := newTestClient(emptySettings(), &MockTokenStore{}).AuthURL(ctx, testRedirect)
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("carries state, pkce, offline access, consent, scopes and redirect", func(t *testing.T) {
		got, err := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{}).AuthURL(ctx, testRedirect)
		require.NoError(t, err)

		parsed, err := url.Parse(got)
		require.NoError(t, err)
		q := parsed.Query()
		assert.NotEmpty(t, q.Get("state"))
		assert.NotEmpty(t, q.Get("code_challenge"))
		assert.Equal(t, "S256", q.Get("code_challenge_method"))
		assert.Equal(t, "offline", q.Get("access_type"))
		assert.Equal(t, "consent", q.Get("prompt"))
		assert.Equal(t, "https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/drive.file", q.Get("scope"))
		assert.Equal(t, testRedirect, q.Get("redirect_uri"))
	})

	t.Run("uses the given redirect for each call", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		got, err := c.AuthURL(ctx, "https://example.test/other")
		require.NoError(t, err)
		parsed, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, "https://example.test/other", parsed.Query().Get("redirect_uri"))
		assert.Equal(t, testSettings().ClientID, parsed.Query().Get("client_id"))
	})

	t.Run("each call gets a fresh state", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		first, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)
		second, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)
		assert.NotEqual(t, stateFrom(t, first), stateFrom(t, second))
	})

	t.Run("expired pending states are purged", func(t *testing.T) {
		now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		c.now = func() time.Time { return now }
		_, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)

		now = now.Add(stateTTL + time.Second)
		_, err = c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)
		assert.Len(t, c.pending, 1)
	})
}

func TestExchange(t *testing.T) {
	ctx := context.Background()
	const tokenBody = `{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600}`

	t.Run("not configured", func(t *testing.T) {
		err := newTestClient(emptySettings(), &MockTokenStore{}).Exchange(ctx, "s", "c")
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("unknown state", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		require.ErrorIs(t, c.Exchange(ctx, "nope", "code"), ErrBadState)
	})

	t.Run("expired state", func(t *testing.T) {
		now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		c.now = func() time.Time { return now }
		authURL, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)

		now = now.Add(stateTTL)
		require.ErrorIs(t, c.Exchange(ctx, stateFrom(t, authURL), "code"), ErrBadState)
	})

	t.Run("saves the token and consumes the state", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, tokenBody)
		var saved *oauth2.Token
		store := &MockTokenStore{SaveFunc: func(_ context.Context, token *oauth2.Token) error {
			saved = token
			return nil
		}}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)
		state := stateFrom(t, authURL)

		require.NoError(t, c.Exchange(ctx, state, "auth-code"))
		require.NotNil(t, saved)
		assert.Equal(t, "at", saved.AccessToken)
		assert.Equal(t, "rt", saved.RefreshToken)
		require.Len(t, ts.forms, 1)
		assert.Equal(t, "auth-code", ts.forms[0].Get("code"))
		assert.NotEmpty(t, ts.forms[0].Get("code_verifier"))

		require.ErrorIs(t, c.Exchange(ctx, state, "auth-code"), ErrBadState)
		assert.Len(t, store.SaveCalls(), 1)
	})

	t.Run("exchange uses the redirect saved for the state", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, tokenBody)
		store := &MockTokenStore{SaveFunc: func(context.Context, *oauth2.Token) error { return nil }}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(ctx, "https://example.test/other")
		require.NoError(t, err)

		require.NoError(t, c.Exchange(ctx, stateFrom(t, authURL), "auth-code"))
		require.Len(t, ts.forms, 1)
		assert.Equal(t, "https://example.test/other", ts.forms[0].Get("redirect_uri"))
	})

	t.Run("token endpoint rejection is returned and nothing saved", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
		store := &MockTokenStore{}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)

		err = c.Exchange(ctx, stateFrom(t, authURL), "bad")
		var retrieveErr *oauth2.RetrieveError
		require.ErrorAs(t, err, &retrieveErr)
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("save failure is returned", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, tokenBody)
		store := &MockTokenStore{SaveFunc: func(context.Context, *oauth2.Token) error { return errors.New("keyring down") }}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(ctx, testRedirect)
		require.NoError(t, err)

		require.Error(t, c.Exchange(ctx, stateFrom(t, authURL), "code"))
	})
}

func TestDisconnect(t *testing.T) {
	ctx := context.Background()

	t.Run("revokes the refresh token and deletes", func(t *testing.T) {
		rev := newTokenServer(t, http.StatusOK, `{}`)
		store := &MockTokenStore{
			LoadFunc: func(context.Context) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at", RefreshToken: "rt"}, nil
			},
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.revokeURL = rev.srv.URL

		require.NoError(t, c.Disconnect(ctx))
		require.Len(t, rev.forms, 1)
		assert.Equal(t, "rt", rev.forms[0].Get("token"))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("revoke failure still deletes", func(t *testing.T) {
		rev := newTokenServer(t, http.StatusBadRequest, `{}`)
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return &oauth2.Token{RefreshToken: "rt"}, nil },
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.revokeURL = rev.srv.URL

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("unreachable revoke endpoint still deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return &oauth2.Token{RefreshToken: "rt"}, nil },
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.revokeURL = "http://127.0.0.1:1"

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("no stored token skips revoke and deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, nil },
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(emptySettings(), store)

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("load failure still deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, errors.New("keyring down") },
			DeleteFunc: func(context.Context) error { return nil },
		}
		require.NoError(t, newTestClient(emptySettings(), store).Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("delete failure is returned", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, nil },
			DeleteFunc: func(context.Context) error { return errors.New("keyring down") },
		}
		require.Error(t, newTestClient(emptySettings(), store).Disconnect(ctx))
	})
}
