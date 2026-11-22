package httpapi

import (
	"fuck-toscana-salute/internal/reportsview"
)

func toReportsQueryBody(p reportsview.Page) ReportsQueryBody {
	rows := make([]ReportRowBody, 0, len(p.Rows))
	for _, r := range p.Rows {
		rows = append(rows, toReportRowBody(r))
	}
	kinds := make([]KindOptionBody, 0, len(p.Kinds))
	for _, k := range p.Kinds {
		kinds = append(kinds, KindOptionBody{Key: k.Key, Label: k.Label, Count: k.Count, Excluded: k.Excluded})
	}
	excluded := append([]string{}, p.ExcludedKinds...)
	failures := make([]ReportFailureBody, 0, len(p.Failures))
	for _, f := range p.Failures {
		failures = append(failures, ReportFailureBody{Kind: f.Kind, KindLabel: f.KindLabel, Message: f.Message})
	}
	return ReportsQueryBody{
		Rows:           rows,
		Total:          p.Total,
		Page:           p.Page,
		PageSize:       p.PageSize,
		Kinds:          kinds,
		ExcludedKinds:  excluded,
		Failures:       failures,
		FSEUnavailable: p.FSEUnavailable,
		Refreshing:     p.Refreshing,
		Updated:        toUpdatedBody(p.Updated),
		Issue:          string(p.Issue),
	}
}

func toReportRowBody(r reportsview.Row) ReportRowBody {
	return ReportRowBody{
		ID:          r.ID,
		Ref:         toReportRefBody(r.Ref),
		Kind:        r.Kind,
		KindLabel:   r.KindLabel,
		Title:       r.Title,
		CustomTitle: r.CustomTitle,
		Date:        timePtr(r.Date),
		EditDate:    r.EditDate,
		UpdatedAt:   timePtr(r.UpdatedAt),
		Provider:    r.Provider,
		Unread:      r.Unread,
		Hidden:      r.Hidden,
		Origin:      string(r.Origin),
		Visits:      toLinkedVisitBodies(r.Visits),
		LinkedLabel: r.LinkedLabel,
	}
}

func toLinkedVisitBodies(visits []reportsview.LinkedVisit) []LinkedVisitBody {
	bodies := make([]LinkedVisitBody, 0, len(visits))
	for _, v := range visits {
		bodies = append(bodies, LinkedVisitBody{NRE: v.NRE, Title: v.Title})
	}
	return bodies
}
