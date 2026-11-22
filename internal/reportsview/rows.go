package reportsview

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/reports"
	"fuck-toscana-salute/internal/textutil"
)

func buildRows(list []reports.Report, titles map[string]string) []Row {
	rows := make([]Row, 0, len(list))
	for _, report := range list {
		rows = append(rows, buildRow(report, titles))
	}
	return rows
}

func buildRow(report reports.Report, titles map[string]string) Row {
	kindLabel := report.Ref.Kind.Label()
	row := Row{
		ID:          report.Ref.Key(),
		Ref:         report.Ref,
		Kind:        string(report.Ref.Kind),
		KindLabel:   kindLabel,
		Title:       kindLabel,
		CustomTitle: report.Title,
		Date:        report.Date,
		UpdatedAt:   report.UpdatedAt,
		Provider:    report.Provider,
		Unread:      report.Unread,
		Hidden:      report.Hidden,
		Origin:      report.Origin,
		Visits:      linkedVisits(report.NREs, titles),
		LinkedLabel: linkedLabel(len(report.NREs)),
	}
	if report.Origin == reports.OriginLocal && report.Title != "" {
		row.Title = report.Title
	}
	if !report.Date.IsZero() {
		row.EditDate = itfmt.Day(report.Date)
	}
	return row
}

func linkedLabel(count int) string {
	switch count {
	case 0:
		return ""
	case 1:
		return "Collegato a 1 visita"
	}
	return fmt.Sprintf("Collegato a %d visite", count)
}

func linkedVisits(nres []string, titles map[string]string) []LinkedVisit {
	visits := make([]LinkedVisit, 0, len(nres))
	for _, nre := range nres {
		title, ok := titles[strings.ToUpper(nre)]
		if !ok {
			title = nre
		}
		visits = append(visits, LinkedVisit{NRE: nre, Title: title})
	}
	return visits
}

func unlinkedVisits(rows []agenda.Row, linked []string) []LinkedVisit {
	candidates := make([]agenda.Row, 0, len(rows))
	for _, row := range rows {
		if !row.IsVisit() || containsFold(linked, row.NRE) {
			continue
		}
		candidates = append(candidates, row)
	}
	slices.SortStableFunc(candidates, func(a, b agenda.Row) int {
		if c := b.PrescribedOn().Compare(a.PrescribedOn()); c != 0 {
			return c
		}
		return cmp.Compare(a.NRE, b.NRE)
	})
	visits := make([]LinkedVisit, 0, len(candidates))
	for _, row := range candidates {
		visits = append(visits, LinkedVisit{NRE: row.NRE, Title: row.Title()})
	}
	return visits
}

func containsFold(list []string, value string) bool {
	return slices.ContainsFunc(list, func(item string) bool { return strings.EqualFold(item, value) })
}

func kindOptions(rows []Row, excluded map[string]bool) []KindOption {
	index := map[string]int{}
	options := make([]KindOption, 0)
	for _, row := range rows {
		i, ok := index[row.Kind]
		if !ok {
			i = len(options)
			index[row.Kind] = i
			options = append(options, KindOption{Key: row.Kind, Label: row.KindLabel, Excluded: excluded[row.Kind]})
		}
		options[i].Count++
	}
	slices.SortFunc(options, func(a, b KindOption) int {
		if c := textutil.CompareNatural(a.Label, b.Label); c != 0 {
			return c
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return options
}

func splitFailures(failures []referti.Failure) ([]Failure, bool) {
	out := make([]Failure, 0, len(failures))
	fseUnavailable := false
	for _, f := range failures {
		if f.Kind == referti.KindFSE {
			fseUnavailable = true
			continue
		}
		out = append(out, Failure{Kind: string(f.Kind), KindLabel: f.Kind.Label(), Message: f.Err.Error()})
	}
	return out, fseUnavailable
}
