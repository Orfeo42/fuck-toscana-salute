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

func excludeKinds(rows []Row, excluded map[string]bool) []Row {
	found := make([]Row, 0, len(rows))
	for _, row := range rows {
		if !excluded[row.Kind] {
			found = append(found, row)
		}
	}
	return found
}
