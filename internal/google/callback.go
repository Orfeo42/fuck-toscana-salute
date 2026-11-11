package google

import (
	"errors"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	CallbackPath = "/api/google/callback"

	successPage = `<!doctype html><html lang="it"><head><meta charset="utf-8"><title>Google</title></head><body><p>Google collegato. Puoi chiudere questa scheda.</p></body></html>`
	failurePage = `<!doctype html><html lang="it"><head><meta charset="utf-8"><title>Google</title></head><body><p>Collegamento non riuscito. Riprova dall'app.</p></body></html>`
	callbackCSP = "default-src 'none'"
)

func (c *Client) CallbackHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", callbackCSP)
		w.Header().Set("Cache-Control", "no-store")

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		if query.Get("error") != "" {
			slog.WarnContext(r.Context(), "google authorization denied")
			writePage(w, r, http.StatusBadRequest, failurePage)
			return
		}

		err := c.Exchange(r.Context(), query.Get("state"), query.Get("code"))
		if err != nil {
			var retrieveErr *oauth2.RetrieveError
			if errors.Is(err, ErrBadState) || errors.Is(err, ErrNotConfigured) || errors.As(err, &retrieveErr) {
				slog.WarnContext(r.Context(), "google callback rejected", slog.String("error", err.Error()))
			} else {
				slog.ErrorContext(r.Context(), "google callback failed", slog.String("error", err.Error()))
			}
			writePage(w, r, http.StatusBadRequest, failurePage)
			return
		}

		slog.InfoContext(r.Context(), "google connected")
		writePage(w, r, http.StatusOK, successPage)
	})
}

func writePage(w http.ResponseWriter, r *http.Request, status int, page string) {
	w.WriteHeader(status)
	if _, err := w.Write([]byte(page)); err != nil {
		slog.WarnContext(r.Context(), "google callback write failed", slog.String("error", err.Error()))
	}
}
