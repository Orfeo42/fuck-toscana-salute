package drugsview

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/notimpl"
)

type Service struct {
	ag    Agenda
	local Local
	ref   Refresher
	now   func() time.Time
}

func New(ag Agenda, local Local, ref Refresher) *Service {
	return &Service{ag: ag, local: local, ref: ref, now: time.Now}
}

func (s *Service) Query(_ context.Context, _ Query) (Page, error) {
	return Page{}, notimpl.ErrNotImplemented
}

func (s *Service) Row(_ context.Context, _ string) (RowResult, error) {
	return RowResult{}, notimpl.ErrNotImplemented
}
