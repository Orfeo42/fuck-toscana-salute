package httpapi

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
)

//go:generate moq -out deps_mock.go . SessionStore:MockSessionStore LoginFlow:MockLoginFlow
type SessionStore interface {
	Status() session.Status
	Clear()
}

type LoginFlow interface {
	Start(ctx context.Context, portal session.Portal) error
	StartAll(ctx context.Context) error
	Import(ctx context.Context, res login.Result) error
	States() map[session.Portal]loginflow.State
}

//go:generate moq -out data_mock.go . Agenda:MockAgenda Documents:MockDocuments Reports:MockReports Freshness:MockFreshness
type Agenda interface {
	Appointments(ctx context.Context) (agenda.Result, error)
	Prescriptions(ctx context.Context) ([]agenda.Row, error)
}

type Documents interface {
	PrescriptionPDF(ctx context.Context, documentRef string) ([]byte, error)
	Promemoria(ctx context.Context, bookingID string) ([]byte, error)
}

type Reports interface {
	List(ctx context.Context) (referti.Listing, error)
	Documents(ctx context.Context, ref referti.Ref) ([]referti.Document, error)
	Download(ctx context.Context, ref referti.Ref, position string) ([]byte, error)
}

type Freshness interface {
	FetchedAt(ctx context.Context, source string) (map[string]time.Time, error)
}

//go:generate moq -out google_mock.go . Google:MockGoogle
type Google interface {
	Status(ctx context.Context) (google.Status, error)
	AuthURL(ctx context.Context, redirectURL string) (string, error)
	Settings(ctx context.Context) (google.Settings, error)
	SaveSettings(ctx context.Context, s google.Settings) error
	Disconnect(ctx context.Context) error
	AddEvent(ctx context.Context, ev google.Event, pdf []byte) (google.EventResult, error)
}
