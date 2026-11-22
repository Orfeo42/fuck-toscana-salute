package refresh

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/session"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/reports"
)

type Job string

const (
	JobVisits  Job = "visits"
	JobLists   Job = "lists"
	JobReports Job = "reports"
)

type Issue string

const (
	IssueNone             Issue = "none"
	IssueLoginRequired    Issue = "login_required"
	IssueGestioneRequired Issue = "gestione_required"
	IssueUnavailable      Issue = "unavailable"
)

type Scope struct {
	All     bool
	Lists   bool
	Reports bool
	NREs    []string
}

type Issues struct {
	Lists        Issue
	Appointments Issue
	Reports      Issue
}

type Status struct {
	Active bool
	Jobs   []Job
	NREs   []string
	Issues Issues
	Failed map[string]Issue
}

//go:generate moq -out deps_mock.go . Agenda:MockAgenda Reports:MockReports
type Agenda interface {
	Appointments(ctx context.Context) (agenda.Result, error)
	Prescriptions(ctx context.Context) ([]agenda.Row, error)
	RefreshNREs(ctx context.Context, nres []string) error
}

type Reports interface {
	List(ctx context.Context) (reports.Listing, error)
}

type refreshError string

func (e refreshError) Error() string { return string(e) }

const ErrEmptyScope = refreshError("refresh: empty scope")

const unavailableBackoff = 60 * time.Second

func Classify(err error) Issue {
	switch {
	case err == nil:
		return IssueNone
	case errors.Is(err, session.ErrSessionExpired), errors.Is(err, session.ErrNotLoggedIn):
		return IssueLoginRequired
	case errors.Is(err, cup.ErrGestioneNotReady):
		return IssueGestioneRequired
	}
	return IssueUnavailable
}

type task struct {
	job    Job
	forced bool
	scope  cache.Scope
}

type Refresher struct {
	ag  Agenda
	rep Reports
	now func() time.Time

	mu          sync.Mutex
	queue       []task
	running     *Job
	nreQueue    []string
	nreRunning  []string
	issues      Issues
	failed      map[string]Issue
	unavailable map[Job]time.Time
	bulkWake    chan struct{}
	nreWake     chan struct{}
}

func New(ag Agenda, rep Reports) *Refresher {
	return &Refresher{
		ag:          ag,
		rep:         rep,
		now:         time.Now,
		issues:      Issues{Lists: IssueNone, Appointments: IssueNone, Reports: IssueNone},
		failed:      map[string]Issue{},
		unavailable: map[Job]time.Time{},
		bulkWake:    make(chan struct{}, 1),
		nreWake:     make(chan struct{}, 1),
	}
}

func (r *Refresher) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r.bulkLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		r.nreLoop(ctx)
	}()
	wg.Wait()
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (r *Refresher) Kick(job Job) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running != nil && *r.running == job {
		return
	}
	if slices.ContainsFunc(r.queue, func(t task) bool { return t.job == job }) {
		return
	}
	if last, ok := r.unavailable[job]; ok && r.now().Sub(last) < unavailableBackoff {
		return
	}
	r.queue = append(r.queue, task{job: job})
	signal(r.bulkWake)
}

func (r *Refresher) Start(scope Scope) error {
	if !scope.All && !scope.Lists && !scope.Reports && len(scope.NREs) == 0 {
		return ErrEmptyScope
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if scope.All {
		forced := cache.Scope{All: true, Lists: true, Reports: true}
		r.enqueueForced(JobVisits, forced)
		r.enqueueForced(JobLists, forced)
		r.enqueueForced(JobReports, forced)
	}
	if scope.Lists && !scope.All {
		r.enqueueForced(JobLists, cache.Scope{Lists: true})
	}
	if scope.Reports && !scope.All {
		r.enqueueForced(JobReports, cache.Scope{Reports: true})
	}
	signal(r.bulkWake)

	if len(scope.NREs) > 0 {
		for _, nre := range scope.NREs {
			if !slices.Contains(r.nreQueue, nre) {
				r.nreQueue = append(r.nreQueue, nre)
			}
		}
		signal(r.nreWake)
	}
	return nil
}

func (r *Refresher) enqueueForced(job Job, scope cache.Scope) {
	for i := range r.queue {
		if r.queue[i].job != job {
			continue
		}
		r.queue[i].forced = true
		r.queue[i].scope.All = r.queue[i].scope.All || scope.All
		r.queue[i].scope.Lists = r.queue[i].scope.Lists || scope.Lists
		r.queue[i].scope.Reports = r.queue[i].scope.Reports || scope.Reports
		return
	}
	r.queue = append(r.queue, task{job: job, forced: true, scope: scope})
}

func (r *Refresher) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()

	jobs := []Job{}
	if r.running != nil {
		jobs = append(jobs, *r.running)
	}
	for _, t := range r.queue {
		jobs = append(jobs, t.job)
	}
	nres := slices.Concat(r.nreRunning, r.nreQueue)
	if nres == nil {
		nres = []string{}
	}
	failed := make(map[string]Issue, len(r.failed))
	for nre, issue := range r.failed {
		failed[nre] = issue
	}
	return Status{
		Active: len(jobs) > 0 || len(nres) > 0,
		Jobs:   jobs,
		NREs:   nres,
		Issues: r.issues,
		Failed: failed,
	}
}

func (r *Refresher) bulkLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.bulkWake:
		}
		for r.runNextBulk(ctx) {
		}
	}
}

func (r *Refresher) runNextBulk(ctx context.Context) bool {
	r.mu.Lock()
	if ctx.Err() != nil || len(r.queue) == 0 {
		r.mu.Unlock()
		return false
	}
	t := r.queue[0]
	r.queue = r.queue[1:]
	r.running = &t.job
	r.mu.Unlock()

	runCtx := ctx
	if t.forced {
		runCtx = cache.WithRefresh(ctx, t.scope)
	}
	start := r.now()
	count, failures, err := r.execute(runCtx, t.job)
	r.finishBulk(ctx, t.job, count, failures, err, r.now().Sub(start))
	return true
}

func (r *Refresher) execute(ctx context.Context, job Job) (int, map[string]Issue, error) {
	switch job {
	case JobVisits:
		result, err := r.ag.Appointments(ctx)
		failures := make(map[string]Issue, len(result.Failed))
		for _, f := range result.Failed {
			failures[f.NRE] = Classify(f.Err)
		}
		return len(result.Entries), failures, err
	case JobLists:
		rows, err := r.ag.Prescriptions(ctx)
		return len(rows), nil, err
	case JobReports:
		listing, err := r.rep.List(ctx)
		return len(listing.Reports), nil, err
	}
	return 0, nil, nil
}

func (r *Refresher) finishBulk(ctx context.Context, job Job, count int, failures map[string]Issue, err error, took time.Duration) {
	issue := Classify(err)

	r.mu.Lock()
	r.running = nil
	switch job {
	case JobVisits:
		r.issues.Appointments = issue
		if err == nil {
			r.failed = failures
		}
	case JobLists:
		r.issues.Lists = issue
	case JobReports:
		r.issues.Reports = issue
	}
	if issue == IssueUnavailable {
		r.unavailable[job] = r.now()
	} else {
		delete(r.unavailable, job)
	}
	r.mu.Unlock()

	if err != nil {
		slog.WarnContext(ctx, "refresh: job failed", slog.String("job", string(job)), slog.String("issue", string(issue)), slog.Duration("took", took))
		return
	}
	slog.InfoContext(ctx, "refresh: job done", slog.String("job", string(job)), slog.Int("count", count), slog.Duration("took", took))
}

func (r *Refresher) nreLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.nreWake:
		}
		for r.runNextNREs(ctx) {
		}
	}
}

func (r *Refresher) runNextNREs(ctx context.Context) bool {
	r.mu.Lock()
	if ctx.Err() != nil || len(r.nreQueue) == 0 {
		r.mu.Unlock()
		return false
	}
	batch := r.nreQueue
	r.nreQueue = nil
	r.nreRunning = batch
	r.mu.Unlock()

	start := r.now()
	err := r.ag.RefreshNREs(cache.WithRefresh(ctx, cache.Scope{NREs: batch}), batch)
	took := r.now().Sub(start)
	issue := Classify(err)

	r.mu.Lock()
	r.nreRunning = nil
	for _, nre := range batch {
		if err == nil {
			delete(r.failed, nre)
			continue
		}
		r.failed[nre] = issue
	}
	r.mu.Unlock()

	if err != nil {
		slog.WarnContext(ctx, "refresh: nres failed", slog.String("issue", string(issue)), slog.Int("count", len(batch)), slog.Duration("took", took))
		return true
	}
	slog.InfoContext(ctx, "refresh: nres done", slog.Int("count", len(batch)), slog.Duration("took", took))
	return true
}
