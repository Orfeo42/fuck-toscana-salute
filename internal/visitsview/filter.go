package visitsview

import (
	"slices"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/textutil"
)

func matchesSearch(r Row, search string) bool {
	if textutil.Fold(search) == "" {
		return true
	}
	texts := []string{r.NRE, r.Title}
	if r.Appointment != nil {
		u := r.Appointment.Unit
		texts = append(texts, u.Name, u.Description, u.City)
	}
	return slices.ContainsFunc(texts, func(t string) bool { return textutil.ContainsFolded(t, search) })
}

func searchRows(rows []Row, search string) []Row {
	found := make([]Row, 0, len(rows))
	for _, r := range rows {
		if matchesSearch(r, search) {
			found = append(found, r)
		}
	}
	return found
}

func excludeRows(rows []Row, excluded map[string]bool) []Row {
	kept := make([]Row, 0, len(rows))
	for _, r := range rows {
		if !excluded[r.Status.Key] {
			kept = append(kept, r)
		}
	}
	return kept
}

func countStatuses(rows []Row, excluded map[string]bool) []listview.StatusOption {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status.Key]++
	}
	options := []listview.StatusOption{}
	for _, d := range statusDefs {
		if counts[d.key] == 0 {
			continue
		}
		options = append(options, listview.StatusOption{
			Key:      d.key,
			Label:    d.label,
			Tone:     d.tone,
			Count:    counts[d.key],
			Excluded: excluded[d.key],
		})
	}
	return options
}

func excludedSet(requested []string) map[string]bool {
	if requested == nil {
		return defaultExcluded()
	}
	excluded := map[string]bool{}
	for _, key := range requested {
		excluded[key] = true
	}
	return excluded
}

func excludedKeys(excluded map[string]bool) []string {
	keys := []string{}
	for _, d := range statusDefs {
		if excluded[d.key] {
			keys = append(keys, d.key)
		}
	}
	return keys
}
