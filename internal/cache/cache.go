package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/store"
)

const (
	SourceCUPPrescriptions = "cup.prescriptions"
	SourceCUPAppointments  = "cup.appointments"
	SourceCUPVerdict       = "cup.verdict"
	SourceFSEPrescriptions = "fse.prescriptions"
	SourceFSEReports       = "fse.reports"
)

const (
	listsTTL        = time.Hour
	appointmentsTTL = time.Hour
	verdictsTTL     = 24 * time.Hour
	statusOK        = 200
)

//go:generate moq -out store_mock.go . Store:MockStore
type Store interface {
	Put(ctx context.Context, rec store.Record) error
	Latest(ctx context.Context, source, subject string) (store.Record, error)
	LatestBySource(ctx context.Context, source string) ([]store.Record, error)
}

type Policy struct {
	Lists        time.Duration
	Appointments time.Duration
	Verdicts     time.Duration
}

func DefaultPolicy() Policy {
	return Policy{Lists: listsTTL, Appointments: appointmentsTTL, Verdicts: verdictsTTL}
}

type Scope struct {
	All  bool
	NREs []string
}

type scopeKey struct{}

func WithRefresh(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func forced(ctx context.Context, nre string) bool {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok {
		return false
	}
	if scope.All {
		return true
	}
	if nre == "" {
		return false
	}
	for _, n := range scope.NREs {
		if strings.EqualFold(n, nre) {
			return true
		}
	}
	return false
}

type query struct {
	source  string
	subject string
	ttl     time.Duration
	forced  bool
}

func readThrough[T any](ctx context.Context, st Store, now func() time.Time, q query, fetch func(context.Context) (T, error)) (T, error) {
	var stored T
	haveStored := false
	rec, err := st.Latest(ctx, q.source, q.subject)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.WarnContext(ctx, "cache: store read failed", slog.String("source", q.source), slog.String("error", err.Error()))
	}
	if err == nil {
		if jerr := json.Unmarshal(rec.Payload, &stored); jerr != nil {
			slog.WarnContext(ctx, "cache: stored payload undecodable", slog.String("source", q.source), slog.String("error", jerr.Error()))
		} else {
			haveStored = true
		}
	}
	if haveStored && !q.forced && now().Sub(rec.FetchedAt) < q.ttl {
		return stored, nil
	}

	fresh, err := fetch(ctx)
	if err != nil {
		if !haveStored {
			return fresh, err
		}
		slog.WarnContext(ctx, "cache: upstream failed, serving stale", slog.String("source", q.source), slog.String("error", err.Error()))
		return stored, nil
	}

	payload, err := json.Marshal(fresh)
	if err != nil {
		slog.WarnContext(ctx, "cache: encode failed", slog.String("source", q.source), slog.String("error", err.Error()))
		return fresh, nil
	}
	put := store.Record{Source: q.source, Subject: q.subject, FetchedAt: now().UTC(), Status: statusOK, Payload: payload}
	if err := st.Put(ctx, put); err != nil {
		slog.WarnContext(ctx, "cache: store write failed", slog.String("source", q.source), slog.String("error", err.Error()))
	}
	return fresh, nil
}

func FetchedAt(ctx context.Context, st Store, source string) (map[string]time.Time, error) {
	recs, err := st.LatestBySource(ctx, source)
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(recs))
	for _, r := range recs {
		out[r.Subject] = r.FetchedAt
	}
	return out, nil
}

//go:generate moq -out cupsource_mock.go . CUPSource:MockCUPSource
type CUPSource interface {
	Prescriptions(ctx context.Context) ([]cup.Prescription, error)
	Appointments(ctx context.Context, nre string) ([]cup.Appointment, error)
	Verdict(ctx context.Context, nre string) (cup.Verdict, error)
}

//go:generate moq -out fsesource_mock.go . FSESource:MockFSESource
type FSESource interface {
	Prescriptions(ctx context.Context) ([]fse.Prescription, error)
}

type CUP struct {
	src    CUPSource
	st     Store
	policy Policy
	now    func() time.Time
}

func NewCUP(src CUPSource, st Store, p Policy) *CUP {
	return &CUP{src: src, st: st, policy: p, now: time.Now}
}

func (c *CUP) Prescriptions(ctx context.Context) ([]cup.Prescription, error) {
	q := query{source: SourceCUPPrescriptions, ttl: c.policy.Lists, forced: forced(ctx, "")}
	return readThrough(ctx, c.st, c.now, q, c.src.Prescriptions)
}

func (c *CUP) Appointments(ctx context.Context, nre string) ([]cup.Appointment, error) {
	nre = strings.ToUpper(nre)
	q := query{source: SourceCUPAppointments, subject: nre, ttl: c.policy.Appointments, forced: forced(ctx, nre)}
	return readThrough(ctx, c.st, c.now, q, func(ctx context.Context) ([]cup.Appointment, error) {
		return c.src.Appointments(ctx, nre)
	})
}

func (c *CUP) Verdict(ctx context.Context, nre string) (cup.Verdict, error) {
	nre = strings.ToUpper(nre)
	q := query{source: SourceCUPVerdict, subject: nre, ttl: c.policy.Verdicts, forced: forced(ctx, nre)}
	return readThrough(ctx, c.st, c.now, q, func(ctx context.Context) (cup.Verdict, error) {
		v, err := c.src.Verdict(ctx, nre)
		if err != nil {
			return v, err
		}
		v.Raw = stripPatient(v.Raw)
		return v, nil
	})
}

func stripPatient(raw []byte) []byte {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return raw
	}
	respRaw, ok := top["response"]
	if !ok {
		return raw
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return raw
	}
	if _, ok := resp["paziente"]; !ok {
		return raw
	}
	delete(resp, "paziente")
	newResp, err := json.Marshal(resp)
	if err != nil {
		return raw
	}
	top["response"] = newResp
	out, err := json.Marshal(top)
	if err != nil {
		return raw
	}
	return out
}

type FSE struct {
	src    FSESource
	st     Store
	policy Policy
	now    func() time.Time
}

func NewFSE(src FSESource, st Store, p Policy) *FSE {
	return &FSE{src: src, st: st, policy: p, now: time.Now}
}

func (f *FSE) Prescriptions(ctx context.Context) ([]fse.Prescription, error) {
	q := query{source: SourceFSEPrescriptions, ttl: f.policy.Lists, forced: forced(ctx, "")}
	return readThrough(ctx, f.st, f.now, q, f.src.Prescriptions)
}

//go:generate moq -out reportssource_mock.go . ReportsSource:MockReportsSource
type ReportsSource interface {
	List(ctx context.Context) (referti.Listing, error)
}

type storedFailureError string

func (e storedFailureError) Error() string { return string(e) }

type storedFailure struct {
	Kind    referti.Kind `json:"kind"`
	Message string       `json:"message"`
}

type storedListing struct {
	Reports  []referti.Report `json:"reports"`
	Failures []storedFailure  `json:"failures"`
}

func toStored(l referti.Listing) storedListing {
	failures := make([]storedFailure, 0, len(l.Failures))
	for _, f := range l.Failures {
		failures = append(failures, storedFailure{Kind: f.Kind, Message: f.Err.Error()})
	}
	return storedListing{Reports: l.Reports, Failures: failures}
}

func fromStored(s storedListing) referti.Listing {
	failures := make([]referti.Failure, 0, len(s.Failures))
	for _, f := range s.Failures {
		failures = append(failures, referti.Failure{Kind: f.Kind, Err: storedFailureError(f.Message)})
	}
	return referti.Listing{Reports: s.Reports, Failures: failures}
}

type Reports struct {
	src    ReportsSource
	st     Store
	policy Policy
	now    func() time.Time
}

func NewReports(src ReportsSource, st Store, p Policy) *Reports {
	return &Reports{src: src, st: st, policy: p, now: time.Now}
}

func (r *Reports) List(ctx context.Context) (referti.Listing, error) {
	q := query{source: SourceFSEReports, ttl: r.policy.Lists, forced: forced(ctx, "")}
	stored, err := readThrough(ctx, r.st, r.now, q, func(ctx context.Context) (storedListing, error) {
		listing, err := r.src.List(ctx)
		if err != nil {
			return storedListing{}, err
		}
		return toStored(listing), nil
	})
	if err != nil {
		return referti.Listing{}, err
	}
	return fromStored(stored), nil
}
