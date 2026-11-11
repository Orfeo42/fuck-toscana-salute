package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var se huma.StatusError
	require.ErrorAs(t, err, &se)
	return se.GetStatus()
}

func TestToHTTPError(t *testing.T) {
	ctx := context.Background()

	t.Run("not logged in is 401", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", session.ErrNotLoggedIn))
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, err))
	})

	t.Run("session expired is 401", func(t *testing.T) {
		err := toHTTPError(ctx, "op", session.ErrSessionExpired)
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, err))
	})

	t.Run("gestione not ready is 409", func(t *testing.T) {
		err := toHTTPError(ctx, "op", cup.ErrGestioneNotReady)
		assert.Equal(t, http.StatusConflict, statusOf(t, err))
	})

	t.Run("referti no document is 404", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", referti.ErrNoDocument))
		assert.Equal(t, http.StatusNotFound, statusOf(t, err))
	})

	t.Run("referti not a pdf is 502", func(t *testing.T) {
		err := toHTTPError(ctx, "op", referti.ErrNotPDF)
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
	})

	t.Run("referti all failed is 502", func(t *testing.T) {
		err := toHTTPError(ctx, "op", referti.ErrAllFailed)
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
	})

	t.Run("google settings validation is 422 with message", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", google.ErrDriveFolderInvalid))
		assert.Equal(t, http.StatusUnprocessableEntity, statusOf(t, err))
		assert.Contains(t, err.Error(), google.ErrDriveFolderInvalid.Error())
	})

	t.Run("no document is 404", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fse.ErrNoDocument)
		assert.Equal(t, http.StatusNotFound, statusOf(t, err))
	})

	t.Run("no document logs a warn", func(t *testing.T) {
		var buf bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(previous)

		_ = toHTTPError(ctx, "op", fse.ErrNoDocument)
		logged := buf.String()
		assert.Contains(t, logged, "level=WARN")
		assert.Contains(t, logged, "document not found")
		assert.Contains(t, logged, "op=op")
	})

	t.Run("upstream status error is 502", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", &upstream.StatusError{Status: 503}))
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
	})

	t.Run("upstream status error logs exception class", func(t *testing.T) {
		var buf bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(previous)

		_ = toHTTPError(ctx, "op", &upstream.StatusError{Status: 500})
		logged := buf.String()
		assert.Contains(t, logged, "exception=")
		assert.Contains(t, logged, "error=")
	})

	t.Run("upstream status error logs body shape", func(t *testing.T) {
		var buf bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(previous)

		_ = toHTTPError(ctx, "op", &upstream.StatusError{Status: 500})
		assert.Contains(t, buf.String(), "shape=empty")
	})

	t.Run("google not configured is 409", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", google.ErrNotConfigured))
		assert.Equal(t, http.StatusConflict, statusOf(t, err))
		assert.Contains(t, err.Error(), "google_not_configured")
	})

	t.Run("google not connected is 401", func(t *testing.T) {
		err := toHTTPError(ctx, "op", google.ErrNotConnected)
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, err))
		assert.Contains(t, err.Error(), "google_not_connected")
	})

	t.Run("google api error is 502 logged as warning", func(t *testing.T) {
		var buf bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		defer slog.SetDefault(previous)

		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", &googleapi.Error{Code: 503}))
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
		assert.Contains(t, buf.String(), "level=WARN")
	})

	t.Run("oauth2 retrieve error is 502", func(t *testing.T) {
		err := toHTTPError(ctx, "op", &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "boom"})
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
	})

	t.Run("anything else is 500", func(t *testing.T) {
		err := toHTTPError(ctx, "op", errors.New("boom"))
		assert.Equal(t, http.StatusInternalServerError, statusOf(t, err))
	})
}
