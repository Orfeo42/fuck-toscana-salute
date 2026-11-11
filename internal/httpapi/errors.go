package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func toHTTPError(ctx context.Context, op string, err error) error {
	if errors.Is(err, session.ErrNotLoggedIn) {
		return huma.Error401Unauthorized("not logged in")
	}
	if errors.Is(err, session.ErrSessionExpired) {
		return huma.Error401Unauthorized("session expired")
	}
	if errors.Is(err, cup.ErrGestioneNotReady) {
		return huma.Error409Conflict("complete the Gestione Prenotazioni login step")
	}
	if errors.Is(err, fse.ErrNoDocument) {
		return huma.Error404NotFound("document not found")
	}
	var statusErr *upstream.StatusError
	if errors.As(err, &statusErr) {
		slog.WarnContext(ctx, "upstream error", slog.String("op", op), slog.Int("status", statusErr.Status))
		return huma.Error502BadGateway("upstream error")
	}
	slog.ErrorContext(ctx, "request failed", slog.String("op", op), slog.String("error", err.Error()))
	return huma.Error500InternalServerError("internal error")
}
