package reportsview

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
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

type loaded struct {
	cached   reports.CachedListing
	snapshot agenda.Snapshot
	status   refresh.Status
}

func (s *Service) load(ctx context.Context) (loaded, error) {
	cached, err := s.rep.CachedList(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("reportsview cached list: %w", err)
	}
	snapshot, err := s.ag.Snapshot(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("reportsview snapshot: %w", err)
	}
	if cached.Stale || cached.Missing {
		s.ref.Kick(refresh.JobReports)
	}
	return loaded{cached: cached, snapshot: snapshot, status: s.ref.Status()}, nil
}

func (l loaded) refreshing() bool {
	return l.status.Active && slices.Contains(l.status.Jobs, refresh.JobReports)
}

func (s *Service) Query(ctx context.Context, q Query) (Page, error) {
	data, err := s.load(ctx)
	if err != nil {
		return Page{}, err
	}
	all := buildRows(data.cached.Listing.Reports, visitTitles(data.snapshot.Rows))
	failures, fseUnavailable := splitFailures(data.cached.Listing.Failures)

	excluded := make(map[string]bool, len(q.ExcludeKinds))
	for _, kind := range q.ExcludeKinds {
		excluded[kind] = true
	}
	kinds := kindOptions(all, excluded)
	excludedKinds := make([]string, 0, len(kinds))
	for _, k := range kinds {
		if k.Excluded {
			excludedKinds = append(excludedKinds, k.Key)
		}
	}
	visible := excludeKinds(searchRows(all, q.Search), excluded)
	sortRows(visible, q.Sort, q.Dir)
	window := listview.Paginate(len(visible), q.Page, q.PageSize)

	return Page{
		Rows:           visible[window.Start:window.End:window.End],
		Total:          len(visible),
		Page:           window.Page,
		PageSize:       window.PageSize,
		Kinds:          kinds,
		ExcludedKinds:  excludedKinds,
		Failures:       failures,
		FSEUnavailable: fseUnavailable,
		Refreshing:     data.refreshing(),
		Updated:        s.updated(data.cached),
		Issue:          issue(data),
	}, nil
}

func (s *Service) Row(ctx context.Context, ref referti.Ref) (RowResult, error) {
	data, err := s.load(ctx)
	if err != nil {
		return RowResult{}, err
	}
	key := ref.Key()
	for _, row := range buildRows(data.cached.Listing.Reports, visitTitles(data.snapshot.Rows)) {
		if row.ID == key {
			return RowResult{Row: row, Refreshing: data.refreshing()}, nil
		}
	}
	return RowResult{}, ErrNotFound
}

func (s *Service) VisitOptions(ctx context.Context, ref referti.Ref) ([]LinkedVisit, error) {
	data, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	key := ref.Key()
	for _, report := range data.cached.Listing.Reports {
		if report.Ref.Key() == key {
			return unlinkedVisits(data.snapshot.Rows, report.NREs), nil
		}
	}
	return nil, ErrNotFound
}

func (s *Service) updated(cached reports.CachedListing) listview.Updated {
	if cached.Missing {
		return listview.Updated{}
	}
	return listview.Updated{At: cached.FetchedAt, Label: itfmt.UpdatedLabel(cached.FetchedAt, s.now())}
}

func issue(data loaded) refresh.Issue {
	if !data.cached.Missing {
		return refresh.IssueNone
	}
	switch data.status.Issues.Reports {
	case refresh.IssueLoginRequired, refresh.IssueUnavailable:
		return data.status.Issues.Reports
	}
	return refresh.IssueNone
}

func visitTitles(rows []agenda.Row) map[string]string {
	titles := make(map[string]string, len(rows))
	for _, row := range rows {
		titles[strings.ToUpper(row.NRE)] = row.Title()
	}
	return titles
}
