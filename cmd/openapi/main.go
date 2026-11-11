package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"fuck-toscana-salute/internal/httpapi"
)

type openapiError string

func (e openapiError) Error() string { return string(e) }

const ErrSpecUnavailable openapiError = "openapi spec unavailable"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(os.Stdout); err != nil {
		slog.Error("openapi failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(w io.Writer) error {
	handler := httpapi.New(nil, nil, nil, nil, nil, nil, nil, "window", false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrSpecUnavailable, rec.Code)
	}
	if _, err := w.Write(rec.Body.Bytes()); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}
