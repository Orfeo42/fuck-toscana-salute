package visitsview

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
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

type loaded struct {
	in         inputs
	status     refresh.Status
	refreshing bool
	listing    reports.CachedListing
}

func (s *Service) load(ctx context.Context) (loaded, error) {
	snap, err := s.ag.Snapshot(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("visitsview snapshot: %w", err)
	}
	listing, err := s.rep.CachedList(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("visitsview reports: %w", err)
	}
	overrides, err := s.local.Overrides(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("visitsview overrides: %w", err)
	}
	if needsRefresh(snap) {
		s.ref.Kick(refresh.JobVisits)
	}
	status := s.ref.Status()
	in := inputs{
		snap:      snap,
		overrides: map[string]int{},
		linked:    linkedByNRE(listing),
		forced:    map[string]bool{},
		failed:    map[string]refresh.Issue{},
		visitsRun: status.Active && slices.Contains(status.Jobs, refresh.JobVisits),
		now:       s.now(),
	}
	for _, o := range overrides {
		in.overrides[strings.ToUpper(o.NRE)] = o.Status
	}
	for _, nre := range status.NREs {
		in.forced[strings.ToUpper(nre)] = true
	}
	for nre, issue := range status.Failed {
		in.failed[strings.ToUpper(nre)] = issue
	}
	feeding := status.Active && (in.visitsRun || slices.Contains(status.Jobs, refresh.JobLists) || len(status.NREs) > 0)
	return loaded{in: in, status: status, refreshing: feeding, listing: listing}, nil
}

func needsRefresh(snap agenda.Snapshot) bool {
	if snap.CUPList.Missing || snap.CUPList.Stale || snap.FSEList.Missing || snap.FSEList.Stale {
		return true
	}
	for _, l := range snap.Lookups {
		if !l.Candidate {
			continue
		}
		if !l.Checked || l.Stale {
			return true
		}
		if !l.Closed && len(l.Appointments) == 0 && (l.Verdict == nil || l.Verdict.Stale) {
			return true
		}
	}
	return false
}

func (l loaded) issues() Issues {
	issues := Issues{Lists: refresh.IssueNone, Appointments: refresh.IssueNone}
	snap := l.in.snap
	if snap.CUPList.Missing && snap.FSEList.Missing {
		issues.Lists = l.status.Issues.Lists
	}
	for _, lk := range snap.Lookups {
		if lk.Checked {
			return issues
		}
	}
	if l.status.Issues.Appointments != "" {
		issues.Appointments = l.status.Issues.Appointments
	}
	return issues
}

func (l loaded) updated() listview.Updated {
	at := l.in.snap.CUPList.At
	other := l.in.snap.FSEList.At
	if at.IsZero() || (!other.IsZero() && other.Before(at)) {
		at = other
	}
	return listview.Updated{At: at, Label: itfmt.UpdatedLabel(at, l.in.now)}
}

func (s *Service) Query(ctx context.Context, q Query) (Page, error) {
	l, err := s.load(ctx)
	if err != nil {
		return Page{}, err
	}
	rows := l.in.rows()
	excluded := excludedSet(q.ExcludeStatuses)
	search := q.Search
	var focus *listview.Focus
	focusAt := -1
	if q.Focus != "" {
		focus = &listview.Focus{NRE: q.Focus}
		if at := findRow(rows, q.Focus); at >= 0 {
			focus.Found = true
			target := rows[at]
			if !matchesSearch(target, search) || excluded[target.Status.Key] {
				delete(excluded, target.Status.Key)
				search = ""
			}
		}
	}
	searched := searchRows(rows, search)
	statuses := countStatuses(searched, excluded)
	visible := excludeRows(searched, excluded)
	key, dir := q.Sort, q.Dir
	if key == "" {
		key = SortPrescribed
	}
	if dir == "" {
		dir = listview.DirDesc
	}
	sortRows(visible, key, dir)

	size := listview.PageSizeOrDefault(q.PageSize)
	page := q.Page
	if focus != nil && focus.Found {
		focusAt = findRow(visible, q.Focus)
	}
	if focusAt >= 0 {
		page = focusAt/size + 1
	}
	window := listview.Paginate(len(visible), page, size)
	return Page{
		Rows:             visible[window.Start:window.End],
		Total:            len(visible),
		Page:             window.Page,
		PageSize:         size,
		Statuses:         statuses,
		ExcludedStatuses: excludedKeys(excluded),
		Search:           search,
		Focus:            focus,
		Refreshing:       l.refreshing,
		Updated:          l.updated(),
		Issues:           l.issues(),
	}, nil
}

func (s *Service) Row(ctx context.Context, nre string) (RowResult, error) {
	l, err := s.load(ctx)
	if err != nil {
		return RowResult{}, err
	}
	rows := l.in.rows()
	at := findRow(rows, nre)
	if at < 0 {
		return RowResult{}, ErrNotFound
	}
	row := rows[at]
	manual := ManualStatus{Options: make([]ManualOption, 0, len(cup.SelectableStatuses))}
	if code, ok := l.in.overrides[strings.ToUpper(nre)]; ok {
		manual.Current = &code
	}
	for _, st := range cup.SelectableStatuses {
		manual.Options = append(manual.Options, ManualOption{Code: int(st), Label: st.Label()})
	}
	return RowResult{Row: row, ManualStatus: manual, Refreshing: row.Refreshing, Issues: l.issues()}, nil
}

func (s *Service) ReportOptions(ctx context.Context, nre string) ([]ReportOption, error) {
	listing, err := s.rep.CachedList(ctx)
	if err != nil {
		return nil, fmt.Errorf("visitsview reports: %w", err)
	}
	options := []ReportOption{}
	for _, r := range listing.Listing.Reports {
		if slices.ContainsFunc(r.NREs, func(n string) bool { return strings.EqualFold(n, nre) }) {
			continue
		}
		options = append(options, ReportOption{Ref: r.Ref, Label: reportLabel(r), Date: r.Date, Provider: r.Provider, Origin: r.Origin})
	}
	slices.SortStableFunc(options, func(a, b ReportOption) int { return compareTimes(a.Date, b.Date, listview.DirDesc) })
	return options, nil
}
