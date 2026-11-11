package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func emptyFreshness() *MockFreshness {
	return &MockFreshness{FetchedAtFunc: func(context.Context, string) (map[string]time.Time, error) { return nil, nil }}
}

func TestNew(t *testing.T) {
	t.Run("serves the session endpoint", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		rec := httptest.NewRecorder()
		New(store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
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
		register(api, store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/session")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"gestioneReady": true,
			"loginMode": "window",
			"viewer": false,
			"portals": [
				{"portal":"cup","state":"active","accessExpiresAt":"2026-01-02T03:04:05Z","refreshExpiresAt":"2026-01-02T03:04:05Z","login":{"phase":"waiting"}},
				{"portal":"fse","state":"logged_out","login":{"phase":"idle"}}
			]
		}`, resp.Body.String())
	})
}

func TestRegisterGetSessionViewer(t *testing.T) {
	t.Run("reports the viewer as available", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		_, api := humatest.New(t)
		register(api, store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", true)

		resp := api.Get("/api/session")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"viewer":true`)
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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusConflict, resp.Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/login/xyz")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.StartCalls())
	})
}

func TestRegisterStartLoginAll(t *testing.T) {
	t.Run("accepts and starts the combined flow", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/login")
		assert.Equal(t, http.StatusAccepted, resp.Code)
		assert.Len(t, flow.StartAllCalls(), 1)
	})

	t.Run("conflict when a login is in progress", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return loginflow.ErrLoginInProgress }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusConflict, api.Post("/api/login").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusInternalServerError, api.Post("/api/login").Code)
	})
}

func importToken(t *testing.T, azp string, exp time.Time) string {
	t.Helper()
	enc := base64.RawURLEncoding
	payload := fmt.Sprintf(`{"iss":"https://auth.regione.toscana.it/auth/realms/rt","azp":%q,"exp":%d}`, azp, exp.Unix())
	return enc.EncodeToString([]byte(`{"alg":"none"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".sig"
}

func TestRegisterImportLogin(t *testing.T) {
	const (
		origin = "Origin: http://example.com"
		host   = "Host: example.com"
	)
	validCUP := func(t *testing.T) string {
		return importToken(t, "r_toscan__portale_cup2_0", time.Now().Add(time.Hour))
	}

	t.Run("passes the parsed result to the flow and returns 204", func(t *testing.T) {
		var got login.Result
		flow := &MockLoginFlow{ImportFunc: func(_ context.Context, res login.Result) error {
			got = res
			return nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "rssmra80a01h501u", "tessera": "80380000000000000000"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Equal(t, session.PortalCUP, got.Portal)
		assert.Equal(t, tok, got.Tokens.Access)
		assert.Equal(t, "RSSMRA80A01H501U", got.Identity.CodiceFiscale)
		assert.Equal(t, "80380000000000000000", got.Identity.TesseraSanitaria)
		assert.True(t, got.Tokens.GestioneVerified)
	})

	t.Run("cup without tessera is not gestione verified", func(t *testing.T) {
		var got login.Result
		flow := &MockLoginFlow{ImportFunc: func(_ context.Context, res login.Result) error {
			got = res
			return nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Empty(t, got.Identity.TesseraSanitaria)
		assert.False(t, got.Tokens.GestioneVerified)
	})

	t.Run("fse ignores a tessera", func(t *testing.T) {
		var got login.Result
		flow := &MockLoginFlow{ImportFunc: func(_ context.Context, res login.Result) error {
			got = res
			return nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := importToken(t, "r_toscan__fse_web", time.Now().Add(time.Hour))

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "fse", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U", "tessera": "80380000000000000000"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Empty(t, got.Identity.TesseraSanitaria)
		assert.False(t, got.Tokens.GestioneVerified)
	})

	t.Run("missing origin is forbidden", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, "Origin: https://evil.example", map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("mismatching token is unprocessable", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return login.ErrTokenMismatch }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), "non appartengono")
	})

	t.Run("expired token is unprocessable", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return login.ErrTokenExpired }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), "scaduto")
	})

	t.Run("malformed token is unprocessable and never reaches the flow", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": "not-a-jwt", "refresh": "not-a-jwt", "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected by the schema", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "xyz", "access": "a", "refresh": "b", "cf": "c"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("empty access is rejected by the schema", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": "", "refresh": "b", "cf": "c"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})
}

func TestRegisterGetSessionLoginMode(t *testing.T) {
	t.Run("exposes the configured login mode", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		_, api := humatest.New(t)
		register(api, store, flow, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "bookmarklet", false)

		resp := api.Get("/api/session")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"loginMode":"bookmarklet"`)
	})
}

func TestRegisterLogout(t *testing.T) {
	t.Run("clears the store and returns 204", func(t *testing.T) {
		store := &MockSessionStore{ClearFunc: func() {}}
		_, api := humatest.New(t)
		register(api, store, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
						Unit: cup.Unit{Name: "Unit", Description: "Desc", Place: "Ospedale Esempio", Address: "Via Test 1", PostalCode: "50100", City: "Firenze", Province: "FI"},
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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"appointments": [
				{"id":"a1","nre":"n1","status":"booked","at":"2026-03-04T09:30:00Z",
				 "unit":{"name":"Unit","description":"Desc","place":"Ospedale Esempio","address":"Via Test 1","postalCode":"50100","city":"Firenze","province":"FI"},
				 "cup":{"nre":"n1","kind":"visit","status":2,"items":["x"]}},
				{"id":"a2","nre":"n2","status":"",
				 "unit":{"name":"","description":"","address":"","city":"","province":""},
				 "fse":{"id":"f2","nre":"n2","description":"D","kind":"visit","status":30,"documentRef":"ref"}}
			],
			"failed": [],
			"unbooked": [],
			"verdicts": [],
			"updated": {"lists": null, "appointments": {}, "verdicts": {}}
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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
			"unbooked": [],
			"verdicts": [],
			"updated": {"lists": null, "appointments": {}, "verdicts": {}}
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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusUnauthorized, api.Get("/api/appointments").Code)
	})

	t.Run("conflict when gestione is not ready", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusConflict, api.Get("/api/appointments").Code)
	})

	t.Run("bad gateway on upstream status error", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, &upstream.StatusError{Status: 503}
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusBadGateway, api.Get("/api/appointments").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{}, errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"prescriptions": [
				{"nre":"n1","cup":{"nre":"n1","prescribedOn":"2026-02-03T00:00:00Z","kind":"visit","status":1,"items":["a","b"]}},
				{"nre":"","fse":{"id":"f1","nre":"","description":"Desc","kind":"pharma","status":3,"date":"2026-02-03T00:00:00Z","documentRef":"ref-1"}},
				{"nre":"n3","cup":{"nre":"n3","kind":"","status":0,"items":[]},"fse":{"id":"f3","nre":"n3","description":"","kind":"","status":0,"documentRef":""}}
			],
			"updated": null
		}`, resp.Body.String())
	})

	t.Run("empty list is an empty array", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) { return nil, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"prescriptions":[],"updated":null}`, resp.Body.String())
	})

	t.Run("unauthorized when session expired", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) {
			return nil, session.ErrSessionExpired
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/documents/prescription")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PrescriptionPDFCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PrescriptionPDFFunc: func(context.Context, string) ([]byte, error) {
			return nil, fse.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/documents/booking?id=a-b")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("missing id is rejected", func(t *testing.T) {
		docs := &MockDocuments{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusUnprocessableEntity, api.Get("/api/documents/booking").Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusNotFound, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})

	t.Run("gestione not ready is a conflict", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusConflict, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})
}

func TestRegisterGoogleStatus(t *testing.T) {
	t.Run("returns configured and connected", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) {
			return google.Status{Configured: true, Connected: true}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/google")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"configured":true,"connected":true}`, resp.Body.String())
	})

	t.Run("store failure is 500", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) { return google.Status{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusInternalServerError, api.Get("/api/google").Code)
	})
}

func TestRegisterGoogleConnect(t *testing.T) {
	t.Run("returns the authorization url", func(t *testing.T) {
		g := &MockGoogle{AuthURLFunc: func(context.Context, string) (string, error) {
			return "https://accounts.example.test/auth?state=s", nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/google/connect")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"authUrl":"https://accounts.example.test/auth?state=s"}`, resp.Body.String())
	})

	t.Run("not configured is 409", func(t *testing.T) {
		g := &MockGoogle{AuthURLFunc: func(context.Context, string) (string, error) { return "", google.ErrNotConfigured }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/google/connect")
		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_configured")
	})
}

func TestRegisterGoogleDisconnect(t *testing.T) {
	t.Run("disconnects with 204", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Delete("/api/google")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, g.DisconnectCalls(), 1)
	})

	t.Run("failure is 500", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, docs, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "attachPromemoria": false,
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_connected")
	})

	t.Run("bad appointment id is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "bad id!", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("out of range duration is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "durationMinutes": 4,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("bad booking id is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": "../x",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("missing title is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{"appointmentId": "APP1", "start": "2026-03-10T09:30:00+01:00"})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterListAppointmentsVerdictsAndUpdated(t *testing.T) {
	t0 := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

	t.Run("verdicts follow unbooked order and updated uses the older list time", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) {
			return agenda.Result{
				Unbooked: []string{"0900a0000000001", "0900A0000000002", "0900A0000000003"},
				Verdicts: map[string]cup.Verdict{
					"0900A0000000001": {Reason: cup.VerdictHandled, Message: "gestita"},
					"0900A0000000002": {Reason: cup.VerdictBookable, Message: "ok"},
				},
			}, nil
		}}
		fresh := &MockFreshness{FetchedAtFunc: func(_ context.Context, source string) (map[string]time.Time, error) {
			switch source {
			case cache.SourceCUPPrescriptions:
				return map[string]time.Time{"": t0}, nil
			case cache.SourceFSEPrescriptions:
				return map[string]time.Time{"": t1}, nil
			case cache.SourceCUPAppointments:
				return map[string]time.Time{"0900A0000000001": t0}, nil
			}
			return map[string]time.Time{"0900A0000000002": t1}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, fresh, "window", false)

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"appointments": [],
			"failed": [],
			"unbooked": ["0900a0000000001","0900A0000000002","0900A0000000003"],
			"verdicts": [
				{"nre":"0900a0000000001","reason":"handled","message":"gestita"},
				{"nre":"0900A0000000002","reason":"bookable","message":"ok"}
			],
			"updated": {
				"lists": "2026-11-11T09:00:00Z",
				"appointments": {"0900A0000000001":"2026-11-11T10:00:00Z"},
				"verdicts": {"0900A0000000002":"2026-11-11T09:00:00Z"}
			}
		}`, resp.Body.String())
	})

	t.Run("freshness failure is omitted and still 200", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, nil }}
		fresh := &MockFreshness{FetchedAtFunc: func(context.Context, string) (map[string]time.Time, error) {
			return nil, errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, fresh, "window", false)

		resp := api.Get("/api/appointments")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"updated":{"lists":null,"appointments":{},"verdicts":{}}`)
	})
}

func TestRegisterListPrescriptionsUpdated(t *testing.T) {
	t.Run("updated is the fse list time", func(t *testing.T) {
		ag := &MockAgenda{PrescriptionsFunc: func(context.Context) ([]agenda.Row, error) { return nil, nil }}
		at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
		fresh := &MockFreshness{FetchedAtFunc: func(_ context.Context, source string) (map[string]time.Time, error) {
			if source != cache.SourceFSEPrescriptions {
				return nil, nil
			}
			return map[string]time.Time{"": at}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, fresh, "window", false)

		resp := api.Get("/api/prescriptions")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"prescriptions":[],"updated":"2026-11-11T10:00:00Z"}`, resp.Body.String())
	})
}

func TestRegisterRefresh(t *testing.T) {
	const (
		origin = "Origin: http://example.com"
		host   = "Host: example.com"
	)
	t.Run("all refreshes appointments and reports", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, nil }}
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"all": true})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, ag.AppointmentsCalls(), 1)
		assert.Len(t, reports.ListCalls(), 1)
	})

	t.Run("nres refreshes appointments only", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, nil }}
		reports := &MockReports{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"nres": []string{"0900A0000000001"}})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, ag.AppointmentsCalls(), 1)
		assert.Empty(t, reports.ListCalls())
	})

	t.Run("neither all nor nres is 422", func(t *testing.T) {
		ag := &MockAgenda{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, ag.AppointmentsCalls())
	})

	t.Run("malformed nre is 422", func(t *testing.T) {
		ag := &MockAgenda{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"nres": []string{"../x"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, ag.AppointmentsCalls())
	})

	t.Run("more than 200 nres is 422", func(t *testing.T) {
		nres := make([]string, 201)
		for i := range nres {
			nres[i] = "0900A0000000001"
		}
		ag := &MockAgenda{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"nres": nres})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		ag := &MockAgenda{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, "Origin: http://evil.test", map[string]any{"all": true})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, ag.AppointmentsCalls())
	})

	t.Run("agenda not logged in is 401", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, session.ErrNotLoggedIn }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"all": true})

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("reports login errors are skipped", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, nil }}
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, session.ErrSessionExpired }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"all": true})

		assert.Equal(t, http.StatusNoContent, resp.Code)
	})

	t.Run("reports other errors are propagated", func(t *testing.T) {
		ag := &MockAgenda{AppointmentsFunc: func(context.Context) (agenda.Result, error) { return agenda.Result{}, nil }}
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, ag, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/refresh", host, origin, map[string]any{"all": true})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestRegisterGoogleSettings(t *testing.T) {
	t.Run("returns settings and a redirect built from the request", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) {
			return google.Settings{ClientID: "id.apps.googleusercontent.com", CalendarID: "primary", DriveFolderID: "folder-1234567", HasSecret: true}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/google/settings", "Host: fts.example.test", "X-Forwarded-Proto: https")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"clientId":"id.apps.googleusercontent.com","calendarId":"primary","driveFolderId":"folder-1234567","hasSecret":true,"redirectUri":"https://fts.example.test/api/google/callback"}`, resp.Body.String())
	})

	t.Run("scheme defaults to http", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) { return google.Settings{}, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Get("/api/google/settings", "Host: localhost")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"redirectUri":"http://localhost/api/google/callback"`)
	})

	t.Run("store failure is 500", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) { return google.Settings{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusInternalServerError, api.Get("/api/google/settings").Code)
	})
}

func TestRegisterSaveGoogleSettings(t *testing.T) {
	const (
		origin = "Origin: http://example.com"
		host   = "Host: example.com"
	)

	t.Run("passes the settings to google and returns 204", func(t *testing.T) {
		g := &MockGoogle{SaveSettingsFunc: func(context.Context, google.Settings) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": "id.apps.googleusercontent.com", "clientSecret": "s", "calendarId": "cal", "driveFolderId": "folder-1234567"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, g.SaveSettingsCalls(), 1)
		assert.Equal(t, google.Settings{ClientID: "id.apps.googleusercontent.com", ClientSecret: "s", CalendarID: "cal", DriveFolderID: "folder-1234567"}, g.SaveSettingsCalls()[0].S)
	})

	t.Run("validation error is 422 with the message", func(t *testing.T) {
		g := &MockGoogle{SaveSettingsFunc: func(context.Context, google.Settings) error { return google.ErrClientIDInvalid }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": "x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), google.ErrClientIDInvalid.Error())
	})

	t.Run("too long client id is rejected by the schema", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": strings.Repeat("a", 201)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.SaveSettingsCalls())
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Put("/api/google/settings", host, "Origin: http://evil.test", map[string]string{"clientId": "id.apps.googleusercontent.com"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, g.SaveSettingsCalls())
	})
}

func TestRegisterGoogleConnectRedirect(t *testing.T) {
	t.Run("redirect is built from host and forwarded proto", func(t *testing.T) {
		g := &MockGoogle{AuthURLFunc: func(context.Context, string) (string, error) { return "https://accounts.example.test/auth", nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, g, &MockReports{}, emptyFreshness(), "window", false)

		resp := api.Post("/api/google/connect", "Host: fts.example.test", "X-Forwarded-Proto: https")

		require.Equal(t, http.StatusOK, resp.Code)
		require.Len(t, g.AuthURLCalls(), 1)
		assert.Equal(t, "https://fts.example.test/api/google/callback", g.AuthURLCalls()[0].RedirectURL)
	})
}
