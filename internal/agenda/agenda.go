package agenda

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/session"
)

type agendaError string

func (e agendaError) Error() string { return string(e) }

const (
	ErrCUPPrescriptions = agendaError("agenda: cup prescriptions failed")
	ErrFSEPrescriptions = agendaError("agenda: fse prescriptions failed")
)

const (
	bookedStatus     = "PRENOTATO"
	maxPrescribedAge = 365 * 24 * time.Hour
)

//go:generate moq -out deps_mock.go . CUP:MockCUP FSE:MockFSE
type CUP interface {
	Prescriptions(ctx context.Context) ([]cup.Prescription, error)
	Appointments(ctx context.Context, nre string) ([]cup.Appointment, error)
}

type FSE interface {
	Prescriptions(ctx context.Context) ([]fse.Prescription, error)
}

type Entry struct {
	Appointment cup.Appointment
	NRE         string
	CUP         *cup.Prescription
	FSE         *fse.Prescription
}

type Failure struct {
	NRE string
	CUP *cup.Prescription
	FSE *fse.Prescription
	Err error
}

type candidate struct {
	nre string
	cup *cup.Prescription
	fse *fse.Prescription
}

type Result struct {
	Entries []Entry
	Failed  []Failure
}

type Row struct {
	NRE string
	CUP *cup.Prescription
	FSE *fse.Prescription
}

type Agenda struct {
	cup         CUP
	fse         FSE
	concurrency int
	now         func() time.Time
}

func New(cupClient CUP, fseClient FSE, concurrency int) *Agenda {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Agenda{cup: cupClient, fse: fseClient, concurrency: concurrency, now: time.Now}
}

func (a *Agenda) Appointments(ctx context.Context) (Result, error) {
	cupPrescriptions, fsePrescriptions, err := a.sources(ctx)
	if err != nil {
		return Result{}, err
	}
	candidates := a.candidates(cupPrescriptions, fsePrescriptions)
	results, errs := a.lookupAppointments(ctx, candidates)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	result := Result{Entries: []Entry{}, Failed: []Failure{}}
	for i, c := range candidates {
		if errs[i] != nil {
			result.Failed = append(result.Failed, Failure{NRE: c.nre, CUP: c.cup, FSE: c.fse, Err: errs[i]})
			continue
		}
		for _, appointment := range results[i] {
			result.Entries = append(result.Entries, Entry{Appointment: appointment, NRE: c.nre, CUP: c.cup, FSE: c.fse})
		}
	}
	slices.SortStableFunc(result.Entries, compareEntries)
	return result, nil
}

func (a *Agenda) sources(ctx context.Context) ([]cup.Prescription, []fse.Prescription, error) {
	cupPrescriptions, cupErr := a.cup.Prescriptions(ctx)
	fsePrescriptions, fseErr := a.fse.Prescriptions(ctx)
	cupLogin := errors.Is(cupErr, session.ErrNotLoggedIn) || errors.Is(cupErr, session.ErrSessionExpired)
	fseLogin := errors.Is(fseErr, session.ErrNotLoggedIn) || errors.Is(fseErr, session.ErrSessionExpired)
	if cupLogin && fseLogin {
		return nil, nil, fmt.Errorf("%w: %w", ErrCUPPrescriptions, cupErr)
	}
	if cupErr != nil && !cupLogin {
		return nil, nil, fmt.Errorf("%w: %w", ErrCUPPrescriptions, cupErr)
	}
	if fseErr != nil && !fseLogin {
		return nil, nil, fmt.Errorf("%w: %w", ErrFSEPrescriptions, fseErr)
	}
	return cupPrescriptions, fsePrescriptions, nil
}

func (a *Agenda) candidates(cupPrescriptions []cup.Prescription, fsePrescriptions []fse.Prescription) []candidate {
	limit := a.now().Add(-maxPrescribedAge)
	candidates := []candidate{}
	index := map[string]int{}
	for i := range cupPrescriptions {
		p := &cupPrescriptions[i]
		if p.NRE == "" || isClosed(int(p.Status)) || (!p.PrescribedOn.IsZero() && p.PrescribedOn.Before(limit)) {
			continue
		}
		key := strings.ToUpper(p.NRE)
		if at, ok := index[key]; ok {
			candidates[at].cup = p
			continue
		}
		index[key] = len(candidates)
		candidates = append(candidates, candidate{nre: p.NRE, cup: p})
	}
	for i := range fsePrescriptions {
		p := &fsePrescriptions[i]
		if p.Kind != fse.KindVisit || p.NRE == "" || isClosed(p.Status) || (!p.Date.IsZero() && p.Date.Before(limit)) {
			continue
		}
		key := strings.ToUpper(p.NRE)
		if at, ok := index[key]; ok {
			candidates[at].fse = p
			continue
		}
		index[key] = len(candidates)
		candidates = append(candidates, candidate{nre: p.NRE, fse: p})
	}
	return candidates
}

func isClosed(status int) bool {
	switch cup.Status(status) {
	case cup.StatusCancelled, cup.StatusDone, cup.StatusDoneAfterCancel:
		return true
	}
	return false
}

func (a *Agenda) lookupAppointments(ctx context.Context, candidates []candidate) ([][]cup.Appointment, []error) {
	results := make([][]cup.Appointment, len(candidates))
	errs := make([]error, len(candidates))
	semaphore := make(chan struct{}, a.concurrency)
	var wg sync.WaitGroup
	for i := range candidates {
		semaphore <- struct{}{}
		wg.Go(func() {
			defer func() { <-semaphore }()
			results[i], errs[i] = a.cup.Appointments(ctx, candidates[i].nre)
		})
	}
	wg.Wait()
	return results, errs
}

func compareEntries(a, b Entry) int {
	aBooked := a.Appointment.Status == bookedStatus
	bBooked := b.Appointment.Status == bookedStatus
	if aBooked != bBooked {
		if aBooked {
			return -1
		}
		return 1
	}
	return compareDatesAscZeroLast(a.Appointment.At, b.Appointment.At)
}

func compareDatesAscZeroLast(a, b time.Time) int {
	if a.IsZero() || b.IsZero() {
		switch {
		case a.IsZero() && b.IsZero():
			return 0
		case a.IsZero():
			return 1
		default:
			return -1
		}
	}
	return a.Compare(b)
}

func (a *Agenda) Prescriptions(ctx context.Context) ([]Row, error) {
	cupPrescriptions, fsePrescriptions, err := a.sources(ctx)
	if err != nil {
		return nil, err
	}

	rows := []Row{}
	index := map[string]int{}
	for i := range cupPrescriptions {
		key := strings.ToUpper(cupPrescriptions[i].NRE)
		if at, ok := index[key]; ok {
			rows[at].CUP = &cupPrescriptions[i]
			continue
		}
		index[key] = len(rows)
		rows = append(rows, Row{NRE: cupPrescriptions[i].NRE, CUP: &cupPrescriptions[i]})
	}
	for i := range fsePrescriptions {
		key := "fse:" + fsePrescriptions[i].ID
		if fsePrescriptions[i].NRE != "" {
			key = strings.ToUpper(fsePrescriptions[i].NRE)
		}
		if at, ok := index[key]; ok {
			rows[at].FSE = &fsePrescriptions[i]
			continue
		}
		index[key] = len(rows)
		rows = append(rows, Row{NRE: fsePrescriptions[i].NRE, FSE: &fsePrescriptions[i]})
	}
	slices.SortStableFunc(rows, compareRows)
	return rows, nil
}

func compareRows(a, b Row) int {
	aDate := rowDate(a)
	bDate := rowDate(b)
	if aDate.IsZero() || bDate.IsZero() {
		return compareDatesAscZeroLast(aDate, bDate)
	}
	return bDate.Compare(aDate)
}

func rowDate(r Row) time.Time {
	var date time.Time
	if r.CUP != nil {
		date = r.CUP.PrescribedOn
	}
	if r.FSE != nil && r.FSE.Date.After(date) {
		date = r.FSE.Date
	}
	return date
}
