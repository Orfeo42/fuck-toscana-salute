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

	"fuck-toscana-salute/internal/drugsview"
	"fuck-toscana-salute/internal/listview"
)

func newDrugsAPI(t *testing.T, view DrugsView) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerDrugs(api, view)
	return api
}

func emptyDrugs() *MockDrugsView {
	return &MockDrugsView{QueryFunc: func(context.Context, drugsview.Query) (drugsview.Page, error) {
		return drugsview.Page{}, nil
	}}
}

func TestRegisterQueryDrugs(t *testing.T) {
	t.Run("applies defaults for an empty body", func(t *testing.T) {
		view := emptyDrugs()
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		q := view.QueryCalls()[0].Q
		assert.Equal(t, drugsview.SortDate, q.Sort)
		assert.Equal(t, listview.DirDesc, q.Dir)
		assert.Equal(t, 1, q.Page)
		assert.Equal(t, 50, q.PageSize)
		assert.Nil(t, q.ExcludeStatuses)
	})

	t.Run("empty excludeStatuses reaches the service as empty not nil", func(t *testing.T) {
		view := emptyDrugs()
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/query", map[string]any{"excludeStatuses": []string{}})

		require.Equal(t, http.StatusOK, resp.Code)
		q := view.QueryCalls()[0].Q
		assert.NotNil(t, q.ExcludeStatuses)
		assert.Empty(t, q.ExcludeStatuses)
	})

	t.Run("passes the filters through", func(t *testing.T) {
		view := emptyDrugs()
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/query", map[string]any{"search": "tachi", "excludeStatuses": []string{"done"}, "sort": "name", "dir": "asc", "page": 2, "pageSize": 25, "focus": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, drugsview.Query{Search: "tachi", ExcludeStatuses: []string{"done"}, Sort: drugsview.SortName, Dir: listview.DirAsc, Page: 2, PageSize: 25, Focus: validNRE}, view.QueryCalls()[0].Q)
	})

	t.Run("a visit-only status is rejected", func(t *testing.T) {
		api := newDrugsAPI(t, emptyDrugs())

		resp := api.Post("/api/drugs/query", map[string]any{"excludeStatuses": []string{"unverified"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("unknown sort is rejected", func(t *testing.T) {
		api := newDrugsAPI(t, emptyDrugs())

		resp := api.Post("/api/drugs/query", map[string]any{"sort": "place"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("page size 30 is rejected", func(t *testing.T) {
		api := newDrugsAPI(t, emptyDrugs())

		resp := api.Post("/api/drugs/query", map[string]any{"pageSize": 30})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("search over 100 characters is rejected", func(t *testing.T) {
		api := newDrugsAPI(t, emptyDrugs())

		resp := api.Post("/api/drugs/query", map[string]any{"search": strings.Repeat("a", 101)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("maps the page and serializes empty arrays as lists", func(t *testing.T) {
		at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
		view := &MockDrugsView{QueryFunc: func(context.Context, drugsview.Query) (drugsview.Page, error) {
			return drugsview.Page{
				Rows:    []drugsview.Row{{NRE: validNRE, Title: "Farmaco", Date: at, Status: drugsview.Status{Key: "prescribed", Label: "Da ritirare", Tone: listview.ToneToBook}}},
				Total:   1,
				Focus:   &listview.Focus{NRE: validNRE},
				Updated: listview.Updated{At: at, Label: "Aggiornato alle 10:00"},
				Issue:   "none",
			}, nil
		}}
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		body := resp.Body.String()
		assert.Contains(t, body, `"status":{"key":"prescribed","label":"Da ritirare","tone":"tobook"}`)
		assert.Contains(t, body, `"statuses":[]`)
		assert.Contains(t, body, `"excludedStatuses":[]`)
		assert.Contains(t, body, `"focus":{"nre":"`+validNRE+`","found":false}`)
	})

	t.Run("an empty page has empty rows not null", func(t *testing.T) {
		api := newDrugsAPI(t, emptyDrugs())

		resp := api.Post("/api/drugs/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"rows":[]`)
	})
}

func TestRegisterGetDrugRow(t *testing.T) {
	t.Run("returns the row", func(t *testing.T) {
		view := &MockDrugsView{RowFunc: func(context.Context, string) (drugsview.RowResult, error) {
			return drugsview.RowResult{Row: drugsview.Row{NRE: validNRE, Title: "Farmaco", Status: drugsview.Status{Key: "done", Label: "Erogata", Tone: listview.ToneDone}}, Refreshing: true}, nil
		}}
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/row", map[string]any{"nre": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, validNRE, view.RowCalls()[0].Nre)
		assert.Contains(t, resp.Body.String(), `"refreshing":true`)
	})

	t.Run("unknown drug is 404", func(t *testing.T) {
		view := &MockDrugsView{RowFunc: func(context.Context, string) (drugsview.RowResult, error) {
			return drugsview.RowResult{}, drugsview.ErrNotFound
		}}
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/row", map[string]any{"nre": validNRE})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("malformed nre is rejected", func(t *testing.T) {
		view := &MockDrugsView{}
		api := newDrugsAPI(t, view)

		resp := api.Post("/api/drugs/row", map[string]any{"nre": "../x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, view.RowCalls())
	})
}
