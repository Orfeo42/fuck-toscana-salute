package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
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

	t.Run("no document is 404", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fse.ErrNoDocument)
		assert.Equal(t, http.StatusNotFound, statusOf(t, err))
	})

	t.Run("upstream status error is 502", func(t *testing.T) {
		err := toHTTPError(ctx, "op", fmt.Errorf("wrap: %w", &upstream.StatusError{Status: 503}))
		assert.Equal(t, http.StatusBadGateway, statusOf(t, err))
	})

	t.Run("anything else is 500", func(t *testing.T) {
		err := toHTTPError(ctx, "op", errors.New("boom"))
		assert.Equal(t, http.StatusInternalServerError, statusOf(t, err))
	})
}
