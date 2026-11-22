package visitsview

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

const (
	n1 = "0900A0000000001"
	n2 = "0900A0000000002"
	n3 = "0900A0000000003"
	n4 = "0900A0000000004"
	n5 = "0900A0000000005"
	n6 = "0900A0000000006"
	n7 = "0900A0000000007"
)

func baseRows() []agenda.Row {
	cupOnly := agenda.Row{NRE: n6, CUP: &cup.Prescription{
		NRE: n6, Kind: "visit", Status: 3, Items: []string{"Ecografia esempio", "Esame sangue"}, PrescribedOn: at("2026-11-05T09:00:00Z"),
	}}
	nullItems := agenda.Row{NRE: n7, CUP: &cup.Prescription{NRE: n7, Kind: "visit", Status: 3}}
	return []agenda.Row{
		fseRow(n1, fse.KindVisit, 3, "2026-11-04T09:00:00Z", "Visita esempio"),
		fseRow(n2, fse.KindVisit, 5, "2026-11-01T09:00:00Z", "Visita esempio"),
		fseRow(n3, fse.KindVisit, 8, "2026-10-01T09:00:00Z", "Visita esempio"),
		fseRow(n4, fse.KindVisit, 4, "2026-09-01T09:00:00Z", "Visita esempio"),
		fseRow(n5, fse.KindDrug, 5, "2026-11-02T09:00:00Z", "Farmaco"),
		cupOnly,
		nullItems,
	}
}

func query(t *testing.T, f *fixture, q Query) Page {
	t.Helper()
	page, err := f.svc.Query(context.Background(), q)
	require.NoError(t, err)
	return page
}

func TestQueryRows(t *testing.T) {
	t.Run("excludes drug prescriptions and keeps cup only rows", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{ExcludeStatuses: []string{}})

		assert.ElementsMatch(t, []string{n1, n2, n3, n4, n6, n7}, nres(page.Rows))
		assert.Equal(t, 6, page.Total)
	})

	t.Run("a row only known to the lookups is not added", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n2: checked(booking(n2, "PRENOTATO", "", "U"))}))

		page := query(t, f, Query{})

		assert.Equal(t, []string{n1}, nres(page.Rows))
	})

	t.Run("a live booking makes the row Prenotata with its appointment", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n2: checked(booking(n2, "PRENOTATO", "2026-12-01T09:00:00Z", "Unita esempio"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))

		page := query(t, f, Query{Search: n2})

		require.Len(t, page.Rows, 1)
		assert.Equal(t, "Prenotata", page.Rows[0].Status.Label)
		require.NotNil(t, page.Rows[0].Appointment)
		assert.Equal(t, "tra 21 giorni", page.Rows[0].Appointment.RelativeDay)
		assert.Equal(t, LookupDone, page.Rows[0].AppointmentLookup)
		assert.Equal(t, at("2026-11-10T08:00:00Z"), page.Rows[0].CheckedAt.Appointments)
	})

	t.Run("upper-cases the row NRE for the lookup", func(t *testing.T) {
		rows := []agenda.Row{fseRow("0900a0000000009", fse.KindVisit, 5, "2026-11-01T09:00:00Z", "x")}
		lookups := map[string]agenda.Lookup{"0900A0000000009": checked(booking("0900A0000000009", "PRENOTATO", "", "U"))}
		f := newFixture(freshSnapshot(rows, lookups))

		page := query(t, f, Query{})

		assert.Equal(t, "Prenotata", page.Rows[0].Status.Label)
	})

	t.Run("a DISDETTO booking keeps the official state and shows no appointment", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n1: checked(booking(n1, "DISDETTO", "2026-12-01T09:00:00Z", "U"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))

		page := query(t, f, Query{Search: n1})

		assert.Equal(t, "Da prenotare", page.Rows[0].Status.Label)
		assert.Nil(t, page.Rows[0].Appointment)
	})

	t.Run("a manual status replaces only the status and keeps the appointment", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n2: checked(booking(n2, "PRENOTATO", "2026-12-01T09:00:00Z", "Unita esempio"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))
		f.local.OverridesFunc = func(context.Context) ([]localrecords.Override, error) {
			return []localrecords.Override{{NRE: n2, Status: 8}}, nil
		}

		page := query(t, f, Query{ExcludeStatuses: []string{}, Search: n2})

		row := page.Rows[0]
		assert.Equal(t, "Erogata", row.Status.Label)
		assert.True(t, row.Status.Manual)
		require.NotNil(t, row.Appointment)
		assert.Equal(t, "Unita esempio", row.Appointment.Unit.DisplayName)
	})

	t.Run("a manual status row still sorts by its appointment", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{
			n1: checked(booking(n1, "PRENOTATO", "2026-12-05T09:00:00Z", "U")),
			n2: checked(booking(n2, "PRENOTATO", "2026-12-01T09:00:00Z", "U")),
		}
		f := newFixture(freshSnapshot(baseRows()[:3], lookups))
		f.local.OverridesFunc = func(context.Context) ([]localrecords.Override, error) {
			return []localrecords.Override{{NRE: n2, Status: 3}}, nil
		}

		page := query(t, f, Query{Sort: SortAppointment, Dir: listview.DirAsc})

		assert.Equal(t, []string{n2, n1}, nres(page.Rows))
	})

	t.Run("groups unknown codes under other", func(t *testing.T) {
		rows := []agenda.Row{
			fseRow(n1, fse.KindVisit, 99, "2026-11-01T09:00:00Z", "x"),
			fseRow(n2, fse.KindVisit, 98, "2026-11-01T09:00:00Z", "x"),
			fseRow(n3, fse.KindVisit, 8, "2026-11-01T09:00:00Z", "x"),
		}
		f := newFixture(freshSnapshot(rows, nil))

		page := query(t, f, Query{ExcludeStatuses: []string{}})

		assert.Equal(t, []listview.StatusOption{
			{Key: "done", Label: "Erogata", Tone: listview.ToneDone, Count: 1},
			{Key: "other", Label: "Altro stato", Tone: listview.ToneClosed, Count: 2},
		}, page.Statuses)
		labels := []string{page.Rows[0].Status.Label, page.Rows[1].Status.Label, page.Rows[2].Status.Label}
		assert.ElementsMatch(t, []string{"Stato 99", "Stato 98", "Erogata"}, labels)
	})

	t.Run("row carries the document ref, upload defaults and linked reports", func(t *testing.T) {
		rows := baseRows()[:1]
		rows[0].FSE.DocumentRef = "ref-1"
		lookups := map[string]agenda.Lookup{n1: checked(booking(n1, "PRENOTATO", "2026-11-12T23:30:00Z", "Unita esempio"))}
		f := newFixture(freshSnapshot(rows, lookups))
		ref := referti.Ref{Kind: referti.KindLab, AbstractID: "a1"}
		f.reports.CachedListFunc = func(context.Context) (reports.CachedListing, error) {
			return reports.CachedListing{Listing: reports.Listing{Reports: []reports.Report{
				{Report: referti.Report{Ref: ref, Provider: "Ospedale"}, Origin: reports.OriginRemote, NREs: []string{n1}},
				{Report: referti.Report{Ref: referti.Ref{Kind: referti.KindLocal, AbstractID: "l1"}}, Origin: reports.OriginLocal, Title: "Mio", NREs: []string{"other"}},
			}}}, nil
		}

		row := query(t, f, Query{}).Rows[0]

		assert.Equal(t, "ref-1", row.DocumentRef)
		assert.Equal(t, UploadDefaults{Title: "Visita esempio", Date: "2026-11-13", Provider: "Unita esempio"}, row.Upload)
		require.Len(t, row.Reports, 1)
		assert.Equal(t, LinkedReport{Ref: ref, Label: "Laboratorio analisi", Provider: "Ospedale", Origin: reports.OriginRemote, Available: true}, row.Reports[0])
	})

	t.Run("upload defaults without an appointment use today in Rome and no provider", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], nil))

		row := query(t, f, Query{}).Rows[0]

		assert.Equal(t, UploadDefaults{Title: "Visita esempio", Date: "2026-11-10"}, row.Upload)
		assert.Equal(t, []LinkedReport{}, row.Reports)
	})
}

func TestQueryExcludeAndCounts(t *testing.T) {
	t.Run("nil excludeStatuses hides the defaults", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{})

		assert.ElementsMatch(t, []string{n1, n2, n6, n7}, nres(page.Rows))
		assert.Equal(t, []string{"done", "done_after_cancel", "cancelled", "expired"}, page.ExcludedStatuses)
	})

	t.Run("empty excludeStatuses shows everything", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{ExcludeStatuses: []string{}})

		assert.Len(t, page.Rows, 6)
		assert.Empty(t, page.ExcludedStatuses)
	})

	t.Run("explicit excludeStatuses is applied as given", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{ExcludeStatuses: []string{"prescribed"}})

		assert.ElementsMatch(t, []string{n2, n3, n4}, nres(page.Rows))
	})

	t.Run("a booked row with a done fse status is not hidden", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n3: checked(booking(n3, "PRENOTATO", "2026-12-01T09:00:00Z", "U"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))

		page := query(t, f, Query{Search: n3})

		assert.Equal(t, []string{n3}, nres(page.Rows))
	})

	t.Run("a DISDETTO booking on a done row stays hidden", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n3: checked(booking(n3, "DISDETTO", "2026-12-01T09:00:00Z", "U"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))

		page := query(t, f, Query{Search: n3})

		assert.Empty(t, page.Rows)
	})

	t.Run("counts follow the display order and flag excluded options", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n2: checked(booking(n2, "PRENOTATO", "2026-11-12T14:00:00Z", "U"))}
		expired := expiring(fseRow("0900A0000000056", fse.KindVisit, 3, "2026-11-01T09:00:00Z", "x"), "2026-11-09T09:00:00Z", false)
		f := newFixture(freshSnapshot(append(baseRows(), expired), lookups))

		page := query(t, f, Query{})

		assert.Equal(t, []listview.StatusOption{
			{Key: "booked", Label: "Prenotata", Tone: listview.ToneBooked, Count: 1},
			{Key: "prescribed", Label: "Da prenotare", Tone: listview.ToneToBook, Count: 3},
			{Key: "done", Label: "Erogata", Tone: listview.ToneDone, Count: 1, Excluded: true},
			{Key: "cancelled", Label: "Annullata", Tone: listview.ToneClosed, Count: 1, Excluded: true},
			{Key: "expired", Label: "Scaduta", Tone: listview.ToneClosed, Count: 1, Excluded: true},
		}, page.Statuses)
	})

	t.Run("lists unverified after prescribed and handled after unverified", func(t *testing.T) {
		rows := []agenda.Row{
			fseRow(n1, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "x"),
			fseRow(n2, fse.KindVisit, 30, "2026-11-01T09:00:00Z", "x"),
			fseRow(n3, fse.KindVisit, 30, "2026-11-01T09:00:00Z", "x"),
		}
		unbooked3 := checked()
		unbooked3.Verdict = verdict(cup.VerdictHandled, "m")
		lookups := map[string]agenda.Lookup{n2: checked(), n3: unbooked3}
		f := newFixture(freshSnapshot(rows, lookups))

		page := query(t, f, Query{})

		keys := make([]string, 0, len(page.Statuses))
		for _, s := range page.Statuses {
			keys = append(keys, s.Key)
		}
		assert.Equal(t, []string{"prescribed", "unverified", "handled"}, keys)
	})

	t.Run("counts ignore the exclusion but follow the search", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{Search: "Ecografia"})

		require.Len(t, page.Statuses, 1)
		assert.Equal(t, 1, page.Statuses[0].Count)
	})
}

func TestQuerySearch(t *testing.T) {
	t.Run("searches the title case-insensitively", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		assert.Equal(t, []string{n6}, nres(query(t, f, Query{Search: "ECOGRAFIA"}).Rows))
	})

	t.Run("searches the NRE", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		assert.Equal(t, []string{n2}, nres(query(t, f, Query{Search: "0900a0000000002"}).Rows))
	})

	t.Run("ignores accents", func(t *testing.T) {
		rows := []agenda.Row{fseRow(n1, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "Visita pediatrica")}
		f := newFixture(freshSnapshot(rows, nil))

		assert.Len(t, query(t, f, Query{Search: "pédiatrica"}).Rows, 1)
	})

	t.Run("searches the unit name and city of the appointment", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n2: checked(booking(n2, "PRENOTATO", "", "Unita esempio"))}
		f := newFixture(freshSnapshot(baseRows(), lookups))

		assert.Equal(t, []string{n2}, nres(query(t, f, Query{Search: "firenze"}).Rows))
		assert.Equal(t, []string{n2}, nres(query(t, f, Query{Search: "UNITA"}).Rows))
	})

	t.Run("ignores a blank search", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{Search: "   "})

		assert.Len(t, page.Rows, 4)
	})

	t.Run("handles a row without items", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		assert.Empty(t, query(t, f, Query{Search: "x"}).Rows)
	})
}

func TestQuerySort(t *testing.T) {
	soon := fseRow("0900A0000000030", fse.KindVisit, 30, "2026-10-01T09:00:00Z", "S")
	later := fseRow("0900A0000000031", fse.KindVisit, 30, "2026-10-02T09:00:00Z", "S")
	past := fseRow("0900A0000000032", fse.KindVisit, 30, "2026-10-20T09:00:00Z", "S")
	plain := fseRow(n1, fse.KindVisit, 3, "2026-11-04T09:00:00Z", "S")
	plain2 := fseRow(n2, fse.KindVisit, 5, "2026-11-01T09:00:00Z", "S")
	appointmentRows := []agenda.Row{plain, past, later, plain2, soon}
	appointmentLookups := map[string]agenda.Lookup{
		"0900A0000000030": checked(booking("0900A0000000030", "PRENOTATO", "2026-11-11T09:00:00Z", "U")),
		"0900A0000000031": checked(booking("0900A0000000031", "PRENOTATO", "2026-12-01T09:00:00Z", "U")),
		"0900A0000000032": checked(booking("0900A0000000032", "PRENOTATO", "2026-11-01T09:00:00Z", "U")),
	}

	t.Run("appointment asc sorts past and future by time, rows without one last", func(t *testing.T) {
		f := newFixture(freshSnapshot(appointmentRows, appointmentLookups))

		page := query(t, f, Query{Sort: SortAppointment, Dir: listview.DirAsc})

		assert.Equal(t, []string{"0900A0000000032", "0900A0000000030", "0900A0000000031", n1, n2}, nres(page.Rows))
	})

	t.Run("appointment desc keeps rows without one last", func(t *testing.T) {
		f := newFixture(freshSnapshot(appointmentRows, appointmentLookups))

		page := query(t, f, Query{Sort: SortAppointment, Dir: listview.DirDesc})

		assert.Equal(t, []string{"0900A0000000031", "0900A0000000030", "0900A0000000032", n1, n2}, nres(page.Rows))
	})

	t.Run("undated appointment rows order by prescription date, undated last", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{Sort: SortAppointment, Dir: listview.DirAsc})

		assert.Equal(t, []string{n6, n1, n2, n7}, nres(page.Rows))
	})

	t.Run("prescribed sorts by date with the NRE tie-break", func(t *testing.T) {
		rows := []agenda.Row{
			fseRow(n2, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "x"),
			fseRow(n1, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "x"),
			fseRow(n3, fse.KindVisit, 3, "2026-11-03T09:00:00Z", "x"),
		}
		f := newFixture(freshSnapshot(rows, nil))

		assert.Equal(t, []string{n3, n1, n2}, nres(query(t, f, Query{Sort: SortPrescribed, Dir: listview.DirDesc}).Rows))
		assert.Equal(t, []string{n1, n2, n3}, nres(query(t, f, Query{Sort: SortPrescribed, Dir: listview.DirAsc}).Rows))
	})

	t.Run("title sorts with the date desc tie-break and keeps it on desc", func(t *testing.T) {
		a := fseRow(n1, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "Alfa")
		b := fseRow(n2, fse.KindVisit, 3, "2026-11-03T09:00:00Z", "Alfa")
		c := fseRow(n3, fse.KindVisit, 5, "2026-11-02T09:00:00Z", "Beta")
		f := newFixture(freshSnapshot([]agenda.Row{c, a, b}, nil))

		assert.Equal(t, []string{n2, n1, n3}, nres(query(t, f, Query{Sort: SortTitle, Dir: listview.DirAsc}).Rows))
		assert.Equal(t, []string{n3, n2, n1}, nres(query(t, f, Query{Sort: SortTitle, Dir: listview.DirDesc}).Rows))
	})

	t.Run("nre sorts naturally", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:3], nil))

		assert.Equal(t, []string{n1, n2}, nres(query(t, f, Query{Sort: SortNRE, Dir: listview.DirAsc}).Rows))
		assert.Equal(t, []string{n2, n1}, nres(query(t, f, Query{Sort: SortNRE, Dir: listview.DirDesc}).Rows))
	})

	t.Run("status sorts by display order, not alphabetically", func(t *testing.T) {
		rows := []agenda.Row{
			fseRow(n1, fse.KindVisit, 5, "2026-11-01T09:00:00Z", "x"),
			fseRow(n2, fse.KindVisit, 3, "2026-11-02T09:00:00Z", "x"),
			fseRow(n3, fse.KindVisit, 6, "2026-11-03T09:00:00Z", "x"),
			fseRow(n4, fse.KindVisit, 7, "2026-11-04T09:00:00Z", "x"),
		}
		f := newFixture(freshSnapshot(rows, nil))

		asc := query(t, f, Query{Sort: SortStatus, Dir: listview.DirAsc})
		desc := query(t, f, Query{Sort: SortStatus, Dir: listview.DirDesc})

		assert.Equal(t, []string{n2, n1, n4, n3}, nres(asc.Rows))
		assert.Equal(t, []string{n3, n4, n1, n2}, nres(desc.Rows))
	})

	t.Run("place sorts by unit, rows without an appointment last in both directions", func(t *testing.T) {
		rows := baseRows()[:3]
		lookups := map[string]agenda.Lookup{
			n1: checked(booking(n1, "PRENOTATO", "", "Zeta")),
			n2: checked(booking(n2, "PRENOTATO", "", "Alfa")),
		}
		f := newFixture(freshSnapshot(rows, lookups))

		asc := query(t, f, Query{Sort: SortPlace, Dir: listview.DirAsc, ExcludeStatuses: []string{}})
		desc := query(t, f, Query{Sort: SortPlace, Dir: listview.DirDesc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{n2, n1, n3}, nres(asc.Rows))
		assert.Equal(t, []string{n1, n2, n3}, nres(desc.Rows))
	})

	t.Run("does not depend on the snapshot order", func(t *testing.T) {
		rows := baseRows()
		f := newFixture(freshSnapshot(rows, nil))
		before := nres(query(t, f, Query{Sort: SortNRE, Dir: listview.DirAsc}).Rows)
		f.agenda.SnapshotFunc = func(context.Context) (agenda.Snapshot, error) {
			reversed := []agenda.Row{rows[6], rows[5], rows[4], rows[3], rows[2], rows[1], rows[0]}
			return freshSnapshot(reversed, nil), nil
		}

		assert.Equal(t, before, nres(query(t, f, Query{Sort: SortNRE, Dir: listview.DirAsc}).Rows))
	})
}

func manyRows(count int) []agenda.Row {
	rows := make([]agenda.Row, 0, count)
	for i := range count {
		nre := "0900B00000" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + "00"
		rows = append(rows, fseRow(nre, fse.KindVisit, 3, "2026-11-01T09:00:00Z", "x"))
	}
	return rows
}

func TestQueryPaging(t *testing.T) {
	t.Run("pages are stable and cover every row once", func(t *testing.T) {
		f := newFixture(freshSnapshot(manyRows(60), nil))

		first := query(t, f, Query{Page: 1, PageSize: 25})
		second := query(t, f, Query{Page: 2, PageSize: 25})
		third := query(t, f, Query{Page: 3, PageSize: 25})

		assert.Equal(t, 60, first.Total)
		assert.Len(t, first.Rows, 25)
		assert.Len(t, second.Rows, 25)
		assert.Len(t, third.Rows, 10)
		all := append(append(nres(first.Rows), nres(second.Rows)...), nres(third.Rows)...)
		assert.Len(t, all, 60)
		assert.Len(t, map[string]bool{all[0]: true, all[59]: true}, 2)
	})

	t.Run("clamps a page past the end to the last page", func(t *testing.T) {
		f := newFixture(freshSnapshot(manyRows(60), nil))

		page := query(t, f, Query{Page: 99, PageSize: 25})

		assert.Equal(t, 3, page.Page)
		assert.Len(t, page.Rows, 10)
	})

	t.Run("defaults to page 1 of 50", func(t *testing.T) {
		f := newFixture(freshSnapshot(manyRows(60), nil))

		page := query(t, f, Query{})

		assert.Equal(t, 1, page.Page)
		assert.Equal(t, 50, page.PageSize)
		assert.Len(t, page.Rows, 50)
	})

	t.Run("an empty result is page 1 with no rows", func(t *testing.T) {
		f := newFixture(freshSnapshot(nil, nil))

		page := query(t, f, Query{Page: 5})

		assert.Equal(t, 1, page.Page)
		assert.Equal(t, 0, page.Total)
		assert.Equal(t, []Row{}, page.Rows)
	})
}

func TestQueryFocus(t *testing.T) {
	t.Run("a hidden row is un-excluded, search cleared and its page returned", func(t *testing.T) {
		rows := append(manyRows(30), fseRow(n3, fse.KindVisit, 8, "2026-01-01T09:00:00Z", "Visita fatta"))
		f := newFixture(freshSnapshot(rows, nil))

		page := query(t, f, Query{Search: "nonexistent", PageSize: 25, Focus: n3, Sort: SortPrescribed, Dir: listview.DirDesc})

		require.NotNil(t, page.Focus)
		assert.Equal(t, listview.Focus{NRE: n3, Found: true}, *page.Focus)
		assert.Empty(t, page.Search)
		assert.NotContains(t, page.ExcludedStatuses, "done")
		assert.Contains(t, page.ExcludedStatuses, "cancelled")
		assert.Equal(t, 2, page.Page)
		assert.Contains(t, nres(page.Rows), n3)
	})

	t.Run("a visible row keeps the search and exclusions", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{Search: "Ecografia", Focus: n6})

		assert.Equal(t, "Ecografia", page.Search)
		assert.Equal(t, []string{"done", "done_after_cancel", "cancelled", "expired"}, page.ExcludedStatuses)
		assert.True(t, page.Focus.Found)
	})

	t.Run("an unknown NRE reports found false and changes nothing", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		page := query(t, f, Query{Search: "Ecografia", Focus: "0900Z0000000009"})

		assert.Equal(t, listview.Focus{NRE: "0900Z0000000009"}, *page.Focus)
		assert.Equal(t, "Ecografia", page.Search)
	})

	t.Run("no focus leaves the field nil", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		assert.Nil(t, query(t, f, Query{}).Focus)
	})
}

func TestQueryRefreshing(t *testing.T) {
	t.Run("fresh data does not kick", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n1: checked(booking(n1, "PRENOTATO", "", "U"))}
		f := newFixture(freshSnapshot(baseRows()[:1], lookups))

		query(t, f, Query{})

		assert.Empty(t, f.refresher.KickCalls())
	})

	t.Run("a stale list kicks the visits job", func(t *testing.T) {
		snap := freshSnapshot(baseRows()[:1], nil)
		snap.FSEList.Stale = true
		f := newFixture(snap)

		query(t, f, Query{})

		require.Len(t, f.refresher.KickCalls(), 1)
		assert.Equal(t, refresh.JobVisits, f.refresher.KickCalls()[0].Job)
	})

	t.Run("a missing list kicks", func(t *testing.T) {
		snap := freshSnapshot(nil, nil)
		snap.CUPList = agenda.ListState{Missing: true}
		f := newFixture(snap)

		query(t, f, Query{})

		assert.Len(t, f.refresher.KickCalls(), 1)
	})

	t.Run("an unchecked candidate kicks and reads as checking", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: {Candidate: true}}))

		page := query(t, f, Query{})

		assert.Len(t, f.refresher.KickCalls(), 1)
		assert.Equal(t, LookupChecking, page.Rows[0].AppointmentLookup)
	})

	t.Run("a stale lookup kicks", func(t *testing.T) {
		stale := checked(booking(n1, "PRENOTATO", "", "U"))
		stale.Stale = true
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: stale}))

		query(t, f, Query{})

		assert.Len(t, f.refresher.KickCalls(), 1)
	})

	t.Run("an unbooked lookup without a fresh verdict kicks", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: checked()}))

		query(t, f, Query{})

		assert.Len(t, f.refresher.KickCalls(), 1)
	})

	t.Run("an unbooked lookup with a stale verdict kicks", func(t *testing.T) {
		lookup := checked()
		lookup.Verdict = verdict(cup.VerdictBookable, "m")
		lookup.Verdict.Stale = true
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: lookup}))

		query(t, f, Query{})

		assert.Len(t, f.refresher.KickCalls(), 1)
	})

	t.Run("a non-candidate lookup never kicks", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: {}}))

		page := query(t, f, Query{})

		assert.Empty(t, f.refresher.KickCalls())
		assert.Equal(t, LookupNone, page.Rows[0].AppointmentLookup)
	})

	t.Run("refreshing follows the active visits job and flags unchecked rows", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n1: {Candidate: true}, n2: checked(booking(n2, "PRENOTATO", "", "U"))}
		f := newFixture(freshSnapshot(baseRows()[:2], lookups))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobVisits}}
		}

		page := query(t, f, Query{})

		assert.True(t, page.Refreshing)
		byNRE := map[string]Row{}
		for _, r := range page.Rows {
			byNRE[r.NRE] = r
		}
		assert.True(t, byNRE[n1].Refreshing)
		assert.False(t, byNRE[n2].Refreshing)
	})

	t.Run("a reports-only job does not make the view refreshing", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], nil))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobReports}}
		}

		assert.False(t, query(t, f, Query{}).Refreshing)
	})

	t.Run("a forced NRE refreshes that row and the view", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:2], nil))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Active: true, NREs: []string{"0900a0000000002"}}
		}

		page := query(t, f, Query{})

		assert.True(t, page.Refreshing)
		for _, r := range page.Rows {
			assert.Equal(t, r.NRE == n2, r.Refreshing)
		}
	})

	t.Run("a failed lookup reads as failed", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: {Candidate: true}}))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Failed: map[string]refresh.Issue{n1: refresh.IssueUnavailable}}
		}

		assert.Equal(t, LookupFailed, query(t, f, Query{}).Rows[0].AppointmentLookup)
	})
}

func TestQueryUpdatedAndIssues(t *testing.T) {
	t.Run("updated is the oldest of the two lists", func(t *testing.T) {
		snap := freshSnapshot(baseRows()[:1], nil)
		snap.CUPList.At = at("2026-11-10T07:30:00Z")
		snap.FSEList.At = at("2026-11-09T07:30:00Z")
		f := newFixture(snap)

		page := query(t, f, Query{})

		assert.Equal(t, at("2026-11-09T07:30:00Z"), page.Updated.At)
		assert.Equal(t, "Aggiornato il 09/11 08:30", page.Updated.Label)
	})

	t.Run("a missing list is ignored for updated", func(t *testing.T) {
		snap := freshSnapshot(baseRows()[:1], nil)
		snap.CUPList = agenda.ListState{Missing: true}
		f := newFixture(snap)

		assert.Equal(t, at("2026-11-10T08:30:00Z"), query(t, f, Query{}).Updated.At)
	})

	t.Run("no lists at all gives an empty label", func(t *testing.T) {
		snap := agenda.Snapshot{CUPList: agenda.ListState{Missing: true}, FSEList: agenda.ListState{Missing: true}}
		f := newFixture(snap)

		page := query(t, f, Query{})

		assert.True(t, page.Updated.At.IsZero())
		assert.Empty(t, page.Updated.Label)
	})

	t.Run("lists issue only when no list is cached", func(t *testing.T) {
		snap := agenda.Snapshot{CUPList: agenda.ListState{Missing: true}, FSEList: agenda.ListState{Missing: true}}
		f := newFixture(snap)
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Issues: refresh.Issues{Lists: refresh.IssueLoginRequired}}
		}

		assert.Equal(t, refresh.IssueLoginRequired, query(t, f, Query{}).Issues.Lists)
	})

	t.Run("lists issue is none when cached data exists", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], nil))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Issues: refresh.Issues{Lists: refresh.IssueUnavailable}}
		}

		assert.Equal(t, refresh.IssueNone, query(t, f, Query{}).Issues.Lists)
	})

	t.Run("appointments issue shows when no lookup succeeded", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows()[:1], map[string]agenda.Lookup{n1: {Candidate: true}}))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Issues: refresh.Issues{Appointments: refresh.IssueGestioneRequired}}
		}

		assert.Equal(t, refresh.IssueGestioneRequired, query(t, f, Query{}).Issues.Appointments)
	})

	t.Run("appointments issue is none once a lookup succeeded", func(t *testing.T) {
		lookups := map[string]agenda.Lookup{n1: checked(booking(n1, "PRENOTATO", "", "U"))}
		f := newFixture(freshSnapshot(baseRows()[:1], lookups))
		f.refresher.StatusFunc = func() refresh.Status {
			return refresh.Status{Issues: refresh.Issues{Appointments: refresh.IssueUnavailable}}
		}

		assert.Equal(t, refresh.IssueNone, query(t, f, Query{}).Issues.Appointments)
	})
}

func TestQueryErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("a snapshot error is wrapped", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})
		f.agenda.SnapshotFunc = func(context.Context) (agenda.Snapshot, error) { return agenda.Snapshot{}, boom }

		_, err := f.svc.Query(context.Background(), Query{})

		require.ErrorIs(t, err, boom)
	})

	t.Run("a reports error is wrapped", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})
		f.reports.CachedListFunc = func(context.Context) (reports.CachedListing, error) { return reports.CachedListing{}, boom }

		_, err := f.svc.Query(context.Background(), Query{})

		require.ErrorIs(t, err, boom)
	})

	t.Run("an overrides error is wrapped", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})
		f.local.OverridesFunc = func(context.Context) ([]localrecords.Override, error) { return nil, boom }

		_, err := f.svc.Query(context.Background(), Query{})

		require.ErrorIs(t, err, boom)
	})

	t.Run("each query reads each source once", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		query(t, f, Query{})

		assert.Len(t, f.agenda.SnapshotCalls(), 1)
		assert.Len(t, f.reports.CachedListCalls(), 1)
		assert.Len(t, f.local.OverridesCalls(), 1)
	})
}

func TestRow(t *testing.T) {
	t.Run("returns a row ignoring the default filters", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		result, err := f.svc.Row(context.Background(), n3)

		require.NoError(t, err)
		assert.Equal(t, n3, result.Row.NRE)
		assert.Equal(t, "Erogata", result.Row.Status.Label)
	})

	t.Run("matches the NRE case-insensitively", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		result, err := f.svc.Row(context.Background(), "0900a0000000001")

		require.NoError(t, err)
		assert.Equal(t, n1, result.Row.NRE)
	})

	t.Run("an unknown NRE is not found", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		_, err := f.svc.Row(context.Background(), "0900Z0000000009")

		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("a drug NRE is not found", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		_, err := f.svc.Row(context.Background(), n5)

		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("manual status lists the selectable options and the current override", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))
		f.local.OverridesFunc = func(context.Context) ([]localrecords.Override, error) {
			return []localrecords.Override{{NRE: n1, Status: 8}}, nil
		}

		result, err := f.svc.Row(context.Background(), n1)

		require.NoError(t, err)
		require.NotNil(t, result.ManualStatus.Current)
		assert.Equal(t, 8, *result.ManualStatus.Current)
		require.Len(t, result.ManualStatus.Options, len(cup.SelectableStatuses))
		assert.Equal(t, ManualOption{Code: 3, Label: "Da prenotare"}, result.ManualStatus.Options[0])
		assert.Equal(t, ManualOption{Code: 30, Label: "Prenotata"}, result.ManualStatus.Options[1])
	})

	t.Run("current is nil without an override", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))

		result, err := f.svc.Row(context.Background(), n1)

		require.NoError(t, err)
		assert.Nil(t, result.ManualStatus.Current)
	})

	t.Run("refreshing follows the row", func(t *testing.T) {
		f := newFixture(freshSnapshot(baseRows(), nil))
		f.refresher.StatusFunc = func() refresh.Status { return refresh.Status{Active: true, NREs: []string{n1}} }

		result, err := f.svc.Row(context.Background(), n1)

		require.NoError(t, err)
		assert.True(t, result.Refreshing)
	})

	t.Run("a snapshot error is returned", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})
		boom := errors.New("boom")
		f.agenda.SnapshotFunc = func(context.Context) (agenda.Snapshot, error) { return agenda.Snapshot{}, boom }

		_, err := f.svc.Row(context.Background(), n1)

		require.ErrorIs(t, err, boom)
	})
}

func TestReportOptions(t *testing.T) {
	linked := reports.Report{Report: referti.Report{Ref: referti.Ref{Kind: referti.KindLab, AbstractID: "a1"}}, NREs: []string{n1}}
	older := reports.Report{Report: referti.Report{Ref: referti.Ref{Kind: referti.KindRadiology, AbstractID: "a2"}, Date: at("2026-01-01T09:00:00Z"), Provider: "P1"}, Origin: reports.OriginRemote}
	newer := reports.Report{Report: referti.Report{Ref: referti.Ref{Kind: referti.KindLocal, AbstractID: "l1"}, Date: at("2026-05-01T09:00:00Z"), Provider: "P2"}, Origin: reports.OriginLocal, Title: "Mio referto"}
	undated := reports.Report{Report: referti.Report{Ref: referti.Ref{Kind: referti.KindER, AbstractID: "a3"}}}

	t.Run("lists unlinked reports newest first, undated last", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})
		f.reports.CachedListFunc = func(context.Context) (reports.CachedListing, error) {
			return reports.CachedListing{Listing: reports.Listing{Reports: []reports.Report{undated, linked, older, newer}}}, nil
		}

		options, err := f.svc.ReportOptions(context.Background(), "0900a0000000001")

		require.NoError(t, err)
		require.Len(t, options, 3)
		assert.Equal(t, ReportOption{Ref: newer.Ref, Label: "Mio referto", Date: newer.Date, Provider: "P2", Origin: reports.OriginLocal}, options[0])
		assert.Equal(t, "Radiologia", options[1].Label)
		assert.Equal(t, "Pronto soccorso", options[2].Label)
	})

	t.Run("a report linked to another visit stays offered", func(t *testing.T) {
		other := linked
		other.NREs = []string{n2}
		f := newFixture(agenda.Snapshot{})
		f.reports.CachedListFunc = func(context.Context) (reports.CachedListing, error) {
			return reports.CachedListing{Listing: reports.Listing{Reports: []reports.Report{other}}}, nil
		}

		options, err := f.svc.ReportOptions(context.Background(), n1)

		require.NoError(t, err)
		assert.Len(t, options, 1)
	})

	t.Run("no reports gives an empty list", func(t *testing.T) {
		f := newFixture(agenda.Snapshot{})

		options, err := f.svc.ReportOptions(context.Background(), n1)

		require.NoError(t, err)
		assert.Equal(t, []ReportOption{}, options)
	})

	t.Run("a reports error is wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		f := newFixture(agenda.Snapshot{})
		f.reports.CachedListFunc = func(context.Context) (reports.CachedListing, error) { return reports.CachedListing{}, boom }

		_, err := f.svc.ReportOptions(context.Background(), n1)

		require.ErrorIs(t, err, boom)
	})
}
