package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func TestNew(t *testing.T) {
	t.Run("serves the session endpoint", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		rec := httptest.NewRecorder()
		New(store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
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
		register(api, store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusConflict, resp.Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		resp := api.Post("/api/login/xyz")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.StartCalls())
	})
}

func TestRegisterStartLoginAll(t *testing.T) {
	t.Run("accepts and starts the combined flow", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		resp := api.Post("/api/login")
		assert.Equal(t, http.StatusAccepted, resp.Code)
		assert.Len(t, flow.StartAllCalls(), 1)
	})

	t.Run("conflict when a login is in progress", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return loginflow.ErrLoginInProgress }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		assert.Equal(t, http.StatusConflict, api.Post("/api/login").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		assert.Equal(t, http.StatusInternalServerError, api.Post("/api/login").Code)
	})
}

func TestRegisterLogout(t *testing.T) {
	t.Run("clears the store and returns 204", func(t *testing.T) {
		store := &MockSessionStore{ClearFunc: func() {}}
		_, api := humatest.New(t)
		register(api, store, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{})

		resp := api.Post("/api/logout")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, store.ClearCalls(), 1)
	})
}

func TestRegisterListAppointments(t *testing.T) {
	t.Run("maps fields, zero time absent and nil items empty", func(t *testing.T) {
		at := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{Entries: []agenda.Entry{
				{
					NRE: "n1",
					CUP: &cup.Prescription{NRE: "n1", Kind: "visit", Status: 2, Items: []string{"x"}},
					Appointment: cup.Appointment{
						ID: "a1", NRE: "n1", Status: "booked", At: at,
						Unit: cup.Unit{Name: "Unit", Description: "Desc", Address: "Via Test 1", City: "Firenze", Province: "FI"},
					},
				},
				{
					NRE:         "n2",
					FSE:         &fse.Prescription{ID: "f2", NRE: "n2", Description: "D", Kind: fse.KindVisit, Status: 30, DocumentRef: "ref"},
					Appointment: cup.Appointment{ID: "a2"},
				},
			}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"appointments": [
				{"id":"a1","nre":"n1","status":"booked","at":"2026-03-04T09:30:00Z",
				 "unit":{"name":"Unit","description":"Desc","address":"Via Test 1","city":"Firenze","province":"FI"},
				 "cup":{"nre":"n1","kind":"visit","status":2,"items":["x"]}},
				{"id":"a2","nre":"n2","status":"",
				 "unit":{"name":"","description":"","address":"","city":"","province":""},
				 "fse":{"id":"f2","nre":"n2","description":"D","kind":"visit","status":30,"documentRef":"ref"}}
			],
			"failed": [],
			"unbooked": []
		}`, resp.Body.String())
	})

	t.Run("failed lists each reason kind", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{Failed: []agenda.Failure{
				{NRE: "n1", Err: fmt.Errorf("wrap: %w", session.ErrSessionExpired)},
				{NRE: "n2", Err: session.ErrNotLoggedIn},
				{NRE: "n3", Err: cup.ErrGestioneNotReady},
				{NRE: "n4", Err: fmt.Errorf("wrap: %w", &upstream.StatusError{Status: 500})},
				{NRE: "n5", Err: errors.New("boom")},
				{NRE: "n6", Err: &upstream.StatusError{Status: 502}},
			}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"appointments": [],
			"failed": [
				{"nre":"n1","reason":"session_expired"},
				{"nre":"n2","reason":"session_expired"},
				{"nre":"n3","reason":"gestione_not_ready"},
				{"nre":"n4","reason":"upstream_500"},
				{"nre":"n5","reason":"error"},
				{"nre":"n6","reason":"upstream_502"}
			],
			"unbooked": []
		}`, resp.Body.String())
	})

	t.Run("failures emit one warn without NRE", func(t *testing.T) {
		var buf bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(previous)

		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{Failed: []agenda.Failure{
				{NRE: "FAKENRE123", Err: &upstream.StatusError{Status: 500}},
			}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		require.Equal(t, http.StatusOK, api.Get("/api/appointments").Code)
		logged := buf.String()
		assert.Equal(t, 1, strings.Count(logged, "appointment lookups failed"))
		assert.Contains(t, logged, "first_exception")
		assert.Contains(t, logged, "first_error")
		assert.Contains(t, logged, "first_shape=empty")
		assert.NotContains(t, logged, "FAKENRE123")
	})

	t.Run("unauthorized when not logged in", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, session.ErrNotLoggedIn
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		assert.Equal(t, http.StatusUnauthorized, api.Get("/api/appointments").Code)
	})

	t.Run("conflict when gestione is not ready", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		assert.Equal(t, http.StatusConflict, api.Get("/api/appointments").Code)
	})

	t.Run("bad gateway on upstream status error", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, &upstream.StatusError{Status: 503}
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		assert.Equal(t, http.StatusBadGateway, api.Get("/api/appointments").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"prescriptions":[]}`, resp.Body.String())
	})

	t.Run("unauthorized when session expired", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) {
			return nil, session.ErrSessionExpired
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{})

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		resp := api.Get("/api/documents/prescription")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PrescriptionPDFCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PrescriptionPDFFunc: func(context.Context, string) ([]byte, error) {
			return nil, fse.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		assert.Equal(t, http.StatusNotFound, api.Get("/api/documents/prescription?ref=x").Code)
	})
}

func TestRegisterGetBookingPDF(t *testing.T) {
	t.Run("returns the pdf inline", func(t *testing.T) {
		var gotID string
		docs := &MockDocuments{PromemoriaFunc: func(_ context.Context, id string) ([]byte, error) {
			gotID = id
			return []byte("%PDF-1.4 fake"), nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		resp := api.Get("/api/documents/booking?id=ABC123456789")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/pdf", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="promemoria.pdf"`, resp.Header().Get("Content-Disposition"))
		assert.Equal(t, "%PDF-1.4 fake", resp.Body.String())
		assert.Equal(t, "ABC123456789", gotID)
	})

	t.Run("invalid id is rejected by validation", func(t *testing.T) {
		docs := &MockDocuments{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		resp := api.Get("/api/documents/booking?id=a-b")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("missing id is rejected", func(t *testing.T) {
		docs := &MockDocuments{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		assert.Equal(t, http.StatusUnprocessableEntity, api.Get("/api/documents/booking").Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		assert.Equal(t, http.StatusNotFound, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})

	t.Run("gestione not ready is a conflict", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{})

		assert.Equal(t, http.StatusConflict, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})
}

func TestRegisterGoogleStatus(t *testing.T) {
	t.Run("returns configured and connected", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) {
			return google.Status{Configured: true, Connected: true}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Get("/api/google")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"configured":true,"connected":true}`, resp.Body.String())
	})

	t.Run("store failure is 500", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) { return google.Status{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		assert.Equal(t, http.StatusInternalServerError, api.Get("/api/google").Code)
	})
}

func TestRegisterGoogleConnect(t *testing.T) {
	t.Run("returns the authorization url", func(t *testing.T) {
		g := &MockGoogle{AuthURLFunc: func() (string, error) { return "https://accounts.example.test/auth?state=s", nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/google/connect")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"authUrl":"https://accounts.example.test/auth?state=s"}`, resp.Body.String())
	})

	t.Run("not configured is 409", func(t *testing.T) {
		g := &MockGoogle{AuthURLFunc: func() (string, error) { return "", google.ErrNotConfigured }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/google/connect")
		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_configured")
	})
}

func TestRegisterGoogleDisconnect(t *testing.T) {
	t.Run("disconnects with 204", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Delete("/api/google")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, g.DisconnectCalls(), 1)
	})

	t.Run("failure is 500", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		assert.Equal(t, http.StatusInternalServerError, api.Delete("/api/google").Code)
	})
}

func TestRegisterCreateCalendarEvent(t *testing.T) {
	const fakeBooking = "ABC123456789"
	okEvent := func(context.Context, google.Event, []byte) (google.EventResult, error) {
		return google.EventResult{HTMLLink: "https://calendar.example.test/e", Created: true}, nil
	}

	t.Run("attaches the promemoria when booking id is given", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) { return []byte("%PDF-fake"), nil }}
		g := &MockGoogle{AddEventFunc: okEvent}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1",
			"title":         "Visita",
			"start":         "2026-03-10T09:30:00+01:00",
			"location":      "Ospedale Test",
			"details":       "Piano 2",
			"bookingId":     fakeBooking,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"htmlLink":"https://calendar.example.test/e","created":true}`, resp.Body.String())

		require.Len(t, docs.PromemoriaCalls(), 1)
		assert.Equal(t, fakeBooking, docs.PromemoriaCalls()[0].BookingID)
		require.Len(t, g.AddEventCalls(), 1)
		call := g.AddEventCalls()[0]
		assert.Equal(t, []byte("%PDF-fake"), call.Pdf)
		assert.Equal(t, "APP1", call.Ev.Key)
		assert.Equal(t, "Visita", call.Ev.Title)
		assert.Equal(t, "Ospedale Test", call.Ev.Location)
		assert.Equal(t, "Piano 2", call.Ev.Details)
		assert.Equal(t, 60*time.Minute, call.Ev.Duration)
		assert.True(t, time.Date(2026, 3, 10, 8, 30, 0, 0, time.UTC).Equal(call.Ev.Start))
	})

	t.Run("duration is taken from the request", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "durationMinutes": 90, "attachPromemoria": false,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, 90*time.Minute, g.AddEventCalls()[0].Ev.Duration)
	})

	t.Run("attachPromemoria false never fetches the pdf", func(t *testing.T) {
		docs := &MockDocuments{}
		g := &MockGoogle{AddEventFunc: okEvent}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": fakeBooking, "attachPromemoria": false,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
		assert.Nil(t, g.AddEventCalls()[0].Pdf)
	})

	t.Run("no booking id never fetches the pdf", func(t *testing.T) {
		docs := &MockDocuments{}
		g := &MockGoogle{AddEventFunc: okEvent}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("promemoria failure skips the event", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) { return nil, session.ErrNotLoggedIn }}
		g := &MockGoogle{AddEventFunc: okEvent}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": fakeBooking,
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("google not configured is 409", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: func(context.Context, google.Event, []byte) (google.EventResult, error) {
			return google.EventResult{}, google.ErrNotConfigured
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "attachPromemoria": false,
		})
		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_configured")
	})

	t.Run("google not connected is 401", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: func(context.Context, google.Event, []byte) (google.EventResult, error) {
			return google.EventResult{}, google.ErrNotConnected
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "attachPromemoria": false,
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_connected")
	})

	t.Run("bad appointment id is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "bad id!", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("out of range duration is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "durationMinutes": 4,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("bad booking id is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": "../x",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("missing title is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g)

		resp := api.Post("/api/calendar/events", map[string]any{"appointmentId": "APP1", "start": "2026-03-10T09:30:00+01:00"})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}
