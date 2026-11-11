package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
)

func TestNew(t *testing.T) {
	t.Run("sets client ids per portal", func(t *testing.T) {
		c := New(http.DefaultClient, "http://example.invalid/token")

		assert.Equal(t, ClientIDCUP, c.clientIDs[session.PortalCUP])
		assert.Equal(t, ClientIDFSE, c.clientIDs[session.PortalFSE])
		assert.Len(t, c.clientIDs, 2)
		assert.Equal(t, "http://example.invalid/token", c.tokenURL)
	})
}

func TestRefresh(t *testing.T) {
	fixedNow := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	newClient := func(srv *httptest.Server) *Client {
		c := New(srv.Client(), srv.URL)
		c.now = func() time.Time { return fixedNow }
		return c
	}

	t.Run("success", func(t *testing.T) {
		var gotMethod, gotContentType, gotAccept, gotUA string
		var gotForm map[string][]string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotContentType = r.Header.Get("Content-Type")
			gotAccept = r.Header.Get("Accept")
			gotUA = r.Header.Get("User-Agent")
			require.NoError(t, r.ParseForm())
			gotForm = r.PostForm
			_, err := w.Write([]byte(`{"access_token":"fake-access","refresh_token":"fake-refresh","expires_in":300,"refresh_expires_in":1800}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		tokens, err := newClient(srv).Refresh(context.Background(), session.PortalFSE, "old-refresh")

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
		assert.Equal(t, "application/json", gotAccept)
		assert.Equal(t, "fuck-toscana-salute", gotUA)
		assert.Equal(t, []string{"refresh_token"}, gotForm["grant_type"])
		assert.Equal(t, []string{ClientIDFSE}, gotForm["client_id"])
		assert.Equal(t, []string{"old-refresh"}, gotForm["refresh_token"])
		assert.Equal(t, "fake-access", tokens.Access)
		assert.Equal(t, "fake-refresh", tokens.Refresh)
		assert.Equal(t, fixedNow.Add(300*time.Second), tokens.AccessExpiresAt)
		assert.Equal(t, fixedNow.Add(1800*time.Second), tokens.RefreshExpiresAt)
	})

	t.Run("invalid_grant 400 is refresh rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error":"invalid_grant","error_description":"Token is not active"}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, session.ErrRefreshRejected)
		assert.Contains(t, err.Error(), "Token is not active")
	})

	t.Run("invalid_grant 401 is refresh rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, session.ErrRefreshRejected)
	})

	t.Run("400 without invalid_grant is unexpected status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error":"invalid_request"}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, ErrUnexpectedStatus)
	})

	t.Run("500 is unexpected status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`secret-body-fake-token`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.NotContains(t, err.Error(), "secret-body-fake-token")
	})

	t.Run("malformed json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte(`{not json`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, ErrMalformedResponse)
	})

	t.Run("missing access token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte(`{"refresh_token":"fake-refresh","expires_in":300,"refresh_expires_in":1800}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, ErrMalformedResponse)
	})

	t.Run("missing refresh token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte(`{"access_token":"fake-access","expires_in":300,"refresh_expires_in":1800}`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.PortalCUP, "old-refresh")

		require.ErrorIs(t, err, ErrMalformedResponse)
	})

	t.Run("unknown portal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("server must not be called")
		}))
		defer srv.Close()

		_, err := newClient(srv).Refresh(context.Background(), session.Portal("nope"), "old-refresh")

		require.ErrorIs(t, err, ErrUnknownPortal)
	})
}
