package refresh

import (
	"context"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/notimpl"
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

type Refresher struct {
	ag  Agenda
	rep Reports
}

func New(ag Agenda, rep Reports) *Refresher {
	return &Refresher{ag: ag, rep: rep}
}

func (r *Refresher) Run(_ context.Context) {}

func (r *Refresher) Kick(_ Job) {}

func (r *Refresher) Start(_ Scope) error {
	return notimpl.ErrNotImplemented
}

func (r *Refresher) Status() Status {
	return Status{}
}
