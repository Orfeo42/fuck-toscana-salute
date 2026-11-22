package drugsview

import (
	"slices"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/textutil"
)

func searchRows(rows []Row, search string) []Row {
	found := make([]Row, 0, len(rows))
	for _, r := range rows {
		if textutil.ContainsFolded(r.Title, search) || textutil.ContainsFolded(r.NRE, search) {
			found = append(found, r)
		}
	}
	return found
}

func excludeRows(rows []Row, excluded []string) []Row {
	kept := make([]Row, 0, len(rows))
	for _, r := range rows {
		if !slices.Contains(excluded, r.Status.Key) {
			kept = append(kept, r)
		}
	}
	return kept
}

func statusOptions(rows []Row, excluded []string) []listview.StatusOption {
	counts := make(map[string]int, len(statusDefs))
	for _, r := range rows {
		counts[r.Status.Key]++
	}
	options := make([]listview.StatusOption, 0, len(counts))
	for _, def := range statusDefs {
		if counts[def.key] == 0 {
			continue
		}
		options = append(options, listview.StatusOption{
			Key:      def.key,
			Label:    def.label,
			Tone:     def.tone,
			Count:    counts[def.key],
			Excluded: slices.Contains(excluded, def.key),
		})
	}
	return options
}
