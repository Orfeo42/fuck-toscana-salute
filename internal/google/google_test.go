package google

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
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

func testConfig(tokenURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  testRedirect,
		Scopes:       []string{"https://www.googleapis.com/auth/calendar.events", "https://www.googleapis.com/auth/drive.file"},
		Endpoint:     oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: tokenURL},
	}
}

func newTestClient(cfg *oauth2.Config, store TokenStore) *Client {
	return New(cfg, store, http.DefaultClient)
}

func stateFrom(t *testing.T, authURL string) string {
	t.Helper()
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	return parsed.Query().Get("state")
}

func TestConfigFromFile(t *testing.T) {
	t.Run("parses a desktop client file and sets the redirect", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "client.json")
		data := `{"installed":{"client_id":"id","client_secret":"secret","auth_uri":"https://accounts.example.test/auth","token_uri":"https://accounts.example.test/token","redirect_uris":["http://localhost"]}}`
		require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

		cfg, err := ConfigFromFile(path, testRedirect)
		require.NoError(t, err)
		assert.Equal(t, "id", cfg.ClientID)
		assert.Equal(t, testRedirect, cfg.RedirectURL)
		assert.Len(t, cfg.Scopes, 2)
	})

	t.Run("missing file reports not exist", func(t *testing.T) {
		_, err := ConfigFromFile(filepath.Join(t.TempDir(), "absent.json"), testRedirect)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("invalid file is rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "client.json")
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))

		_, err := ConfigFromFile(path, testRedirect)
		require.Error(t, err)
	})
}

func TestStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		got, err := newTestClient(nil, &MockTokenStore{}).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{}, got)
	})

	t.Run("configured without token", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		got, err := newTestClient(testConfig("http://unused.test"), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true}, got)
	})

	t.Run("connected when a token is stored", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "at"}, nil
		}}
		got, err := newTestClient(testConfig("http://unused.test"), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true, Connected: true}, got)
	})

	t.Run("store failure is returned", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, errors.New("keyring down") }}
		_, err := newTestClient(testConfig("http://unused.test"), store).Status(ctx)
		require.Error(t, err)
	})
}

func TestAuthURL(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		_, err := newTestClient(nil, &MockTokenStore{}).AuthURL()
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("carries state, pkce, offline access, consent, scopes and redirect", func(t *testing.T) {
		got, err := newTestClient(testConfig("http://unused.test"), &MockTokenStore{}).AuthURL()
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

	t.Run("each call gets a fresh state", func(t *testing.T) {
		c := newTestClient(testConfig("http://unused.test"), &MockTokenStore{})
		first, err := c.AuthURL()
		require.NoError(t, err)
		second, err := c.AuthURL()
		require.NoError(t, err)
		assert.NotEqual(t, stateFrom(t, first), stateFrom(t, second))
	})

	t.Run("expired pending states are purged", func(t *testing.T) {
		now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		c := newTestClient(testConfig("http://unused.test"), &MockTokenStore{})
		c.now = func() time.Time { return now }
		_, err := c.AuthURL()
		require.NoError(t, err)

		now = now.Add(stateTTL + time.Second)
		_, err = c.AuthURL()
		require.NoError(t, err)
		assert.Len(t, c.pending, 1)
	})
}

func TestExchange(t *testing.T) {
	ctx := context.Background()
	const tokenBody = `{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600}`

	t.Run("not configured", func(t *testing.T) {
		err := newTestClient(nil, &MockTokenStore{}).Exchange(ctx, "s", "c")
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("unknown state", func(t *testing.T) {
		c := newTestClient(testConfig("http://unused.test"), &MockTokenStore{})
		require.ErrorIs(t, c.Exchange(ctx, "nope", "code"), ErrBadState)
	})

	t.Run("expired state", func(t *testing.T) {
		now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		c := newTestClient(testConfig("http://unused.test"), &MockTokenStore{})
		c.now = func() time.Time { return now }
		authURL, err := c.AuthURL()
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
		c := newTestClient(testConfig(ts.srv.URL), store)
		authURL, err := c.AuthURL()
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

	t.Run("token endpoint rejection is returned and nothing saved", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
		store := &MockTokenStore{}
		c := newTestClient(testConfig(ts.srv.URL), store)
		authURL, err := c.AuthURL()
		require.NoError(t, err)

		err = c.Exchange(ctx, stateFrom(t, authURL), "bad")
		var retrieveErr *oauth2.RetrieveError
		require.ErrorAs(t, err, &retrieveErr)
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("save failure is returned", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, tokenBody)
		store := &MockTokenStore{SaveFunc: func(context.Context, *oauth2.Token) error { return errors.New("keyring down") }}
		c := newTestClient(testConfig(ts.srv.URL), store)
		authURL, err := c.AuthURL()
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
		c := newTestClient(testConfig("http://unused.test"), store)
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
		c := newTestClient(testConfig("http://unused.test"), store)
		c.revokeURL = rev.srv.URL

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("unreachable revoke endpoint still deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return &oauth2.Token{RefreshToken: "rt"}, nil },
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(testConfig("http://unused.test"), store)
		c.revokeURL = "http://127.0.0.1:1"

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("no stored token skips revoke and deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, nil },
			DeleteFunc: func(context.Context) error { return nil },
		}
		c := newTestClient(nil, store)

		require.NoError(t, c.Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("load failure still deletes", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, errors.New("keyring down") },
			DeleteFunc: func(context.Context) error { return nil },
		}
		require.NoError(t, newTestClient(nil, store).Disconnect(ctx))
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("delete failure is returned", func(t *testing.T) {
		store := &MockTokenStore{
			LoadFunc:   func(context.Context) (*oauth2.Token, error) { return nil, nil },
			DeleteFunc: func(context.Context) error { return errors.New("keyring down") },
		}
		require.Error(t, newTestClient(nil, store).Disconnect(ctx))
	})
}
