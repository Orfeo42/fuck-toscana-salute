package visitsview

import (
	"context"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

//go:generate moq -out deps_mock.go . Agenda:MockAgenda Reports:MockReports Local:MockLocal Refresher:MockRefresher
type Agenda interface {
	Snapshot(ctx context.Context) (agenda.Snapshot, error)
}

type Reports interface {
	CachedList(ctx context.Context) (reports.CachedListing, error)
}

type Local interface {
	Overrides(ctx context.Context) ([]localrecords.Override, error)
}

type Refresher interface {
	Kick(job refresh.Job)
	Status() refresh.Status
}
