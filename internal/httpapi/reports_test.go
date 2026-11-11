package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
)

func TestRegisterListReports(t *testing.T) {
	t.Run("maps reports, failures and updated", func(t *testing.T) {
		date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) {
			return referti.Listing{
				Reports: []referti.Report{
					{Ref: referti.Ref{Kind: referti.KindLab, AbstractID: "ABS1", DetailID: "DET1", RepCode: "REP1"}, Date: date, Provider: "ASL", Unread: true},
					{Ref: referti.Ref{Kind: referti.KindRadiology, DetailID: "DET2"}, Hidden: true},
				},
				Failures: []referti.Failure{{Kind: referti.KindER, Err: errors.New("upstream: unexpected status 502")}},
			}, nil
		}}
		fresh := &MockFreshness{FetchedAtFunc: func(_ context.Context, source string) (map[string]time.Time, error) {
			assert.Equal(t, cache.SourceFSEReports, source)
			return map[string]time.Time{"": date}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, fresh, "window", false)

		resp := api.Get("/api/reports")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{
			"reports": [
				{"kind":"lab","kindLabel":"Laboratorio analisi","abstractId":"ABS1","detailId":"DET1","repCode":"REP1","date":"2026-10-01T00:00:00Z","provider":"ASL","unread":true,"hidden":false},
				{"kind":"ris","kindLabel":"Radiologia","abstractId":"","detailId":"DET2","repCode":"","provider":"","unread":false,"hidden":true}
			],
			"failures": [{"kind":"ps","kindLabel":"Pronto soccorso","message":"upstream: unexpected status 502"}],
			"updated": "2026-10-01T00:00:00Z"
		}`, resp.Body.String())
	})

	t.Run("empty listing has empty arrays and null updated", func(t *testing.T) {
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Get("/api/reports")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"reports":[],"failures":[],"updated":null}`, resp.Body.String())
	})

	t.Run("not logged in is 401", func(t *testing.T) {
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, session.ErrNotLoggedIn }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusUnauthorized, api.Get("/api/reports").Code)
	})

	t.Run("all kinds failed is 502", func(t *testing.T) {
		reports := &MockReports{ListFunc: func(context.Context) (referti.Listing, error) { return referti.Listing{}, referti.ErrAllFailed }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusBadGateway, api.Get("/api/reports").Code)
	})
}

func TestRegisterListReportDocuments(t *testing.T) {
	t.Run("maps documents and passes the ref in the body", func(t *testing.T) {
		accepted := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
		reports := &MockReports{DocumentsFunc: func(context.Context, referti.Ref) ([]referti.Document, error) {
			return []referti.Document{{Position: "1", Label: "Verbale", AcceptedAt: accepted, Provider: "ASL"}, {Position: "2"}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/reports/documents", map[string]string{"kind": "lab", "abstractId": "ABS1", "detailId": "DET1", "repCode": "REP1"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"documents":[{"position":"1","label":"Verbale","acceptedAt":"2026-10-01T08:00:00Z","provider":"ASL"},{"position":"2","label":""}]}`, resp.Body.String())
		require.Len(t, reports.DocumentsCalls(), 1)
		assert.Equal(t, referti.Ref{Kind: referti.KindLab, AbstractID: "ABS1", DetailID: "DET1", RepCode: "REP1"}, reports.DocumentsCalls()[0].Ref)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		reports := &MockReports{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/reports/documents", map[string]string{"kind": "xyz"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, reports.DocumentsCalls())
	})

	t.Run("not logged in is 401", func(t *testing.T) {
		reports := &MockReports{DocumentsFunc: func(context.Context, referti.Ref) ([]referti.Document, error) { return nil, session.ErrNotLoggedIn }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusUnauthorized, api.Post("/api/reports/documents", map[string]string{"kind": "lab"}).Code)
	})
}

func TestRegisterGetReportPDF(t *testing.T) {
	t.Run("returns the pdf with headers", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return []byte("%PDF-1.4 fake"), nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "ps", "detailId": "DET1", "position": "3"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/pdf", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="referto.pdf"`, resp.Header().Get("Content-Disposition"))
		assert.Equal(t, "%PDF-1.4 fake", resp.Body.String())
		require.Len(t, reports.DownloadCalls(), 1)
		assert.Equal(t, referti.KindER, reports.DownloadCalls()[0].Ref.Kind)
		assert.Equal(t, "3", reports.DownloadCalls()[0].Position)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		reports := &MockReports{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "xyz"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, reports.DownloadCalls())
	})

	t.Run("missing document is 404", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return nil, referti.ErrNoDocument }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusNotFound, api.Post("/api/reports/pdf", map[string]string{"kind": "lab"}).Code)
	})

	t.Run("not a pdf is 502", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return nil, referti.ErrNotPDF }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockAgenda{}, &MockDocuments{}, &MockGoogle{}, reports, emptyFreshness(), "window", false)

		assert.Equal(t, http.StatusBadGateway, api.Post("/api/reports/pdf", map[string]string{"kind": "lab"}).Code)
	})
}
