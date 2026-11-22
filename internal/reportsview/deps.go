package reportsview

import (
	"context"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

//go:generate moq -out deps_mock.go . Reports:MockReports Agenda:MockAgenda Refresher:MockRefresher
type Reports interface {
	CachedList(ctx context.Context) (reports.CachedListing, error)
}

type Agenda interface {
	Snapshot(ctx context.Context) (agenda.Snapshot, error)
}

type Refresher interface {
	Kick(job refresh.Job)
	Status() refresh.Status
}
