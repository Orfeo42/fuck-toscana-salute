package reports

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/notimpl"
	"fuck-toscana-salute/internal/referti"
)

const localPosition = "1"

type Origin string

const (
	OriginRemote Origin = "remote"
	OriginLocal  Origin = "local"
)

type Report struct {
	referti.Report
	Origin Origin
	Title  string
	NREs   []string
}

type Listing struct {
	Reports  []Report
	Failures []referti.Failure
}

type CachedListing struct {
	Listing   Listing
	FetchedAt time.Time
	Stale     bool
	Missing   bool
}

//go:generate moq -out deps_mock.go . Remote:MockRemote Local:MockLocal
type Remote interface {
	List(ctx context.Context) (referti.Listing, error)
	CachedList(ctx context.Context) (cache.Entry[referti.Listing], error)
	Documents(ctx context.Context, ref referti.Ref) ([]referti.Document, error)
	Download(ctx context.Context, ref referti.Ref, position string) ([]byte, error)
}

type Local interface {
	List(ctx context.Context) ([]localrecords.Report, error)
	File(ctx context.Context, id string) (localrecords.File, error)
	Links(ctx context.Context) ([]localrecords.Link, error)
}

type Service struct {
	remote Remote
	local  Local
}

func New(remote Remote, local Local) *Service {
	return &Service{remote: remote, local: local}
}

func (s *Service) List(ctx context.Context) (Listing, error) {
	remote, err := s.remote.List(ctx)
	if err != nil {
		slog.WarnContext(ctx, "remote reports unavailable", slog.String("error", err.Error()))
		remote = referti.Listing{Failures: []referti.Failure{{Kind: referti.KindFSE, Err: err}}}
	}
	locals, err := s.local.List(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("reports list local: %w", err)
	}
	links, err := s.local.Links(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("reports list links: %w", err)
	}
	nres := map[string][]string{}
	for _, l := range links {
		key := l.Ref.Key()
		if slices.Contains(nres[key], l.NRE) {
			continue
		}
		nres[key] = append(nres[key], l.NRE)
	}
	for _, v := range nres {
		slices.Sort(v)
	}

	reports := make([]Report, 0, len(remote.Reports)+len(locals))
	for _, r := range remote.Reports {
		reports = append(reports, Report{Report: r, Origin: OriginRemote, NREs: nresFor(nres, r.Ref)})
	}
	for _, l := range locals {
		ref := localrecords.LocalRef(l.ID)
		reports = append(reports, Report{
			Report: referti.Report{Ref: ref, Date: l.Meta.Date, UpdatedAt: l.CreatedAt, Provider: l.Meta.Provider},
			Origin: OriginLocal,
			Title:  l.Meta.Title,
			NREs:   nresFor(nres, ref),
		})
	}
	slices.SortStableFunc(reports, compareDateDesc)

	slog.InfoContext(ctx, "reports listed",
		slog.Int("remote", len(remote.Reports)),
		slog.Int("local", len(locals)),
		slog.Int("failures", len(remote.Failures)))
	return Listing{Reports: reports, Failures: remote.Failures}, nil
}

func (s *Service) CachedList(_ context.Context) (CachedListing, error) {
	return CachedListing{}, notimpl.ErrNotImplemented
}

func nresFor(nres map[string][]string, ref referti.Ref) []string {
	found := nres[ref.Key()]
	if found == nil {
		return []string{}
	}
	return found
}

func compareDateDesc(a, b Report) int {
	if a.Date.IsZero() != b.Date.IsZero() {
		if a.Date.IsZero() {
			return 1
		}
		return -1
	}
	return b.Date.Compare(a.Date)
}

func (s *Service) Documents(ctx context.Context, ref referti.Ref) ([]referti.Document, error) {
	if ref.Kind != referti.KindLocal {
		return s.remote.Documents(ctx, ref)
	}
	locals, err := s.local.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("reports documents local: %w", err)
	}
	for _, l := range locals {
		if l.ID != ref.AbstractID {
			continue
		}
		label := l.Filename
		if label == "" {
			label = l.Meta.Title
		}
		return []referti.Document{{Position: localPosition, Label: label, AcceptedAt: l.CreatedAt, Provider: l.Meta.Provider}}, nil
	}
	return nil, localrecords.ErrNotFound
}

func (s *Service) Download(ctx context.Context, ref referti.Ref, position string) ([]byte, error) {
	if ref.Kind != referti.KindLocal {
		return s.remote.Download(ctx, ref, position)
	}
	if position != localPosition {
		return nil, referti.ErrNoDocument
	}
	file, err := s.local.File(ctx, ref.AbstractID)
	if err != nil {
		return nil, fmt.Errorf("reports download local: %w", err)
	}
	return file.Data, nil
}
