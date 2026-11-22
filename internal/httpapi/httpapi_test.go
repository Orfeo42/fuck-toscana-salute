package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

func TestNew(t *testing.T) {
	t.Run("serves the session endpoint", func(t *testing.T) {
		store := &MockSessionStore{StatusFunc: func() session.Status { return session.Status{} }}
		flow := &MockLoginFlow{StatesFunc: func() map[session.Portal]loginflow.State { return nil }}
		rec := httptest.NewRecorder()
		New(store, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, &MockFacilities{}, &MockVisitsView{}, &MockReportsView{}, &MockDrugsView{}, &MockRefresher{}, "window", false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
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
		register(api, store, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, store, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", true)

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
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusConflict, resp.Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartFunc: func(context.Context, session.Portal) error {
			return errors.New("boom")
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/login/cup")
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/login/xyz")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.StartCalls())
	})
}

func TestRegisterStartLoginAll(t *testing.T) {
	t.Run("accepts and starts the combined flow", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/login")
		assert.Equal(t, http.StatusAccepted, resp.Code)
		assert.Len(t, flow.StartAllCalls(), 1)
	})

	t.Run("conflict when a login is in progress", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return loginflow.ErrLoginInProgress }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusConflict, api.Post("/api/login").Code)
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{StartAllFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
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
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
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
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := importToken(t, "r_toscan__fse_web", time.Now().Add(time.Hour))

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "fse", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U", "tessera": "80380000000000000000"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Empty(t, got.Identity.TesseraSanitaria)
		assert.False(t, got.Tokens.GestioneVerified)
	})

	t.Run("missing origin is forbidden", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, "Origin: https://evil.example", map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("mismatching token is unprocessable", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return login.ErrTokenMismatch }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), "non appartengono")
	})

	t.Run("expired token is unprocessable", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return login.ErrTokenExpired }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), "scaduto")
	})

	t.Run("malformed token is unprocessable and never reaches the flow", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": "not-a-jwt", "refresh": "not-a-jwt", "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("internal error on other failures", func(t *testing.T) {
		flow := &MockLoginFlow{ImportFunc: func(context.Context, login.Result) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)
		tok := validCUP(t)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "cup", "access": tok, "refresh": tok, "cf": "RSSMRA80A01H501U"})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("unknown portal is rejected by the schema", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)

		resp := api.Post("/api/login/import", host, origin, map[string]string{"portal": "xyz", "access": "a", "refresh": "b", "cf": "c"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, flow.ImportCalls())
	})

	t.Run("empty access is rejected by the schema", func(t *testing.T) {
		flow := &MockLoginFlow{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)

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
		register(api, store, flow, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "bookmarklet", false)

		resp := api.Get("/api/session")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"loginMode":"bookmarklet"`)
	})
}

func TestRegisterLogout(t *testing.T) {
	t.Run("clears the store and returns 204", func(t *testing.T) {
		store := &MockSessionStore{ClearFunc: func() {}}
		_, api := humatest.New(t)
		register(api, store, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/logout")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, store.ClearCalls(), 1)
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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/documents/prescription")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PrescriptionPDFCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PrescriptionPDFFunc: func(context.Context, string) ([]byte, error) {
			return nil, fse.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/documents/booking?id=a-b")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("missing id is rejected", func(t *testing.T) {
		docs := &MockDocuments{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusUnprocessableEntity, api.Get("/api/documents/booking").Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("not found when document is missing", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrNoDocument
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusNotFound, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})

	t.Run("gestione not ready is a conflict", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, &MockGoogle{}, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusConflict, api.Get("/api/documents/booking?id=ABC123456789").Code)
	})
}

func TestRegisterGoogleStatus(t *testing.T) {
	t.Run("returns configured and connected", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) {
			return google.Status{Configured: true, Connected: true, ClientID: "123456789012-abc.apps.googleusercontent.com", Drive: true}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/google")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"configured":true,"connected":true,"clientId":"123456789012-abc.apps.googleusercontent.com","drive":true}`, resp.Body.String())
	})

	t.Run("store failure is 500", func(t *testing.T) {
		g := &MockGoogle{StatusFunc: func(context.Context) (google.Status, error) { return google.Status{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusInternalServerError, api.Get("/api/google").Code)
	})
}

func TestRegisterExchangeGoogleCode(t *testing.T) {
	const (
		origin = "Origin: http://example.com"
		host   = "Host: example.com"
		xhr    = "X-Requested-With: XmlHttpRequest"
	)
	body := map[string]string{"code": "auth-code"}

	t.Run("exchanges the code with the page origin and returns 204", func(t *testing.T) {
		g := &MockGoogle{ExchangeCodeFunc: func(context.Context, string, string) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, xhr, body)

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, g.ExchangeCodeCalls(), 1)
		assert.Equal(t, "auth-code", g.ExchangeCodeCalls()[0].Code)
		assert.Equal(t, "http://example.com", g.ExchangeCodeCalls()[0].Origin)
	})

	t.Run("origin honours forwarded proto", func(t *testing.T) {
		g := &MockGoogle{ExchangeCodeFunc: func(context.Context, string, string) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", "Host: fts.example.test", "Origin: https://fts.example.test", "X-Forwarded-Proto: https", xhr, body)

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, g.ExchangeCodeCalls(), 1)
		assert.Equal(t, "https://fts.example.test", g.ExchangeCodeCalls()[0].Origin)
	})

	t.Run("missing requested-with header is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, body)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.ExchangeCodeCalls())
	})

	t.Run("empty code is 422", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, xhr, map[string]string{"code": ""})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.ExchangeCodeCalls())
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, "Origin: http://evil.test", xhr, body)

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, g.ExchangeCodeCalls())
	})

	t.Run("not configured is 409", func(t *testing.T) {
		g := &MockGoogle{ExchangeCodeFunc: func(context.Context, string, string) error { return google.ErrNotConfigured }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, xhr, body)

		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_configured")
	})

	t.Run("no refresh token is 409 grant reset", func(t *testing.T) {
		g := &MockGoogle{ExchangeCodeFunc: func(context.Context, string, string) error { return google.ErrNoRefreshToken }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, xhr, body)

		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_grant_reset")
	})

	t.Run("missing scopes is 403", func(t *testing.T) {
		g := &MockGoogle{ExchangeCodeFunc: func(context.Context, string, string) error { return google.ErrScopesMissing }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/google/code", host, origin, xhr, body)

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_scopes_missing")
	})
}

func TestRegisterGoogleDisconnect(t *testing.T) {
	t.Run("disconnects with 204", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Delete("/api/google")
		assert.Equal(t, http.StatusNoContent, resp.Code)
		assert.Len(t, g.DisconnectCalls(), 1)
	})

	t.Run("failure is 500", func(t *testing.T) {
		g := &MockGoogle{DisconnectFunc: func(context.Context) error { return errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusInternalServerError, api.Delete("/api/google").Code)
	})
}

func TestRegisterLookupCalendarEvent(t *testing.T) {
	t.Run("existing event", func(t *testing.T) {
		g := &MockGoogle{LookupEventFunc: func(context.Context, string) (google.EventLookup, error) {
			return google.EventLookup{Exists: true, HTMLLink: "https://calendar.example.test/e"}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events/lookup", map[string]any{"appointmentId": "APP1"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"exists":true,"htmlLink":"https://calendar.example.test/e"}`, resp.Body.String())
		assert.Equal(t, "APP1", g.LookupEventCalls()[0].Key)
	})

	t.Run("missing event", func(t *testing.T) {
		g := &MockGoogle{LookupEventFunc: func(context.Context, string) (google.EventLookup, error) { return google.EventLookup{}, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events/lookup", map[string]any{"appointmentId": "APP1"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"exists":false,"htmlLink":""}`, resp.Body.String())
	})

	t.Run("not connected is 401", func(t *testing.T) {
		g := &MockGoogle{LookupEventFunc: func(context.Context, string) (google.EventLookup, error) {
			return google.EventLookup{}, google.ErrNotConnected
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusUnauthorized, api.Post("/api/calendar/events/lookup", map[string]any{"appointmentId": "APP1"}).Code)
	})
}

func TestRegisterCreateCalendarEvent(t *testing.T) {
	const fakeBooking = "ABC123456789"
	noGuests := func(context.Context) (google.Settings, error) { return google.Settings{}, nil }
	okEvent := func(context.Context, google.Event, []byte) (google.EventResult, error) {
		return google.EventResult{HTMLLink: "https://calendar.example.test/e", Created: true}, nil
	}

	t.Run("attaches the promemoria when booking id is given", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) { return []byte("%PDF-fake"), nil }}
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, g, &MockReports{}, &MockLocalRecords{}, "window", false)

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

	t.Run("regenerate is passed to google", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "regenerate": true,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.True(t, g.AddEventCalls()[0].Ev.Regenerate)
	})

	t.Run("missing guests use the settings guests", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: func(context.Context) (google.Settings, error) {
			return google.Settings{Guests: []string{"guest@example.com"}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, []string{"guest@example.com"}, g.AddEventCalls()[0].Ev.Guests)
	})

	t.Run("explicit empty guests mean no guests", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "guests": []string{},
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, g.AddEventCalls()[0].Ev.Guests)
		assert.Empty(t, g.SettingsCalls())
	})

	t.Run("explicit guests are cleaned", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "guests": []string{" guest@example.com ", "GUEST@example.com"},
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, []string{"guest@example.com"}, g.AddEventCalls()[0].Ev.Guests)
	})

	t.Run("invalid guest is 422 and no event is added", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "guests": []string{"nope"},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("duration is taken from the request", func(t *testing.T) {
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "durationMinutes": 90, "attachPromemoria": false,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, 90*time.Minute, g.AddEventCalls()[0].Ev.Duration)
	})

	t.Run("attachPromemoria false never fetches the pdf", func(t *testing.T) {
		docs := &MockDocuments{}
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": fakeBooking, "attachPromemoria": false,
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
		assert.Nil(t, g.AddEventCalls()[0].Pdf)
	})

	t.Run("no booking id never fetches the pdf", func(t *testing.T) {
		docs := &MockDocuments{}
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, docs.PromemoriaCalls())
	})

	t.Run("promemoria failure skips the event", func(t *testing.T) {
		docs := &MockDocuments{PromemoriaFunc: func(context.Context, string) ([]byte, error) { return nil, session.ErrNotLoggedIn }}
		g := &MockGoogle{AddEventFunc: okEvent, SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, docs, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": fakeBooking,
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("google not configured is 409", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests, AddEventFunc: func(context.Context, google.Event, []byte) (google.EventResult, error) {
			return google.EventResult{}, google.ErrNotConfigured
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "attachPromemoria": false,
		})
		assert.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_configured")
	})

	t.Run("google not connected is 401", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests, AddEventFunc: func(context.Context, google.Event, []byte) (google.EventResult, error) {
			return google.EventResult{}, google.ErrNotConnected
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "attachPromemoria": false,
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_not_connected")
	})

	t.Run("bad appointment id is 422", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "bad id!", "title": "Visita", "start": "2026-03-10T09:30:00+01:00",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("out of range duration is 422", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "durationMinutes": 4,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.AddEventCalls())
	})

	t.Run("bad booking id is 422", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{
			"appointmentId": "APP1", "title": "Visita", "start": "2026-03-10T09:30:00+01:00", "bookingId": "../x",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("missing title is 422", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: noGuests}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/calendar/events", map[string]any{"appointmentId": "APP1", "start": "2026-03-10T09:30:00+01:00"})
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterGoogleSettings(t *testing.T) {
	t.Run("returns settings and the javascript origin built from the request", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) {
			return google.Settings{ClientID: "id.apps.googleusercontent.com", CalendarID: "primary", DriveFolderID: "folder-1234567", APIKey: "key-abc", HasSecret: true}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/google/settings", "Host: fts.example.test", "X-Forwarded-Proto: https")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"clientId":"id.apps.googleusercontent.com","calendarId":"primary","driveFolderId":"folder-1234567","apiKey":"key-abc","guests":[],"hasSecret":true,"javascriptOrigin":"https://fts.example.test"}`, resp.Body.String())
	})

	t.Run("returns the guests", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) {
			return google.Settings{Guests: []string{"guest@example.com"}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/google/settings", "Host: localhost")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"guests":["guest@example.com"]`)
	})

	t.Run("scheme defaults to http", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) { return google.Settings{}, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Get("/api/google/settings", "Host: localhost")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"javascriptOrigin":"http://localhost"`)
	})

	t.Run("store failure is 500", func(t *testing.T) {
		g := &MockGoogle{SettingsFunc: func(context.Context) (google.Settings, error) { return google.Settings{}, errors.New("boom") }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

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
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": "id.apps.googleusercontent.com", "clientSecret": "s", "calendarId": "cal", "driveFolderId": "folder-1234567", "apiKey": "key-abc"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, g.SaveSettingsCalls(), 1)
		assert.Equal(t, google.Settings{ClientID: "id.apps.googleusercontent.com", ClientSecret: "s", CalendarID: "cal", DriveFolderID: "folder-1234567", APIKey: "key-abc"}, g.SaveSettingsCalls()[0].S)
	})

	t.Run("passes the guests to google", func(t *testing.T) {
		g := &MockGoogle{SaveSettingsFunc: func(context.Context, google.Settings) error { return nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]any{"clientId": "id.apps.googleusercontent.com", "guests": []string{"guest@example.com"}})

		require.Equal(t, http.StatusNoContent, resp.Code)
		assert.Equal(t, []string{"guest@example.com"}, g.SaveSettingsCalls()[0].S.Guests)
	})

	t.Run("more than ten guests is rejected by the schema", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]any{"clientId": "id.apps.googleusercontent.com", "guests": make([]string, 11)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.SaveSettingsCalls())
	})

	t.Run("invalid guest error is 422 in italian", func(t *testing.T) {
		g := &MockGoogle{SaveSettingsFunc: func(context.Context, google.Settings) error { return google.ErrGuestInvalid }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]any{"clientId": "id.apps.googleusercontent.com", "guests": []string{"nope"}})

		require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), "Indirizzo email dell'invitato non valido")
	})

	t.Run("validation error is 422 with the message", func(t *testing.T) {
		g := &MockGoogle{SaveSettingsFunc: func(context.Context, google.Settings) error { return google.ErrClientIDInvalid }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": "x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Contains(t, resp.Body.String(), google.ErrClientIDInvalid.Error())
	})

	t.Run("too long client id is rejected by the schema", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, origin, map[string]string{"clientId": strings.Repeat("a", 201)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.SaveSettingsCalls())
	})

	t.Run("foreign origin is forbidden", func(t *testing.T) {
		g := &MockGoogle{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, g, &MockReports{}, &MockLocalRecords{}, "window", false)

		resp := api.Put("/api/google/settings", host, "Origin: http://evil.test", map[string]string{"clientId": "id.apps.googleusercontent.com"})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, g.SaveSettingsCalls())
	})
}
