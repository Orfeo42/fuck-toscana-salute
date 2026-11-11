package viewer

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const prefix = "/viewer"

func Handler(addr string) http.Handler {
	target := &url.URL{Scheme: "http", Host: addr}
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.URL.Path = strings.TrimPrefix(r.Out.URL.Path, prefix)
			r.Out.URL.RawPath = ""
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.WarnContext(r.Context(), "viewer upstream failed", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusBadGateway)
		},
	}
}
