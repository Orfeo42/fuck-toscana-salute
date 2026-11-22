package httpapi

import (
	"context"

	"fuck-toscana-salute/internal/drugsview"
	"fuck-toscana-salute/internal/facilities"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reportsview"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/visitsview"
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

//go:generate moq -out data_mock.go . Documents:MockDocuments Reports:MockReports
type Documents interface {
	PrescriptionPDF(ctx context.Context, documentRef string) ([]byte, error)
	Promemoria(ctx context.Context, bookingID string) ([]byte, error)
}

type Reports interface {
	Documents(ctx context.Context, ref referti.Ref) ([]referti.Document, error)
	Download(ctx context.Context, ref referti.Ref, position string) ([]byte, error)
}

//go:generate moq -out google_mock.go . Google:MockGoogle
type Google interface {
	Status(ctx context.Context) (google.Status, error)
	ExchangeCode(ctx context.Context, code, origin string) error
	Settings(ctx context.Context) (google.Settings, error)
	SaveSettings(ctx context.Context, s google.Settings) error
	Disconnect(ctx context.Context) error
	AddEvent(ctx context.Context, ev google.Event, pdf []byte) (google.EventResult, error)
	LookupEvent(ctx context.Context, key string) (google.EventLookup, error)
	PickerConfig(ctx context.Context) (google.PickerConfig, error)
	DownloadFile(ctx context.Context, fileID string) (google.DriveFile, error)
}

//go:generate moq -out local_mock.go . LocalRecords:MockLocalRecords
type LocalRecords interface {
	Create(ctx context.Context, nr localrecords.NewReport) (localrecords.Report, error)
	Update(ctx context.Context, id string, meta localrecords.Meta) error
	Delete(ctx context.Context, id string) error
	Link(ctx context.Context, nre string, ref referti.Ref) error
	Unlink(ctx context.Context, nre string, ref referti.Ref) error
	Links(ctx context.Context) ([]localrecords.Link, error)
	SetStatus(ctx context.Context, nre string, status int) error
	ResetStatus(ctx context.Context, nre string) error
	Overrides(ctx context.Context) ([]localrecords.Override, error)
}

//go:generate moq -out facilities_mock.go . Facilities:MockFacilities
type Facilities interface {
	Search(ctx context.Context, query string) ([]facilities.Facility, error)
}

//go:generate moq -out views_mock.go . VisitsView:MockVisitsView ReportsView:MockReportsView DrugsView:MockDrugsView Refresher:MockRefresher
type VisitsView interface {
	Query(ctx context.Context, q visitsview.Query) (visitsview.Page, error)
	Row(ctx context.Context, nre string) (visitsview.RowResult, error)
	ReportOptions(ctx context.Context, nre string) ([]visitsview.ReportOption, error)
}

type ReportsView interface {
	Query(ctx context.Context, q reportsview.Query) (reportsview.Page, error)
	Row(ctx context.Context, ref referti.Ref) (reportsview.RowResult, error)
	VisitOptions(ctx context.Context, ref referti.Ref) ([]reportsview.LinkedVisit, error)
}

type DrugsView interface {
	Query(ctx context.Context, q drugsview.Query) (drugsview.Page, error)
	Row(ctx context.Context, nre string) (drugsview.RowResult, error)
}

type Refresher interface {
	Start(scope refresh.Scope) error
}
