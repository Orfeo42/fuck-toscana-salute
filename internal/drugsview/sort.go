package drugsview

import (
	"cmp"
	"slices"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/textutil"
)

func sortRows(rows []Row, key Sort, dir listview.Dir) {
	slices.SortStableFunc(rows, func(a, b Row) int {
		if c := comparePrimary(a, b, key, dir); c != 0 {
			return c
		}
		return textutil.CompareNatural(a.NRE, b.NRE)
	})
}

func comparePrimary(a, b Row, key Sort, dir listview.Dir) int {
	if key == SortDate && (a.Date.IsZero() || b.Date.IsZero()) {
		return listview.MissingLast(a.Date.IsZero(), b.Date.IsZero())
	}
	return listview.Directed(comparePresent(a, b, key), dir)
}

func comparePresent(a, b Row, key Sort) int {
	switch key {
	case SortName:
		return textutil.CompareNatural(a.Title, b.Title)
	case SortStatus:
		return cmp.Compare(statusRank(a.Status.Key), statusRank(b.Status.Key))
	case SortNRE:
		return textutil.CompareNatural(a.NRE, b.NRE)
	default:
		return a.Date.Compare(b.Date)
	}
}
