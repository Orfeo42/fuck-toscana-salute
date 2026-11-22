package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"fuck-toscana-salute/internal/jwtclaims"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

type sessionOutput struct {
	Body SessionBody
}

type startLoginInput struct {
	Portal string `path:"portal" enum:"cup,fse"`
}

type importLoginInput struct {
	Body ImportLoginRequest
}

type ImportLoginRequest struct {
	Portal  string `json:"portal" required:"true" enum:"cup,fse"`
	Access  string `json:"access" required:"true" minLength:"1"`
	Refresh string `json:"refresh" required:"true" minLength:"1"`
	CF      string `json:"cf" required:"true" minLength:"1"`
	Tessera string `json:"tessera,omitempty"`
}

type emptyOutput struct{}

type prescriptionPDFInput struct {
	Ref string `query:"ref" required:"true" minLength:"1" maxLength:"200"`
}

type prescriptionPDFOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

type bookingPDFInput struct {
	ID string `query:"id" required:"true" minLength:"1" maxLength:"32" pattern:"^[A-Za-z0-9]+$"`
}

func New(store SessionStore, flow LoginFlow, docs Documents, g Google, reports Reports, local LocalRecords, fac Facilities, visits VisitsView, reportsView ReportsView, drugs DrugsView, refresher Refresher, loginMode string, viewer bool) http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("fuck-toscana-salute", "0.1.0"))
	register(api, store, flow, docs, g, reports, local, loginMode, viewer)
	registerFacilities(api, fac)
	registerVisits(api, visits)
	registerReportsQuery(api, reportsView)
	registerDrugs(api, drugs)
	registerRefresh(api, refresher)
	return mux
}

func sameOrigin(api huma.API) huma.Middlewares {
	return huma.Middlewares{func(ctx huma.Context, next func(huma.Context)) {
		origin, err := url.Parse(ctx.Header("Origin"))
		if err != nil || origin.Host == "" || origin.Host != ctx.Host() {
			slog.WarnContext(ctx.Context(), "request rejected: origin mismatch", slog.String("op", ctx.Operation().OperationID))
			if err := huma.WriteErr(api, ctx, http.StatusForbidden, "origin non consentito"); err != nil {
				slog.ErrorContext(ctx.Context(), "write forbidden response failed", slog.String("error", err.Error()))
			}
			return
		}
		next(ctx)
	}}
}

func register(api huma.API, store SessionStore, flow LoginFlow, docs Documents, g Google, reports Reports, local LocalRecords, loginMode string, viewer bool) {
	registerGoogle(api, g, docs)
	registerReports(api, reports)
	registerLocal(api, local, g)

	huma.Register(api, huma.Operation{
		OperationID:   "get-session",
		Method:        http.MethodGet,
		Path:          "/api/session",
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, _ *struct{}) (*sessionOutput, error) {
		return &sessionOutput{Body: toSessionBody(store.Status(), flow.States(), loginMode, viewer)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "import-login",
		Method:        http.MethodPost,
		Path:          "/api/login/import",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *importLoginInput) (*emptyOutput, error) {
		portal := session.Portal(in.Body.Portal)
		res, err := login.ResultFromStorage(portal, in.Body.Access, in.Body.Refresh, in.Body.CF)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Token non valido")
		}
		if portal == session.PortalCUP && in.Body.Tessera != "" {
			res.Identity.TesseraSanitaria = in.Body.Tessera
			res.Tokens.GestioneVerified = true
		}

		err = flow.Import(ctx, res)
		if errors.Is(err, login.ErrTokenMismatch) {
			return nil, huma.Error422UnprocessableEntity("I token non appartengono a questo portale")
		}
		if errors.Is(err, login.ErrTokenExpired) {
			return nil, huma.Error422UnprocessableEntity("Il token è scaduto: accedi di nuovo al portale e riprova")
		}
		if errors.Is(err, jwtclaims.ErrMalformedToken) {
			return nil, huma.Error422UnprocessableEntity("Token non valido")
		}
		if err != nil {
			slog.ErrorContext(ctx, "import login failed", slog.String("portal", in.Body.Portal), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("cannot import login")
		}
		return &emptyOutput{}, nil
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
		OperationID:   "start-login-all",
		Method:        http.MethodPost,
		Path:          "/api/login",
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		err := flow.StartAll(ctx)
		if errors.Is(err, loginflow.ErrLoginInProgress) {
			return nil, huma.Error409Conflict("login already in progress")
		}
		if err != nil {
			slog.ErrorContext(ctx, "start login failed", slog.String("portal", "all"), slog.String("error", err.Error()))
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

	huma.Register(api, huma.Operation{
		OperationID:   "get-prescription-pdf",
		Method:        http.MethodGet,
		Path:          "/api/documents/prescription",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *prescriptionPDFInput) (*prescriptionPDFOutput, error) {
		pdf, err := docs.PrescriptionPDF(ctx, in.Ref)
		if err != nil {
			return nil, toHTTPError(ctx, "get-prescription-pdf", err)
		}
		return &prescriptionPDFOutput{
			ContentType:        "application/pdf",
			ContentDisposition: `inline; filename="ricetta.pdf"`,
			Body:               pdf,
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-booking-pdf",
		Method:        http.MethodGet,
		Path:          "/api/documents/booking",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *bookingPDFInput) (*prescriptionPDFOutput, error) {
		pdf, err := docs.Promemoria(ctx, in.ID)
		if err != nil {
			return nil, toHTTPError(ctx, "get-booking-pdf", err)
		}
		return &prescriptionPDFOutput{
			ContentType:        "application/pdf",
			ContentDisposition: `inline; filename="promemoria.pdf"`,
			Body:               pdf,
		}, nil
	})
}
