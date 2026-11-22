package agenda

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/session"
)

type agendaError string

func (e agendaError) Error() string { return string(e) }

const (
	ErrCUPPrescriptions = agendaError("agenda: cup prescriptions failed")
	ErrFSEPrescriptions = agendaError("agenda: fse prescriptions failed")
	ErrAppointments     = agendaError("agenda: cached appointments failed")
	ErrVerdicts         = agendaError("agenda: cached verdicts failed")
	ErrRefreshNREs      = agendaError("agenda: refresh of some NREs failed")
)

const (
	bookedStatus     = "PRENOTATO"
	cancelledStatus  = "DISDETTO"
	defaultTitle     = "Prescrizione"
	maxPrescribedAge = 365 * 24 * time.Hour
)

//go:generate moq -out deps_mock.go . CUP:MockCUP FSE:MockFSE
type CUP interface {
	Prescriptions(ctx context.Context) ([]cup.Prescription, error)
	Appointments(ctx context.Context, nre string) ([]cup.Appointment, error)
	Verdict(ctx context.Context, nre string) (cup.Verdict, error)
	CachedPrescriptions(ctx context.Context) (cache.Entry[[]cup.Prescription], error)
	CachedAppointments(ctx context.Context) (map[string]cache.Entry[[]cup.Appointment], error)
	CachedVerdicts(ctx context.Context) (map[string]cache.Entry[cup.Verdict], error)
}

type FSE interface {
	Prescriptions(ctx context.Context) ([]fse.Prescription, error)
	CachedPrescriptions(ctx context.Context) (cache.Entry[[]fse.Prescription], error)
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
	fse *fse.Prescription
}

type Result struct {
	Entries  []Entry
	Failed   []Failure
	Unbooked []string
	Verdicts map[string]cup.Verdict
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
	result := Result{Entries: []Entry{}, Failed: []Failure{}, Unbooked: []string{}, Verdicts: map[string]cup.Verdict{}}
	for i, c := range candidates {
		if errs[i] != nil {
			result.Failed = append(result.Failed, Failure{NRE: c.nre, FSE: c.fse, Err: errs[i]})
			continue
		}
		if len(results[i]) == 0 {
			result.Unbooked = append(result.Unbooked, c.nre)
		}
		for _, appointment := range results[i] {
			result.Entries = append(result.Entries, Entry{Appointment: appointment, NRE: c.nre, FSE: c.fse})
		}
	}
	slices.SortStableFunc(result.Entries, compareEntries)
	result.Verdicts = a.lookupVerdicts(ctx, result.Unbooked)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (a *Agenda) lookupVerdicts(ctx context.Context, nres []string) map[string]cup.Verdict {
	verdicts := make([]cup.Verdict, len(nres))
	errs := make([]error, len(nres))
	semaphore := make(chan struct{}, a.concurrency)
	var wg sync.WaitGroup
	for i := range nres {
		semaphore <- struct{}{}
		wg.Go(func() {
			defer func() { <-semaphore }()
			verdicts[i], errs[i] = a.cup.Verdict(ctx, nres[i])
		})
	}
	wg.Wait()

	out := make(map[string]cup.Verdict, len(nres))
	failed := 0
	for i, nre := range nres {
		if errs[i] != nil {
			failed++
			continue
		}
		out[strings.ToUpper(nre)] = verdicts[i]
	}
	if failed > 0 {
		slog.WarnContext(ctx, "verdict lookups failed", slog.Int("failed", failed), slog.Int("total", len(nres)))
	}
	return out
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
	unbooked := make(map[string]struct{}, len(cupPrescriptions))
	for i := range cupPrescriptions {
		unbooked[strings.ToUpper(cupPrescriptions[i].NRE)] = struct{}{}
	}
	candidates := []candidate{}
	index := map[string]int{}
	for i := range fsePrescriptions {
		p := &fsePrescriptions[i]
		if p.Kind != fse.KindVisit || p.NRE == "" || isClosed(p.Status) || (!p.Date.IsZero() && p.Date.Before(limit)) {
			continue
		}
		key := strings.ToUpper(p.NRE)
		if _, ok := unbooked[key]; ok {
			continue
		}
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
	return mergeRows(cupPrescriptions, fsePrescriptions), nil
}

func mergeRows(cupPrescriptions []cup.Prescription, fsePrescriptions []fse.Prescription) []Row {
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
	return rows
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

type VerdictEntry struct {
	Verdict    cup.Verdict
	VerifiedAt time.Time
	Stale      bool
}

type Lookup struct {
	Candidate    bool
	Checked      bool
	CheckedAt    time.Time
	Stale        bool
	Appointments []cup.Appointment
	Verdict      *VerdictEntry
}

type ListState struct {
	At      time.Time
	Stale   bool
	Missing bool
}

type Snapshot struct {
	Rows    []Row
	CUPList ListState
	FSEList ListState
	Lookups map[string]Lookup
}

func listState(at time.Time, stale bool, err error) (ListState, error) {
	if errors.Is(err, cache.ErrNotCached) {
		return ListState{Missing: true}, nil
	}
	if err != nil {
		return ListState{}, err
	}
	return ListState{At: at, Stale: stale}, nil
}

func (a *Agenda) Snapshot(ctx context.Context) (Snapshot, error) {
	cupList, err := a.cup.CachedPrescriptions(ctx)
	cupState, err := listState(cupList.FetchedAt, cupList.Stale, err)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrCUPPrescriptions, err)
	}
	fseList, err := a.fse.CachedPrescriptions(ctx)
	fseState, err := listState(fseList.FetchedAt, fseList.Stale, err)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrFSEPrescriptions, err)
	}
	appointments, err := a.cup.CachedAppointments(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrAppointments, err)
	}
	verdicts, err := a.cup.CachedVerdicts(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrVerdicts, err)
	}

	snapshot := Snapshot{
		Rows:    mergeRows(cupList.Value, fseList.Value),
		CUPList: cupState,
		FSEList: fseState,
		Lookups: map[string]Lookup{},
	}
	for _, c := range a.candidates(cupList.Value, fseList.Value) {
		key := strings.ToUpper(c.nre)
		snapshot.Lookups[key] = buildLookup(appointments[key], verdicts[key])
	}
	return snapshot, nil
}

func buildLookup(appointments cache.Entry[[]cup.Appointment], verdict cache.Entry[cup.Verdict]) Lookup {
	if appointments.FetchedAt.IsZero() {
		return Lookup{Candidate: true}
	}
	lookup := Lookup{
		Candidate:    true,
		Checked:      true,
		CheckedAt:    appointments.FetchedAt,
		Stale:        appointments.Stale,
		Appointments: appointments.Value,
	}
	if len(appointments.Value) == 0 && !verdict.FetchedAt.IsZero() {
		lookup.Verdict = &VerdictEntry{Verdict: verdict.Value, VerifiedAt: verdict.FetchedAt, Stale: verdict.Stale}
	}
	return lookup
}

func (a *Agenda) Lists(ctx context.Context) error {
	_, _, err := a.sources(ctx)
	return err
}

func (a *Agenda) RefreshNREs(ctx context.Context, nres []string) error {
	ctx = cache.WithRefresh(ctx, cache.Scope{NREs: nres})
	candidates := make([]candidate, 0, len(nres))
	for _, nre := range nres {
		candidates = append(candidates, candidate{nre: nre})
	}
	results, errs := a.lookupAppointments(ctx, candidates)
	if err := ctx.Err(); err != nil {
		return err
	}
	unbooked := []string{}
	for i, c := range candidates {
		if errs[i] == nil && len(results[i]) == 0 {
			unbooked = append(unbooked, c.nre)
		}
	}
	a.lookupVerdicts(ctx, unbooked)
	if err := ctx.Err(); err != nil {
		return err
	}
	failed := errors.Join(errs...)
	if failed != nil {
		return fmt.Errorf("%w: %w", ErrRefreshNREs, failed)
	}
	return nil
}

func isLive(appointment cup.Appointment) bool {
	return appointment.Status != cancelledStatus
}

func BestBooking(appointments []cup.Appointment) (cup.Appointment, bool) {
	if len(appointments) == 0 {
		return cup.Appointment{}, false
	}
	best := appointments[0]
	for _, candidate := range appointments[1:] {
		if isLive(candidate) != isLive(best) {
			if isLive(candidate) {
				best = candidate
			}
			continue
		}
		if candidate.At.After(best.At) {
			best = candidate
		}
	}
	return best, true
}

func (r Row) Title() string {
	if r.FSE != nil && r.FSE.Description != "" {
		return r.FSE.Description
	}
	if r.CUP != nil && len(r.CUP.Items) > 0 {
		return strings.Join(r.CUP.Items, ", ")
	}
	return defaultTitle
}

func (r Row) PrescribedOn() time.Time {
	if r.FSE != nil && !r.FSE.Date.IsZero() {
		return r.FSE.Date
	}
	if r.CUP != nil {
		return r.CUP.PrescribedOn
	}
	return time.Time{}
}

func (r Row) IsVisit() bool {
	if r.FSE != nil {
		return r.FSE.Kind == fse.KindVisit
	}
	return r.CUP != nil
}
