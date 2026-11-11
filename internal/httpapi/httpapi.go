package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

type sessionOutput struct {
	Body SessionBody
}

type startLoginInput struct {
	Portal string `path:"portal" enum:"cup,fse"`
}

type emptyOutput struct{}

func New(store SessionStore, flow LoginFlow) http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("fuck-toscana-salute", "0.1.0"))
	register(api, store, flow)
	return mux
}

func register(api huma.API, store SessionStore, flow LoginFlow) {
	huma.Register(api, huma.Operation{
		OperationID:   "get-session",
		Method:        http.MethodGet,
		Path:          "/api/session",
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, _ *struct{}) (*sessionOutput, error) {
		return &sessionOutput{Body: toSessionBody(store.Status(), flow.States())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "start-login",
		Method:        http.MethodPost,
		Path:          "/api/login/{portal}",
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, in *startLoginInput) (*emptyOutput, error) {
		err := flow.Start(ctx, session.Portal(in.Portal))
		if errors.Is(err, loginflow.ErrLoginInProgress) {
			return nil, huma.Error409Conflict("login already in progress")
		}
		if err != nil {
			slog.ErrorContext(ctx, "start login failed", slog.String("portal", in.Portal), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("cannot start login")
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/logout",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, _ *struct{}) (*emptyOutput, error) {
		store.Clear()
		return &emptyOutput{}, nil
	})
}
