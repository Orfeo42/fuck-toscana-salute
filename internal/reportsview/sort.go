package reportsview

import (
	"cmp"
	"slices"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/textutil"
)

func sortRows(rows []Row, sort Sort, dir listview.Dir) {
	slices.SortStableFunc(rows, func(a, b Row) int {
		missingA, missingB := missing(a, sort), missing(b, sort)
		if missingA != missingB {
			if missingA {
				return 1
			}
			return -1
		}
		if !missingA {
			c := comparePrimary(a, b, sort)
			if dir != listview.DirAsc {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		if c := b.Date.Compare(a.Date); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func missing(row Row, sort Sort) bool {
	switch sort {
	case SortKind:
		return row.KindLabel == ""
	case SortProvider:
		return row.Provider == ""
	}
	return row.Date.IsZero()
}

func comparePrimary(a, b Row, sort Sort) int {
	switch sort {
	case SortKind:
		return textutil.CompareNatural(a.KindLabel, b.KindLabel)
	case SortProvider:
		return textutil.CompareNatural(a.Provider, b.Provider)
	}
	return a.Date.Compare(b.Date)
}
