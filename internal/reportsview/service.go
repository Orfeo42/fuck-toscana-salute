package reportsview

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/notimpl"
	"fuck-toscana-salute/internal/referti"
)

type Service struct {
	rep Reports
	ag  Agenda
	ref Refresher
	now func() time.Time
}

func New(rep Reports, ag Agenda, ref Refresher) *Service {
	return &Service{rep: rep, ag: ag, ref: ref, now: time.Now}
}

func (s *Service) Query(_ context.Context, _ Query) (Page, error) {
	return Page{}, notimpl.ErrNotImplemented
}

func (s *Service) Row(_ context.Context, _ referti.Ref) (RowResult, error) {
	return RowResult{}, notimpl.ErrNotImplemented
}

func (s *Service) VisitOptions(_ context.Context, _ referti.Ref) ([]LinkedVisit, error) {
	return nil, notimpl.ErrNotImplemented
}
