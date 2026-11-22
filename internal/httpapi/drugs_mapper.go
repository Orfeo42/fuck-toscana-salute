package httpapi

import (
	"fuck-toscana-salute/internal/drugsview"
)

func toDrugsQueryBody(p drugsview.Page) DrugsQueryBody {
	rows := make([]DrugRowBody, 0, len(p.Rows))
	for _, r := range p.Rows {
		rows = append(rows, toDrugRowBody(r))
	}
	return DrugsQueryBody{
		Rows:             rows,
		Total:            p.Total,
		Page:             p.Page,
		PageSize:         p.PageSize,
		Statuses:         toStatusOptionBodies(p.Statuses),
		ExcludedStatuses: cloneStrings(p.ExcludedStatuses),
		Search:           p.Search,
		Focus:            toFocusBody(p.Focus),
		Refreshing:       p.Refreshing,
		Updated:          toUpdatedBody(p.Updated),
		Issue:            string(p.Issue),
	}
}

func toDrugRowBody(r drugsview.Row) DrugRowBody {
	return DrugRowBody{
		NRE:         r.NRE,
		Title:       r.Title,
		Date:        timePtr(r.Date),
		Status:      DrugStatusBody{Key: r.Status.Key, Label: r.Status.Label, Tone: r.Status.Tone},
		DocumentRef: r.DocumentRef,
	}
}
