package facilities

import "fuck-toscana-salute/internal/textutil"

func mergeMinistry(all []Facility) []Facility {
	index := make(map[string]int, len(all))
	var byName []Facility
	for _, f := range all {
		key := textutil.Fold(f.Name) + "|" + textutil.Fold(f.Comune)
		i, ok := index[key]
		if !ok {
			index[key] = len(byName)
			byName = append(byName, f)
			continue
		}
		if byName[i].Address == "" && f.Address != "" {
			byName[i] = f
		}
	}
	seen := make(map[string]bool, len(byName))
	out := make([]Facility, 0, len(byName))
	for _, f := range byName {
		if seen[f.Code] {
			continue
		}
		seen[f.Code] = true
		out = append(out, f)
	}
	return out
}
