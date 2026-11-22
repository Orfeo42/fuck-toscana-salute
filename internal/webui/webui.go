package webui

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed all:assets
var assets embed.FS

const (
	notBuiltMessage = `web UI not built: run "just build"`
	csp             = "default-src 'self'; img-src 'self' data:; script-src 'self' https://apis.google.com https://accounts.google.com/gsi/client; style-src 'self' 'unsafe-inline' https://accounts.google.com/gsi/style; frame-src 'self' https://docs.google.com https://accounts.google.com/gsi/; connect-src 'self' blob: https://accounts.google.com/gsi/; object-src 'self'"
)

func Handler() http.Handler {
	sub, err := fs.Sub(assets, "assets/dist")
	if err != nil {
		slog.Warn("web UI assets unavailable", slog.String("error", err.Error()))
		return handler(nil)
	}
	return handler(sub)
}

func handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", csp)
		if fsys == nil {
			http.Error(w, notBuiltMessage, http.StatusServiceUnavailable)
			return
		}
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			http.Error(w, notBuiltMessage, http.StatusServiceUnavailable)
			return
		}
		files.ServeHTTP(w, r)
	})
}
