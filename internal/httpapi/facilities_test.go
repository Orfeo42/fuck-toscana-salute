package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/facilities"
)

func newFacilitiesAPI(t *testing.T, fac Facilities) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerFacilities(api, fac)
	return api
}

func TestRegisterSearchFacilities(t *testing.T) {
	t.Run("returns the matches without the code", func(t *testing.T) {
		fac := &MockFacilities{SearchFunc: func(context.Context, string) ([]facilities.Facility, error) {
			return []facilities.Facility{{Code: "090001", Name: "Ospedale", Comune: "Arezzo", Provincia: "AR", Address: "Via Fleming 1", Source: facilities.SourceMinistero}}, nil
		}}
		api := newFacilitiesAPI(t, fac)

		resp := api.Post("/api/facilities/search", testHost, testOrigin, map[string]string{"query": "ospedale"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"facilities":[{"name":"Ospedale","comune":"Arezzo","provincia":"AR","address":"Via Fleming 1","source":"ministero"}]}`, resp.Body.String())
		require.Len(t, fac.SearchCalls(), 1)
		assert.Equal(t, "ospedale", fac.SearchCalls()[0].Query)
	})

	t.Run("no match returns an empty list", func(t *testing.T) {
		fac := &MockFacilities{SearchFunc: func(context.Context, string) ([]facilities.Facility, error) {
			return nil, nil
		}}
		api := newFacilitiesAPI(t, fac)

		resp := api.Post("/api/facilities/search", testHost, testOrigin, map[string]string{"query": "zzzz"})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"facilities":[]}`, resp.Body.String())
	})

	t.Run("query shorter than 2 is rejected", func(t *testing.T) {
		fac := &MockFacilities{}
		api := newFacilitiesAPI(t, fac)

		resp := api.Post("/api/facilities/search", testHost, testOrigin, map[string]string{"query": "a"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, fac.SearchCalls())
	})

	t.Run("query longer than 100 is rejected", func(t *testing.T) {
		api := newFacilitiesAPI(t, &MockFacilities{})

		resp := api.Post("/api/facilities/search", testHost, testOrigin, map[string]string{"query": strings.Repeat("a", 101)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("search failure returns 500", func(t *testing.T) {
		fac := &MockFacilities{SearchFunc: func(context.Context, string) ([]facilities.Facility, error) {
			return nil, errors.New("db down")
		}}
		api := newFacilitiesAPI(t, fac)

		resp := api.Post("/api/facilities/search", testHost, testOrigin, map[string]string{"query": "ospedale"})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}
