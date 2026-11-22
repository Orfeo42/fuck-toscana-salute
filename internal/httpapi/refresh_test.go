package httpapi

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/notimpl"
	"fuck-toscana-salute/internal/refresh"
)

func newRefreshAPI(t *testing.T, refresher Refresher) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerRefresh(api, refresher)
	return api
}

func okRefresher() *MockRefresher {
	return &MockRefresher{StartFunc: func(refresh.Scope) error { return nil }}
}

func TestRegisterStartRefresh(t *testing.T) {
	t.Run("all is accepted", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"all": true})

		require.Equal(t, http.StatusAccepted, resp.Code)
		assert.JSONEq(t, `{"refreshing":true}`, resp.Body.String())
		assert.Equal(t, refresh.Scope{All: true}, refresher.StartCalls()[0].Scope)
	})

	t.Run("lists is accepted", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"lists": true})

		require.Equal(t, http.StatusAccepted, resp.Code)
		assert.Equal(t, refresh.Scope{Lists: true}, refresher.StartCalls()[0].Scope)
	})

	t.Run("reports is accepted", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"reports": true})

		require.Equal(t, http.StatusAccepted, resp.Code)
		assert.Equal(t, refresh.Scope{Reports: true}, refresher.StartCalls()[0].Scope)
	})

	t.Run("nres is accepted", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"nres": []string{validNRE}})

		require.Equal(t, http.StatusAccepted, resp.Code)
		assert.Equal(t, refresh.Scope{NREs: []string{validNRE}}, refresher.StartCalls()[0].Scope)
	})

	t.Run("an empty body is rejected", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, refresher.StartCalls())
	})

	t.Run("all false with no other scope is rejected", func(t *testing.T) {
		api := newRefreshAPI(t, okRefresher())

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"all": false, "nres": []string{}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("malformed nre is rejected", func(t *testing.T) {
		api := newRefreshAPI(t, okRefresher())

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"nres": []string{"../x"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("more than 200 nres is rejected", func(t *testing.T) {
		nres := make([]string, 201)
		for i := range nres {
			nres[i] = validNRE
		}
		api := newRefreshAPI(t, okRefresher())

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"nres": nres})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("a foreign origin is forbidden", func(t *testing.T) {
		refresher := okRefresher()
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, "Origin: http://evil.example", map[string]any{"all": true})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, refresher.StartCalls())
	})

	t.Run("a missing origin is forbidden", func(t *testing.T) {
		api := newRefreshAPI(t, okRefresher())

		resp := api.Post("/api/refresh/start", testHost, map[string]any{"all": true})

		assert.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("not implemented is 501", func(t *testing.T) {
		refresher := &MockRefresher{StartFunc: func(refresh.Scope) error { return notimpl.ErrNotImplemented }}
		api := newRefreshAPI(t, refresher)

		resp := api.Post("/api/refresh/start", testHost, testOrigin, map[string]any{"all": true})

		assert.Equal(t, http.StatusNotImplemented, resp.Code)
	})
}
