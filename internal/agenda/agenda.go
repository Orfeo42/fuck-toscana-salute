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
	ErrAppointments     = agendaError("agenda: appointments lookup failed")
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
	Appointment  cup.Appointment
	Prescription cup.Prescription
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

func (a *Agenda) Appointments(ctx context.Context) ([]Entry, error) {
	prescriptions, err := a.cup.Prescriptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCUPPrescriptions, err)
	}
	candidates := a.candidates(prescriptions)
	results, err := a.lookupAppointments(ctx, candidates)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAppointments, err)
	}
	entries := []Entry{}
	for i, appointments := range results {
		for _, appointment := range appointments {
			entries = append(entries, Entry{Appointment: appointment, Prescription: candidates[i]})
		}
	}
	slices.SortStableFunc(entries, compareEntries)
	return entries, nil
}

func (a *Agenda) candidates(prescriptions []cup.Prescription) []cup.Prescription {
	limit := a.now().Add(-maxPrescribedAge)
	candidates := []cup.Prescription{}
	for _, p := range prescriptions {
		switch p.Status {
		case cup.StatusCancelled, cup.StatusDone, cup.StatusDoneAfterCancel:
			continue
		}
		if !p.PrescribedOn.IsZero() && p.PrescribedOn.Before(limit) {
			continue
		}
		candidates = append(candidates, p)
	}
	return candidates
}

func (a *Agenda) lookupAppointments(ctx context.Context, candidates []cup.Prescription) ([][]cup.Appointment, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]cup.Appointment, len(candidates))
	semaphore := make(chan struct{}, a.concurrency)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for i := range candidates {
		semaphore <- struct{}{}
		wg.Go(func() {
			defer func() { <-semaphore }()
			appointments, err := a.cup.Appointments(ctx, candidates[i].NRE)
			if err != nil {
				mu.Lock()
				defer mu.Unlock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			results[i] = appointments
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
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
	cupPrescriptions, cupErr := a.cup.Prescriptions(ctx)
	fsePrescriptions, fseErr := a.fse.Prescriptions(ctx)
	cupLogin := errors.Is(cupErr, session.ErrNotLoggedIn) || errors.Is(cupErr, session.ErrSessionExpired)
	fseLogin := errors.Is(fseErr, session.ErrNotLoggedIn) || errors.Is(fseErr, session.ErrSessionExpired)
	if cupLogin && fseLogin {
		return nil, fmt.Errorf("%w: %w", ErrCUPPrescriptions, cupErr)
	}
	if cupErr != nil && !cupLogin {
		return nil, fmt.Errorf("%w: %w", ErrCUPPrescriptions, cupErr)
	}
	if fseErr != nil && !fseLogin {
		return nil, fmt.Errorf("%w: %w", ErrFSEPrescriptions, fseErr)
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
