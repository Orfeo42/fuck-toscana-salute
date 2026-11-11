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
	"fuck-toscana-salute/internal/upstream"
)

type sessionOutput struct {
	Body SessionBody
}

type startLoginInput struct {
	Portal string `path:"portal" enum:"cup,fse"`
}

type emptyOutput struct{}

type appointmentsOutput struct {
	Body AppointmentsBody
}

type prescriptionsOutput struct {
	Body PrescriptionsBody
}

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

func New(store SessionStore, flow LoginFlow, ag Agenda, docs Documents) http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("fuck-toscana-salute", "0.1.0"))
	register(api, store, flow, ag, docs)
	return mux
}

func register(api huma.API, store SessionStore, flow LoginFlow, ag Agenda, docs Documents) {
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
		OperationID:   "list-appointments",
		Method:        http.MethodGet,
		Path:          "/api/appointments",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*appointmentsOutput, error) {
		result, err := ag.Appointments(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list-appointments", err)
		}
		if len(result.Failed) > 0 {
			first := result.Failed[0].Err
			var statusErr *upstream.StatusError
			exception := ""
			shape := ""
			if errors.As(first, &statusErr) {
				exception = statusErr.ExceptionClass()
				shape = statusErr.Shape()
			}
			slog.WarnContext(ctx, "appointment lookups failed",
				slog.Int("failed", len(result.Failed)),
				slog.Int("total", len(result.Entries)+len(result.Failed)),
				slog.String("first_error", first.Error()),
				slog.String("first_exception", exception),
				slog.String("first_shape", shape))
		}
		return &appointmentsOutput{Body: toAppointmentsBody(result)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "list-prescriptions",
		Method:        http.MethodGet,
		Path:          "/api/prescriptions",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*prescriptionsOutput, error) {
		rows, err := ag.Prescriptions(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list-prescriptions", err)
		}
		return &prescriptionsOutput{Body: toPrescriptionsBody(rows)}, nil
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
