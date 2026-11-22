package drugsview

import (
	"context"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/refresh"
)

//go:generate moq -out deps_mock.go . Agenda:MockAgenda Local:MockLocal Refresher:MockRefresher
type Agenda interface {
	Snapshot(ctx context.Context) (agenda.Snapshot, error)
}

type Local interface {
	Overrides(ctx context.Context) ([]localrecords.Override, error)
}

type Refresher interface {
	Kick(job refresh.Job)
	Status() refresh.Status
}
