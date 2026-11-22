package reportsview

import (
	"strings"

	"fuck-toscana-salute/internal/textutil"
)

func searchRows(rows []Row, search string) []Row {
	if strings.TrimSpace(search) == "" {
		return append([]Row(nil), rows...)
	}
	found := make([]Row, 0, len(rows))
	for _, row := range rows {
		if textutil.ContainsFolded(row.Provider, search) ||
			textutil.ContainsFolded(row.KindLabel, search) ||
			textutil.ContainsFolded(row.Title, search) {
			found = append(found, row)
		}
	}
	return found
}

func filterByKind(rows []Row, kind string) []Row {
	if kind == "" {
		return rows
	}
	found := make([]Row, 0, len(rows))
	for _, row := range rows {
		if row.Kind == kind {
			found = append(found, row)
		}
	}
	return found
}
