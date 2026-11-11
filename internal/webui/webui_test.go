package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>index</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
}

func TestHandler(t *testing.T) {
	t.Run("serves embedded assets with security headers", func(t *testing.T) {
		rec := httptest.NewRecorder()

		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, rec.Code)
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	})
}

func TestHandlerFS(t *testing.T) {
	t.Run("serves index at root", func(t *testing.T) {
		rec := httptest.NewRecorder()

		handler(builtFS()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "index")
	})

	t.Run("serves an asset", func(t *testing.T) {
		rec := httptest.NewRecorder()

		handler(builtFS()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "console.log(1)", rec.Body.String())
	})

	t.Run("503 when index is missing", func(t *testing.T) {
		rec := httptest.NewRecorder()

		handler(fstest.MapFS{".gitkeep": {}}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "web UI not built: run \"just build\"\n", rec.Body.String())
	})

	t.Run("security headers on success", func(t *testing.T) {
		rec := httptest.NewRecorder()

		handler(builtFS()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
		assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"))
	})

	t.Run("security headers on 503", func(t *testing.T) {
		rec := httptest.NewRecorder()

		handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
		assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"))
	})
}
