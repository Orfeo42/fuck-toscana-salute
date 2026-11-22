package drugsview

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/refresh"
)

const fallbackTitle = "Farmaco"

type Service struct {
	ag    Agenda
	local Local
	ref   Refresher
	now   func() time.Time
}

func New(ag Agenda, local Local, ref Refresher) *Service {
	return &Service{ag: ag, local: local, ref: ref, now: time.Now}
}

type loaded struct {
	rows       []Row
	updated    time.Time
	refreshing bool
	issue      refresh.Issue
}

func (s *Service) load(ctx context.Context) (loaded, error) {
	snap, err := s.ag.Snapshot(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("drugsview snapshot: %w", err)
	}
	overrides, err := s.local.Overrides(ctx)
	if err != nil {
		return loaded{}, fmt.Errorf("drugsview overrides: %w", err)
	}
	if snap.FSEList.Stale || snap.FSEList.Missing {
		s.ref.Kick(refresh.JobLists)
	}
	status := s.ref.Status()
	issue := status.Issues.Lists
	if issue == "" {
		issue = refresh.IssueNone
	}
	statuses := make(map[string]int, len(overrides))
	for _, o := range overrides {
		statuses[strings.ToUpper(o.NRE)] = o.Status
	}
	return loaded{
		rows:       drugRows(snap.Rows, statuses),
		updated:    snap.FSEList.At,
		refreshing: slices.Contains(status.Jobs, refresh.JobLists),
		issue:      issue,
	}, nil
}

func drugRows(rows []agenda.Row, statuses map[string]int) []Row {
	drugs := make([]Row, 0, len(rows))
	for _, r := range rows {
		if r.FSE == nil || r.IsVisit() {
			continue
		}
		code := r.FSE.Status
		if override, ok := statuses[strings.ToUpper(r.NRE)]; ok {
			code = override
		}
		title := r.FSE.Description
		if title == "" {
			title = fallbackTitle
		}
		drugs = append(drugs, Row{
			NRE:         r.NRE,
			Title:       title,
			Date:        r.FSE.Date,
			Status:      statusOf(code),
			DocumentRef: r.FSE.DocumentRef,
		})
	}
	return drugs
}

func (s *Service) Row(ctx context.Context, nre string) (RowResult, error) {
	data, err := s.load(ctx)
	if err != nil {
		return RowResult{}, err
	}
	row, ok := findRow(data.rows, nre)
	if !ok {
		return RowResult{}, ErrNotFound
	}
	return RowResult{Row: row, Refreshing: data.refreshing}, nil
}

func findRow(rows []Row, nre string) (Row, bool) {
	i := slices.IndexFunc(rows, func(r Row) bool { return strings.EqualFold(r.NRE, nre) })
	if i < 0 {
		return Row{}, false
	}
	return rows[i], true
}

func (s *Service) Query(ctx context.Context, q Query) (Page, error) {
	data, err := s.load(ctx)
	if err != nil {
		return Page{}, err
	}
	excluded := slices.Clone(defaultExcluded)
	if q.ExcludeStatuses != nil {
		excluded = slices.Clone(q.ExcludeStatuses)
	}
	search := q.Search
	var focus *listview.Focus
	if q.Focus != "" {
		focus = &listview.Focus{NRE: q.Focus}
		if row, ok := findRow(data.rows, q.Focus); ok {
			focus.Found = true
			excluded = slices.DeleteFunc(excluded, func(key string) bool { return key == row.Status.Key })
			search = ""
		}
	}
	searched := searchRows(data.rows, search)
	visible := excludeRows(searched, excluded)
	sortRows(visible, q.Sort, q.Dir)

	pageSize := listview.PageSizeOrDefault(q.PageSize)
	page := q.Page
	if focus != nil && focus.Found {
		if i := slices.IndexFunc(visible, func(r Row) bool { return strings.EqualFold(r.NRE, focus.NRE) }); i >= 0 {
			page = i/pageSize + 1
		}
	}
	window := listview.Paginate(len(visible), page, pageSize)

	return Page{
		Rows:             visible[window.Start:window.End],
		Total:            len(visible),
		Page:             window.Page,
		PageSize:         window.PageSize,
		Statuses:         statusOptions(searched, excluded),
		ExcludedStatuses: excluded,
		Search:           search,
		Focus:            focus,
		Refreshing:       data.refreshing,
		Updated:          listview.Updated{At: data.updated, Label: itfmt.UpdatedLabel(data.updated, s.now())},
		Issue:            data.issue,
	}, nil
}
