package visitsview

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/notimpl"
)

type Service struct {
	ag    Agenda
	rep   Reports
	local Local
	ref   Refresher
	now   func() time.Time
}

func New(ag Agenda, rep Reports, local Local, ref Refresher) *Service {
	return &Service{ag: ag, rep: rep, local: local, ref: ref, now: time.Now}
}

func (s *Service) Query(_ context.Context, _ Query) (Page, error) {
	return Page{}, notimpl.ErrNotImplemented
}

func (s *Service) Row(_ context.Context, _ string) (RowResult, error) {
	return RowResult{}, notimpl.ErrNotImplemented
}

func (s *Service) ReportOptions(_ context.Context, _ string) ([]ReportOption, error) {
	return nil, notimpl.ErrNotImplemented
}
