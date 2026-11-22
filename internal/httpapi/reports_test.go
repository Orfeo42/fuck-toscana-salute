package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
)

func TestRegisterListReportDocuments(t *testing.T) {
	t.Run("maps documents and passes the ref in the body", func(t *testing.T) {
		accepted := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
		reports := &MockReports{DocumentsFunc: func(context.Context, referti.Ref) ([]referti.Document, error) {
			return []referti.Document{{Position: "1", Label: "Verbale", AcceptedAt: accepted, Provider: "ASL"}, {Position: "2"}}, nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/documents", map[string]string{"kind": "lab", "abstractId": "ABS1", "detailId": "DET1", "repCode": "REP1"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"documents":[{"position":"1","label":"Verbale","acceptedAt":"2026-10-01T08:00:00Z","provider":"ASL"},{"position":"2","label":""}]}`, resp.Body.String())
		require.Len(t, reports.DocumentsCalls(), 1)
		assert.Equal(t, referti.Ref{Kind: referti.KindLab, AbstractID: "ABS1", DetailID: "DET1", RepCode: "REP1"}, reports.DocumentsCalls()[0].Ref)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		reports := &MockReports{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/documents", map[string]string{"kind": "xyz"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, reports.DocumentsCalls())
	})

	t.Run("not logged in is 401", func(t *testing.T) {
		reports := &MockReports{DocumentsFunc: func(context.Context, referti.Ref) ([]referti.Document, error) { return nil, session.ErrNotLoggedIn }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusUnauthorized, api.Post("/api/reports/documents", map[string]string{"kind": "lab"}).Code)
	})
}

func TestRegisterGetReportPDF(t *testing.T) {
	t.Run("returns the pdf with headers", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return []byte("%PDF-1.4 fake"), nil }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "ps", "detailId": "DET1", "position": "3"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/pdf", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="referto.pdf"`, resp.Header().Get("Content-Disposition"))
		assert.Equal(t, "%PDF-1.4 fake", resp.Body.String())
		require.Len(t, reports.DownloadCalls(), 1)
		assert.Equal(t, referti.KindER, reports.DownloadCalls()[0].Ref.Kind)
		assert.Equal(t, "3", reports.DownloadCalls()[0].Position)
	})

	t.Run("png is served as an image", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) {
			return []byte("\x89PNG\r\n\x1a\n0000"), nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "local", "abstractId": "0123456789abcdef0123456789abcdef"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "image/png", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="referto.png"`, resp.Header().Get("Content-Disposition"))
	})

	t.Run("jpeg is served as an image", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) {
			return []byte("\xff\xd8\xff\xe0\x00\x10JFIF0000"), nil
		}}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "local", "abstractId": "0123456789abcdef0123456789abcdef"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "image/jpeg", resp.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="referto.jpg"`, resp.Header().Get("Content-Disposition"))
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		reports := &MockReports{}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		resp := api.Post("/api/reports/pdf", map[string]string{"kind": "xyz"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, reports.DownloadCalls())
	})

	t.Run("missing document is 404", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return nil, referti.ErrNoDocument }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusNotFound, api.Post("/api/reports/pdf", map[string]string{"kind": "lab"}).Code)
	})

	t.Run("not a pdf is 502", func(t *testing.T) {
		reports := &MockReports{DownloadFunc: func(context.Context, referti.Ref, string) ([]byte, error) { return nil, referti.ErrNotPDF }}
		_, api := humatest.New(t)
		register(api, &MockSessionStore{}, &MockLoginFlow{}, &MockDocuments{}, &MockGoogle{}, reports, &MockLocalRecords{}, "window", false)

		assert.Equal(t, http.StatusBadGateway, api.Post("/api/reports/pdf", map[string]string{"kind": "lab"}).Code)
	})
}
