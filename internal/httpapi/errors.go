package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/referti"
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
	if errors.Is(err, fse.ErrNoDocument) || errors.Is(err, cup.ErrNoDocument) || errors.Is(err, referti.ErrNoDocument) {
		slog.WarnContext(ctx, "document not found", slog.String("op", op), slog.String("error", err.Error()))
		return huma.Error404NotFound("document not found")
	}
	if errors.Is(err, referti.ErrNotPDF) || errors.Is(err, referti.ErrAllFailed) {
		slog.WarnContext(ctx, "upstream error", slog.String("op", op), slog.String("error", err.Error()))
		return huma.Error502BadGateway("upstream error")
	}
	if errors.Is(err, google.ErrClientIDRequired) || errors.Is(err, google.ErrClientIDInvalid) || errors.Is(err, google.ErrClientSecretRequired) || errors.Is(err, google.ErrDriveFolderInvalid) {
		return huma.Error422UnprocessableEntity(err.Error())
	}
	if errors.Is(err, google.ErrNotConfigured) {
		return huma.Error409Conflict("google_not_configured")
	}
	if errors.Is(err, google.ErrNotConnected) {
		return huma.Error401Unauthorized("google_not_connected")
	}
	var googleErr *googleapi.Error
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &googleErr) || errors.As(err, &retrieveErr) {
		slog.WarnContext(ctx, "google error", slog.String("op", op), slog.String("error", err.Error()))
		return huma.Error502BadGateway("google error")
	}
	var statusErr *upstream.StatusError
	if errors.As(err, &statusErr) {
		slog.WarnContext(ctx, "upstream error", slog.String("op", op), slog.Int("status", statusErr.Status), slog.String("error", err.Error()), slog.String("exception", statusErr.ExceptionClass()), slog.String("shape", statusErr.Shape()))
		return huma.Error502BadGateway("upstream error")
	}
	slog.ErrorContext(ctx, "request failed", slog.String("op", op), slog.String("error", err.Error()))
	return huma.Error500InternalServerError("internal error")
}
