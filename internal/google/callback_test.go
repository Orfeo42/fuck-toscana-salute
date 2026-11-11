package google

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestCallbackHandler(t *testing.T) {
	const tokenBody = `{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600}`

	t.Run("success serves the static page with security headers", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, tokenBody)
		store := &MockTokenStore{SaveFunc: func(context.Context, *oauth2.Token) error { return nil }}
		c := newTestClient(storedSettings(t, testSettings()), store)
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(context.Background(), testRedirect)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		target := CallbackPath + "?code=abc&state=" + url.QueryEscape(stateFrom(t, authURL))
		c.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Google collegato. Puoi chiudere questa scheda.")
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
		assert.Equal(t, "default-src 'none'", rec.Header().Get("Content-Security-Policy"))
		assert.Len(t, store.SaveCalls(), 1)
	})

	t.Run("bad state is 400 and reflects nothing", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})

		rec := httptest.NewRecorder()
		c.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, CallbackPath+"?code=SECRETCODE&state=REFLECTME", nil))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Collegamento non riuscito. Riprova dall'app.")
		assert.NotContains(t, rec.Body.String(), "SECRETCODE")
		assert.NotContains(t, rec.Body.String(), "REFLECTME")
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	})

	t.Run("error parameter is 400 and not echoed", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})

		rec := httptest.NewRecorder()
		c.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, CallbackPath+"?error=access_denied%3Cscript%3E", nil))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.NotContains(t, rec.Body.String(), "access_denied")
	})

	t.Run("token endpoint rejection is 400", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})
		c.endpoint.TokenURL = ts.srv.URL
		authURL, err := c.AuthURL(context.Background(), testRedirect)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		c.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, CallbackPath+"?code=x&state="+url.QueryEscape(stateFrom(t, authURL)), nil))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("non-GET is 405", func(t *testing.T) {
		c := newTestClient(storedSettings(t, testSettings()), &MockTokenStore{})

		rec := httptest.NewRecorder()
		c.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, CallbackPath, nil))

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
