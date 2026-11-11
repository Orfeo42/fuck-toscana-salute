package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

func TestNew(t *testing.T) {
	t.Run("serves the session endpoint", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		rec := httptest.NewRecorder()
		New(store, flow).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRegisterGetSession(t *testing.T) {
	t.Run("maps states and omits zero expiries", func(t *testing.T) {
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		store := &MockSessionStore{StatusFunc: func() session.Status {
			return session.Status{
				Portals: []session.PortalStatus{
					{Portal: session.PortalCUP, State: session.StateActive, AccessExpiresAt: at, RefreshExpiresAt: at},
					{Portal: session.PortalFSE, State: session.StateLoggedOut},
				},
				GestioneReady: true,
			}
		}}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State {
			return map[session.Portal]loginflow.State{
				session.PortalCUP: {Phase: loginflow.PhaseWaiting},
			}
		}}
		_, api := humatest.New(t)
		register(api, store, flow)

		resp := api.Get("/api/session")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"gestioneReady": true,
			"portals": [
				{"portal":"cup","state":"active","accessExpiresAt":"2026-01-02T03:04:05Z","refreshExpiresAt":"2026-01-02T03:04:05Z","login":{"phase":"waiting"}},
				{"portal":"fse","state":"logged_out","login":{"phase":"idle"}}
			]
		}`, resp.Body.String())
	})
}

func TestRegisterStartLogin(t *testing.T) {
	t.Run("accepts and starts the flow for the portal", func(t *testing.T) {
		var got session.Portal
		flow := &MockLoginFlow{StartFunc: func(_ context.Context, portal session.Portal) error {
			got = portal
			return nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow)

		resp := api.Post("/api/login/fse")
		assert.Equal(t, http.StatusAccepted, resp.Code)
		assert.Equal(t, session.PortalFSE, got)
		assert.Len(t, flow.StartCalls(), 1)
	})

	t.Run("conflict when a login is in progress", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return loginflow.ErrLoginInProgress
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusConflict, resp.Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow)

		resp := api.Post("/api/login/xyz")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.StartCalls())
	})
}

func TestRegisterLogout(t *testing.T) {
	t.Run("clears the store and returns 204", func(t *testing.T) {
		store := &MockSessionStore{ClearFunc: func() {}}
		_, api := humatest.New(t)
		register(api, store, &MockLoginFlow{})

		resp := api.Post("/api/logout")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, store.ClearCalls(), 1)
	})
}
