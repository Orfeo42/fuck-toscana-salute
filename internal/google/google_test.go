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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"fuck-toscana-salute/internal/vault"
)

const testClientID = "123456789012-abc.apps.googleusercontent.com"

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
	return Settings{ClientID: testClientID, ClientSecret: "client-secret", CalendarID: "primary"}
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
		assert.Equal(t, Status{Configured: true, ClientID: testClientID}, got)
	})

	t.Run("connected when a token is stored", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "at"}, nil
		}}
		got, err := newTestClient(storedSettings(t, testSettings()), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true, Connected: true, ClientID: testClientID}, got)
	})

	t.Run("drive is on when an api key is set", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		got, err := newTestClient(storedSettings(t, pickerSettings()), store).Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, Status{Configured: true, ClientID: testClientID, Drive: true}, got)
	})

	t.Run("store failure is returned", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, errors.New("keyring down") }}
		_, err := newTestClient(storedSettings(t, testSettings()), store).Status(ctx)
		require.Error(t, err)
	})
}

func TestExchangeCode(t *testing.T) {
	ctx := context.Background()
	const origin = "http://127.0.0.1:8800"
	const scopes = "https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/drive.file"
	const goodBody = `{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600,"scope":"` + scopes + `"}`

	t.Run("not configured", func(t *testing.T) {
		err := newTestClient(emptySettings(), &MockTokenStore{}).ExchangeCode(ctx, "c", origin)
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("saves the token and sends code and origin without verifier", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, goodBody)
		var saved *oauth2.Token
		store := &MockTokenStore{SaveFunc: func(_ context.Context, token *oauth2.Token) error {
			saved = token
			return nil
		}}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL

		require.NoError(t, c.ExchangeCode(ctx, "auth-code", origin))
		require.NotNil(t, saved)
		assert.Equal(t, "at", saved.AccessToken)
		assert.Equal(t, "rt", saved.RefreshToken)
		require.Len(t, ts.forms, 1)
		assert.Equal(t, "auth-code", ts.forms[0].Get("code"))
		assert.Equal(t, origin, ts.forms[0].Get("redirect_uri"))
		assert.Empty(t, ts.forms[0].Get("code_verifier"))
	})

	t.Run("missing refresh token revokes the grant and saves nothing", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"at","token_type":"Bearer","expires_in":3600,"scope":"`+scopes+`"}`)
		rev := newTokenServer(t, http.StatusOK, `{}`)
		store := &MockTokenStore{}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		c.revokeURL = rev.srv.URL

		require.ErrorIs(t, c.ExchangeCode(ctx, "code", origin), ErrNoRefreshToken)
		require.Len(t, rev.forms, 1)
		assert.Equal(t, "at", rev.forms[0].Get("token"))
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("missing scope revokes the grant and saves nothing", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600,"scope":"https://www.googleapis.com/auth/calendar.events"}`)
		rev := newTokenServer(t, http.StatusOK, `{}`)
		store := &MockTokenStore{}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		c.revokeURL = rev.srv.URL

		require.ErrorIs(t, c.ExchangeCode(ctx, "code", origin), ErrScopesMissing)
		require.Len(t, rev.forms, 1)
		assert.Equal(t, "rt", rev.forms[0].Get("token"))
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("token endpoint rejection is wrapped and nothing saved", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
		store := &MockTokenStore{}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL

		err := c.ExchangeCode(ctx, "bad", origin)
		var retrieveErr *oauth2.RetrieveError
		require.ErrorAs(t, err, &retrieveErr)
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("save failure is returned", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, goodBody)
		store := &MockTokenStore{SaveFunc: func(context.Context, *oauth2.Token) error { return errors.New("keyring down") }}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL

		require.Error(t, c.ExchangeCode(ctx, "code", origin))
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
