package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/refresh"
)

type startRefreshInput struct {
	Body StartRefreshRequest
}

type StartRefreshRequest struct {
	All     bool     `json:"all,omitempty"`
	Lists   bool     `json:"lists,omitempty"`
	Reports bool     `json:"reports,omitempty"`
	NREs    []string `json:"nres,omitempty" maxItems:"200" pattern:"^[0-9A-Za-z]{10,20}$" nullable:"false"`
}

func (r *StartRefreshRequest) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if r.All || r.Lists || r.Reports || len(r.NREs) > 0 {
		return nil
	}
	return []error{&huma.ErrorDetail{Message: "all, lists, reports or nres is required", Location: prefix.With("all"), Value: r.All}}
}

type startRefreshOutput struct {
	Body StartRefreshBody
}

type StartRefreshBody struct {
	Refreshing bool `json:"refreshing"`
}

func registerRefresh(api huma.API, refresher Refresher) {
	huma.Register(api, huma.Operation{
		OperationID:   "start-refresh",
		Method:        http.MethodPost,
		Path:          "/api/refresh/start",
		DefaultStatus: http.StatusAccepted,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *startRefreshInput) (*startRefreshOutput, error) {
		scope := refresh.Scope{All: in.Body.All, Lists: in.Body.Lists, Reports: in.Body.Reports, NREs: in.Body.NREs}
		if err := refresher.Start(scope); err != nil {
			return nil, toHTTPError(ctx, "start-refresh", err)
		}
		slog.InfoContext(ctx, "refresh started",
			slog.Bool("all", scope.All),
			slog.Bool("lists", scope.Lists),
			slog.Bool("reports", scope.Reports),
			slog.Int("nres", len(scope.NREs)))
		return &startRefreshOutput{Body: StartRefreshBody{Refreshing: true}}, nil
	})
}
