package visitsview

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/textutil"
)

func sortRows(rows []Row, key Sort, dir listview.Dir) {
	slices.SortStableFunc(rows, func(a, b Row) int {
		if c := comparePrimary(key, dir, a, b); c != 0 {
			return c
		}
		if c := compareTimes(a.PrescribedOn, b.PrescribedOn, listview.DirDesc); c != 0 {
			return c
		}
		return strings.Compare(strings.ToUpper(a.NRE), strings.ToUpper(b.NRE))
	})
}

func comparePrimary(key Sort, dir listview.Dir, a, b Row) int {
	switch key {
	case SortTitle:
		return listview.Directed(textutil.CompareNatural(a.Title, b.Title), dir)
	case SortStatus:
		return listview.Directed(cmp.Compare(statusRank(a.Status.Key), statusRank(b.Status.Key)), dir)
	case SortNRE:
		return listview.Directed(textutil.CompareNatural(a.NRE, b.NRE), dir)
	case SortAppointment:
		return compareAppointment(a, b, dir)
	case SortPlace:
		return comparePlace(a, b, dir)
	}
	return compareTimes(a.PrescribedOn, b.PrescribedOn, dir)
}

func compareTimes(a, b time.Time, dir listview.Dir) int {
	if a.IsZero() || b.IsZero() {
		return listview.MissingLast(a.IsZero(), b.IsZero())
	}
	return listview.Directed(a.Compare(b), dir)
}

func compareAppointment(a, b Row, dir listview.Dir) int {
	var at, bt time.Time
	if a.Appointment != nil {
		at = a.Appointment.At
	}
	if b.Appointment != nil {
		bt = b.Appointment.At
	}
	return compareTimes(at, bt, dir)
}

func comparePlace(a, b Row, dir listview.Dir) int {
	var an, bn string
	if a.Appointment != nil {
		an = a.Appointment.Unit.DisplayName
	}
	if b.Appointment != nil {
		bn = b.Appointment.Unit.DisplayName
	}
	if an == "" || bn == "" {
		return listview.MissingLast(an == "", bn == "")
	}
	return listview.Directed(textutil.CompareNatural(an, bn), dir)
}
