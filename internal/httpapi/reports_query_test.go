package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/reports"
	"fuck-toscana-salute/internal/reportsview"
)

func newReportsQueryAPI(t *testing.T, view ReportsView) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerReportsQuery(api, view)
	return api
}

func emptyReportsView() *MockReportsView {
	return &MockReportsView{QueryFunc: func(context.Context, reportsview.Query) (reportsview.Page, error) {
		return reportsview.Page{}, nil
	}}
}

func TestRegisterQueryReports(t *testing.T) {
	t.Run("applies defaults for an empty body", func(t *testing.T) {
		view := emptyReportsView()
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		q := view.QueryCalls()[0].Q
		assert.Equal(t, reportsview.SortDate, q.Sort)
		assert.Equal(t, listview.DirDesc, q.Dir)
		assert.Equal(t, 1, q.Page)
		assert.Equal(t, 50, q.PageSize)
		assert.Empty(t, q.ExcludeKinds)
	})

	t.Run("passes the filters through", func(t *testing.T) {
		view := emptyReportsView()
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/query", map[string]any{"search": "rx", "excludeKinds": []string{"ris", "lab"}, "sort": "provider", "dir": "asc", "page": 2, "pageSize": 100})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, reportsview.Query{Search: "rx", ExcludeKinds: []string{"ris", "lab"}, Sort: reportsview.SortProvider, Dir: listview.DirAsc, Page: 2, PageSize: 100}, view.QueryCalls()[0].Q)
	})

	t.Run("unknown kind is rejected", func(t *testing.T) {
		view := emptyReportsView()
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/query", map[string]any{"excludeKinds": []string{"fse"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, view.QueryCalls())
	})

	t.Run("unknown sort is rejected", func(t *testing.T) {
		api := newReportsQueryAPI(t, emptyReportsView())

		resp := api.Post("/api/reports/query", map[string]any{"sort": "title"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("page size 30 is rejected", func(t *testing.T) {
		api := newReportsQueryAPI(t, emptyReportsView())

		resp := api.Post("/api/reports/query", map[string]any{"pageSize": 30})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("search over 100 characters is rejected", func(t *testing.T) {
		api := newReportsQueryAPI(t, emptyReportsView())

		resp := api.Post("/api/reports/query", map[string]any{"search": strings.Repeat("a", 101)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("maps the page and serializes empty arrays as lists", func(t *testing.T) {
		at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
		view := &MockReportsView{QueryFunc: func(context.Context, reportsview.Query) (reportsview.Page, error) {
			return reportsview.Page{
				Rows: []reportsview.Row{{
					ID:          "lab\x1f1\x1f\x1f",
					Ref:         referti.Ref{Kind: referti.KindLab, AbstractID: "1"},
					Kind:        "lab",
					KindLabel:   "Laboratorio analisi",
					Title:       "Emocromo",
					Date:        at,
					EditDate:    "2026-03-10",
					Origin:      reports.OriginRemote,
					LinkedLabel: "Collegato a 1 visita",
					Visits:      []reportsview.LinkedVisit{{NRE: validNRE, Title: "Visita"}},
				}},
				Total:          1,
				Kinds:          []reportsview.KindOption{{Key: "lab", Label: "Laboratorio analisi", Count: 1, Excluded: true}},
				ExcludedKinds:  []string{"lab"},
				Failures:       []reportsview.Failure{{Kind: "ris", KindLabel: "Radiologia", Message: "boom"}},
				FSEUnavailable: true,
				Updated:        listview.Updated{At: at, Label: "Aggiornato alle 10:00"},
				Issue:          "unavailable",
			}, nil
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		body := resp.Body.String()
		assert.Contains(t, body, `"fseUnavailable":true`)
		assert.Contains(t, body, `"editDate":"2026-03-10"`)
		assert.Contains(t, body, `"linkedLabel":"Collegato a 1 visita"`)
		assert.Contains(t, body, `"issue":"unavailable"`)
		assert.Contains(t, body, `"excludedKinds":["lab"]`)
		assert.Contains(t, body, `"excluded":true`)
		assert.Contains(t, body, `"failures":[{"kind":"ris"`)
	})

	t.Run("an empty page has empty lists not null", func(t *testing.T) {
		api := newReportsQueryAPI(t, emptyReportsView())

		resp := api.Post("/api/reports/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"rows":[]`)
		assert.Contains(t, resp.Body.String(), `"kinds":[]`)
		assert.Contains(t, resp.Body.String(), `"excludedKinds":[]`)
		assert.Contains(t, resp.Body.String(), `"failures":[]`)
	})
}

func TestRegisterGetReportRow(t *testing.T) {
	ref := map[string]any{"kind": "lab", "abstractId": "1", "detailId": "2", "repCode": "3"}

	t.Run("returns the row for the ref", func(t *testing.T) {
		view := &MockReportsView{RowFunc: func(context.Context, referti.Ref) (reportsview.RowResult, error) {
			return reportsview.RowResult{Row: reportsview.Row{ID: "x", Origin: reports.OriginRemote}, Refreshing: true}, nil
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/row", map[string]any{"ref": ref})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, referti.Ref{Kind: referti.KindLab, AbstractID: "1", DetailID: "2", RepCode: "3"}, view.RowCalls()[0].Ref)
		assert.Contains(t, resp.Body.String(), `"refreshing":true`)
	})

	t.Run("maps every row field", func(t *testing.T) {
		at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
		view := &MockReportsView{RowFunc: func(context.Context, referti.Ref) (reportsview.RowResult, error) {
			return reportsview.RowResult{Row: reportsview.Row{
				ID:          "x",
				Ref:         referti.Ref{Kind: referti.KindLocal, AbstractID: "7"},
				Kind:        "local",
				KindLabel:   "Caricato",
				Title:       "Eco",
				CustomTitle: "Eco",
				Date:        at,
				EditDate:    "2026-03-10",
				Provider:    "P",
				Unread:      true,
				Origin:      reports.OriginLocal,
				Visits:      []reportsview.LinkedVisit{{NRE: validNRE, Title: "Visita"}},
				LinkedLabel: "Collegato a 1 visita",
			}}, nil
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/row", map[string]any{"ref": ref})

		require.Equal(t, http.StatusOK, resp.Code)
		body := resp.Body.String()
		assert.Contains(t, body, `"kindLabel":"Caricato"`)
		assert.Contains(t, body, `"customTitle":"Eco"`)
		assert.Contains(t, body, `"editDate":"2026-03-10"`)
		assert.Contains(t, body, `"origin":"local"`)
		assert.Contains(t, body, `"unread":true`)
		assert.Contains(t, body, `"visits":[{"nre":"`+validNRE+`","title":"Visita"}]`)
	})

	t.Run("unknown report is 404", func(t *testing.T) {
		view := &MockReportsView{RowFunc: func(context.Context, referti.Ref) (reportsview.RowResult, error) {
			return reportsview.RowResult{}, reportsview.ErrNotFound
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/row", map[string]any{"ref": ref})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		view := &MockReportsView{}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/row", map[string]any{"ref": map[string]any{"kind": "zzz"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, view.RowCalls())
	})

	t.Run("missing ref is rejected", func(t *testing.T) {
		api := newReportsQueryAPI(t, &MockReportsView{})

		resp := api.Post("/api/reports/row", map[string]any{})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterReportVisitOptions(t *testing.T) {
	ref := map[string]any{"kind": "local", "abstractId": "7"}

	t.Run("returns the options", func(t *testing.T) {
		view := &MockReportsView{VisitOptionsFunc: func(context.Context, referti.Ref) ([]reportsview.LinkedVisit, error) {
			return []reportsview.LinkedVisit{{NRE: validNRE, Title: "Visita"}}, nil
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/visit-options", map[string]any{"ref": ref})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"options":[{"nre":"`+validNRE+`","title":"Visita"}]}`, resp.Body.String())
		assert.Equal(t, referti.KindLocal, view.VisitOptionsCalls()[0].Ref.Kind)
	})

	t.Run("no options is an empty list", func(t *testing.T) {
		view := &MockReportsView{VisitOptionsFunc: func(context.Context, referti.Ref) ([]reportsview.LinkedVisit, error) { return nil, nil }}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/visit-options", map[string]any{"ref": ref})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"options":[]}`, resp.Body.String())
	})

	t.Run("unknown report is 404", func(t *testing.T) {
		view := &MockReportsView{VisitOptionsFunc: func(context.Context, referti.Ref) ([]reportsview.LinkedVisit, error) {
			return nil, reportsview.ErrNotFound
		}}
		api := newReportsQueryAPI(t, view)

		resp := api.Post("/api/reports/visit-options", map[string]any{"ref": ref})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		api := newReportsQueryAPI(t, &MockReportsView{})

		resp := api.Post("/api/reports/visit-options", map[string]any{"ref": map[string]any{"kind": "zzz"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}
