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

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func TestNew(t *testing.T) {
	t.Run("serves the session endpoint", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		rec := httptest.NewRecorder()
		New(store, flow, &MockAgenda{}, &MockDocuments{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
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
		register(api, store, flow, &MockAgenda{}, &MockDocuments{})

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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{})

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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{})

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusConflict, resp.Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{})

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{})

		resp := api.Post("/api/login/xyz")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.StartCalls())
	})
}

func TestRegisterLogout(t *testing.T) {
	t.Run("clears the store and returns 204", func(t *testing.T) {
		store := &MockSessionStore{ClearFunc: func() {}}
		_, api := humatest.New(t)
		register(api, store, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{})

		resp := api.Post("/api/logout")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, store.ClearCalls(), 1)
	})
}

func TestRegisterListAppointments(t *testing.T) {
	t.Run("maps fields, zero time absent and nil items empty", func(t *testing.T) {
		at := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) ([]agenda.Entry, error) {
			return []agenda.Entry{
				{
					Appointment: cup.Appointment{
						ID: "a1", NRE: "n1", Status: "booked", At: at,
						Unit: cup.Unit{Name: "Unit", Description: "Desc", Address: "Via Test 1", City: "Firenze", Province: "FI"},
					},
					Prescription: cup.Prescription{NRE: "n1", Kind: "visit", Status: 2, Items: []string{"x"}},
				},
				{
					Appointment:  cup.Appointment{ID: "a2", NRE: "n2"},
					Prescription: cup.Prescription{NRE: "n2"},
				},
			}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"appointments": [
				{"id":"a1","nre":"n1","status":"booked","at":"2026-03-04T09:30:00Z",
				 "unit":{"name":"Unit","description":"Desc","address":"Via Test 1","city":"Firenze","province":"FI"},
				 "prescription":{"nre":"n1","kind":"visit","status":2,"items":["x"]}},
				{"id":"a2","nre":"n2","status":"",
				 "unit":{"name":"","description":"","address":"","city":"","province":""},
				 "prescription":{"nre":"n2","kind":"","status":0,"items":[]}}
			]
		}`, resp.Body.String())
	})

	t.Run("unauthorized when not logged in", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) ([]agenda.Entry, error) {
			return nil, session.ErrNotLoggedIn
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		assert.Equal(t, http.StatusUnauthorized, api.Get("/api/appointments").Code)
	})

	t.Run("conflict when gestione is not ready", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) ([]agenda.Entry, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		assert.Equal(t, http.StatusConflict, api.Get("/api/appointments").Code)
	})

	t.Run("bad gateway on upstream status error", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) ([]agenda.Entry, error) {
			return nil, &upstream.StatusError{Status: 503}
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		assert.Equal(t, http.StatusBadGateway, api.Get("/api/appointments").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) ([]agenda.Entry, error) {
			return nil, errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		assert.Equal(t, http.StatusInternalServerError, api.Get("/api/appointments").Code)
	})
}

func TestRegisterListPrescriptions(t *testing.T) {
	t.Run("maps cup-only, fse-only and both rows", func(t *testing.T) {
		date := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) {
			return []agenda.Row{
				{NRE: "n1", CUP: &cup.Prescription{NRE: "n1", PrescribedOn: date, Kind: "visit", Status: 1, Items: []string{"a", "b"}}},
				{FSE: &fse.Prescription{ID: "f1", NRE: "", Description: "Desc", Kind: "pharma", Status: 3, Date: date, DocumentRef: "ref-1"}},
				{
					NRE: "n3",
					CUP: &cup.Prescription{NRE: "n3"},
					FSE: &fse.Prescription{ID: "f3", NRE: "n3"},
				},
			}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"prescriptions": [
				{"nre":"n1","cup":{"nre":"n1","prescribedOn":"2026-02-03T00:00:00Z","kind":"visit","status":1,"items":["a","b"]}},
				{"nre":"","fse":{"id":"f1","nre":"","description":"Desc","kind":"pharma","status":3,"date":"2026-02-03T00:00:00Z","documentRef":"ref-1"}},
				{"nre":"n3","cup":{"nre":"n3","kind":"","status":0,"items":[]},"fse":{"id":"f3","nre":"n3","description":"","kind":"","status":0,"documentRef":""}}
			]
		}`, resp.Body.String())
	})

	t.Run("empty list is an empty array", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) { return nil, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"prescriptions":[]}`, resp.Body.String())
	})

	t.Run("unauthorized when session expired", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) {
			return nil, session.ErrSessionExpired
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{})

		assert.Equal(t, http.StatusUnauthorized, api.Get("/api/prescriptions").Code)
	})
}

func TestRegisterGetPrescriptionPDF(t *testing.T) {
	t.Run("returns the pdf bytes with headers", func(t *testing.T) {
		var gotRef string
		docs := &MockDocuments{PrescriptionPDFFunc: func(_ context.Context, ref string) ([]byte, error) {
			gotRef = ref
			return []byte("%PDF-1.4 fake"), nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs)

		resp := api.Get("/api/documents/prescription?ref=ref-123")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/pdf", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="ricetta.pdf"`, resp.Header().Get("Content-Disposition"))
		assert.Equal(t, "%PDF-1.4 fake", resp.Body.String())
		assert.Equal(t, "ref-123", gotRef)
	})

	t.Run("missing ref is rejected", func(t *testing.T) {
		docs := &MockDocuments{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs)

		resp := api.Get("/api/documents/prescription")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PrescriptionPDFCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PrescriptionPDFFunc: func(context.Context, string) ([]byte, error) {
			return nil, fse.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs)

		assert.Equal(t, http.StatusNotFound, api.Get("/api/documents/prescription?ref=x").Code)
	})
}
