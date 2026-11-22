package drugsview

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/refresh"
)

const (
	nreA = "0900A0000000001"
	nreB = "0900A0000000002"
	nreC = "0900A0000000003"
	nreD = "0900A0000000004"
)

var errBoom = errors.New("boom")

func drug(nre, description string, status int, date time.Time) agenda.Row {
	return agenda.Row{NRE: nre, FSE: &fse.Prescription{NRE: nre, Description: description, Kind: fse.KindDrug, Status: status, Date: date, DocumentRef: "r"}}
}

var (
	day1 = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	day3 = time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
)

func rowA() agenda.Row { return drug(nreA, "Farmaco alfa", 3, day1) }
func rowB() agenda.Row { return drug(nreB, "Farmaco beta", 8, day3) }
func rowC() agenda.Row { return drug(nreC, "Farmaco gamma", 3, time.Time{}) }
func rowD() agenda.Row { return drug(nreD, "Farmaco delta", 3, day1) }

type fixture struct {
	snapshot  agenda.Snapshot
	overrides []localrecords.Override
	status    refresh.Status
	kicks     *MockRefresher
}

func newFixture(rows ...agenda.Row) *fixture {
	return &fixture{snapshot: agenda.Snapshot{Rows: rows, FSEList: agenda.ListState{At: day3}}}
}

func (f *fixture) service() *Service {
	ag := &MockAgenda{SnapshotFunc: func(context.Context) (agenda.Snapshot, error) { return f.snapshot, nil }}
	local := &MockLocal{OverridesFunc: func(context.Context) ([]localrecords.Override, error) { return f.overrides, nil }}
	f.kicks = &MockRefresher{KickFunc: func(refresh.Job) {}, StatusFunc: func() refresh.Status { return f.status }}
	svc := New(ag, local, f.kicks)
	svc.now = func() time.Time { return day3.Add(time.Hour) }
	return svc
}

func nres(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.NRE)
	}
	return out
}

func keys(options []listview.StatusOption) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, o.Key)
	}
	return out
}

func query(t *testing.T, f *fixture, q Query) Page {
	t.Helper()
	page, err := f.service().Query(t.Context(), q)
	require.NoError(t, err)
	return page
}

func TestStatusOf(t *testing.T) {
	t.Run("status 3 is Da ritirare", func(t *testing.T) {
		assert.Equal(t, Status{Key: "prescribed", Label: "Da ritirare", Tone: listview.ToneToBook}, statusOf(3))
	})

	t.Run("falls back to the shared prescription label", func(t *testing.T) {
		assert.Equal(t, "Erogata", statusOf(8).Label)
	})

	t.Run("unknown code is other with Stato N", func(t *testing.T) {
		assert.Equal(t, Status{Key: "other", Label: "Stato 99", Tone: listview.ToneClosed}, statusOf(99))
	})

	t.Run("maps tones of known statuses", func(t *testing.T) {
		assert.Equal(t, listview.ToneToBook, statusOf(3).Tone)
		assert.Equal(t, listview.ToneDone, statusOf(8).Tone)
		assert.Equal(t, listview.ToneDone, statusOf(9).Tone)
		assert.Equal(t, listview.TonePending, statusOf(7).Tone)
		assert.Equal(t, listview.ToneClosed, statusOf(4).Tone)
	})
}

func TestQueryRows(t *testing.T) {
	t.Run("keeps only drug rows with an fse prescription", func(t *testing.T) {
		f := newFixture(rowA(), agenda.Row{NRE: "0900A0000000099", CUP: &cup.Prescription{NRE: "0900A0000000099"}}, agenda.Row{NRE: "0900A0000000098", FSE: &fse.Prescription{Kind: fse.KindVisit}})

		page := query(t, f, Query{Sort: SortNRE, Dir: listview.DirAsc})

		assert.Equal(t, []string{nreA}, nres(page.Rows))
	})

	t.Run("maps title date status and document ref", func(t *testing.T) {
		f := newFixture(rowA())

		page := query(t, f, Query{Sort: SortDate})

		require.Len(t, page.Rows, 1)
		assert.Equal(t, Row{NRE: nreA, Title: "Farmaco alfa", Date: day1, Status: statusOf(3), DocumentRef: "r"}, page.Rows[0])
	})

	t.Run("empty description falls back to Farmaco", func(t *testing.T) {
		f := newFixture(drug(nreA, "", 3, day1))

		page := query(t, f, Query{Sort: SortDate})

		assert.Equal(t, "Farmaco", page.Rows[0].Title)
	})
}

func TestQuerySearch(t *testing.T) {
	t.Run("matches description case-insensitively", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB()), Query{Search: "ALFA", ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreA}, nres(page.Rows))
	})

	t.Run("matches NRE", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB()), Query{Search: "0000001", ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreA}, nres(page.Rows))
	})

	t.Run("matches everything on blank search", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{Search: "  ", ExcludeStatuses: []string{}})

		assert.Len(t, page.Rows, 3)
	})

	t.Run("rejects a non matching term", func(t *testing.T) {
		page := query(t, newFixture(rowA()), Query{Search: "zzz", ExcludeStatuses: []string{}})

		assert.Empty(t, page.Rows)
	})

	t.Run("applies the search term", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{Search: "beta", ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreB}, nres(page.Rows))
	})

	t.Run("echoes the search", func(t *testing.T) {
		page := query(t, newFixture(rowA()), Query{Search: "alfa"})

		assert.Equal(t, "alfa", page.Search)
	})
}

func TestQueryExclude(t *testing.T) {
	t.Run("nil exclusion applies the defaults", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{Sort: SortNRE, Dir: listview.DirAsc})

		assert.Equal(t, []string{nreA, nreC}, nres(page.Rows))
		assert.Equal(t, []string{"done", "done_after_cancel", "cancelled"}, page.ExcludedStatuses)
	})

	t.Run("empty exclusion keeps everything", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{ExcludeStatuses: []string{}})

		assert.Len(t, page.Rows, 3)
		assert.Empty(t, page.ExcludedStatuses)
	})

	t.Run("drops rows of an excluded status", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{ExcludeStatuses: []string{"prescribed"}})

		assert.Equal(t, []string{nreB}, nres(page.Rows))
	})
}

func TestQueryCounts(t *testing.T) {
	t.Run("counts rows per status in the fixed order", func(t *testing.T) {
		page := query(t, newFixture(rowB(), rowA(), rowC()), Query{})

		assert.Equal(t, []listview.StatusOption{
			{Key: "prescribed", Label: "Da ritirare", Tone: listview.ToneToBook, Count: 2},
			{Key: "done", Label: "Erogata", Tone: listview.ToneDone, Count: 1, Excluded: true},
		}, page.Statuses)
	})

	t.Run("unknown codes are grouped under other after the known ones", func(t *testing.T) {
		x := drug("0900A0000000010", "x", 99, day1)
		y := drug("0900A0000000011", "y", 50, day1)

		page := query(t, newFixture(x, y, rowB(), rowA()), Query{})

		assert.Equal(t, []string{"prescribed", "done", "other"}, keys(page.Statuses))
		assert.Equal(t, listview.StatusOption{Key: "other", Label: "Altro stato", Tone: listview.ToneClosed, Count: 2}, page.Statuses[2])
	})

	t.Run("counts follow the search but not the exclusion", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{Search: "beta"})

		assert.Equal(t, []string{"done"}, keys(page.Statuses))
		assert.Equal(t, 1, page.Statuses[0].Count)
	})

	t.Run("a row with an unknown code shows Stato N", func(t *testing.T) {
		page := query(t, newFixture(drug(nreA, "x", 99, day1)), Query{})

		assert.Equal(t, "Stato 99", page.Rows[0].Status.Label)
		assert.Equal(t, "other", page.Rows[0].Status.Key)
	})
}

func TestQuerySort(t *testing.T) {
	t.Run("date descending with missing last and NRE tie-break", func(t *testing.T) {
		page := query(t, newFixture(rowC(), rowA(), rowB(), rowD()), Query{Sort: SortDate, Dir: listview.DirDesc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreB, nreA, nreD, nreC}, nres(page.Rows))
	})

	t.Run("date ascending keeps missing last", func(t *testing.T) {
		page := query(t, newFixture(rowC(), rowB(), rowA()), Query{Sort: SortDate, Dir: listview.DirAsc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreA, nreB, nreC}, nres(page.Rows))
	})

	t.Run("name", func(t *testing.T) {
		page := query(t, newFixture(rowC(), rowA(), rowB()), Query{Sort: SortName, Dir: listview.DirAsc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreA, nreB, nreC}, nres(page.Rows))
	})

	t.Run("status uses the display order", func(t *testing.T) {
		page := query(t, newFixture(rowB(), rowA()), Query{Sort: SortStatus, Dir: listview.DirAsc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreA, nreB}, nres(page.Rows))
	})

	t.Run("status descending reverses the display order with NRE ascending ties", func(t *testing.T) {
		page := query(t, newFixture(rowC(), rowB(), rowA()), Query{Sort: SortStatus, Dir: listview.DirDesc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreB, nreA, nreC}, nres(page.Rows))
	})

	t.Run("nre descending", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB(), rowC()), Query{Sort: SortNRE, Dir: listview.DirDesc, ExcludeStatuses: []string{}})

		assert.Equal(t, []string{nreC, nreB, nreA}, nres(page.Rows))
	})

	t.Run("does not mutate the snapshot rows", func(t *testing.T) {
		f := newFixture(rowB(), rowA())

		query(t, f, Query{Sort: SortNRE, Dir: listview.DirAsc, ExcludeStatuses: []string{}})

		assert.Equal(t, nreB, f.snapshot.Rows[0].NRE)
	})
}

func TestQueryPaging(t *testing.T) {
	rows := make([]agenda.Row, 0, 5)
	for i := range 5 {
		rows = append(rows, drug(fmt.Sprintf("0900A000000010%d", i), "x", 3, day1))
	}

	t.Run("returns the requested page", func(t *testing.T) {
		page := query(t, newFixture(rows...), Query{Sort: SortNRE, Dir: listview.DirAsc, Page: 2, PageSize: 2})

		assert.Equal(t, []string{"0900A0000000102", "0900A0000000103"}, nres(page.Rows))
		assert.Equal(t, 2, page.Page)
		assert.Equal(t, 5, page.Total)
		assert.Equal(t, 2, page.PageSize)
	})

	t.Run("clamps a page past the end", func(t *testing.T) {
		page := query(t, newFixture(rows...), Query{Sort: SortNRE, Dir: listview.DirAsc, Page: 9, PageSize: 2})

		assert.Equal(t, 3, page.Page)
		assert.Equal(t, []string{"0900A0000000104"}, nres(page.Rows))
	})

	t.Run("an empty result is page one with no rows", func(t *testing.T) {
		page := query(t, newFixture(), Query{Page: 4})

		assert.Equal(t, 1, page.Page)
		assert.Empty(t, page.Rows)
	})

	t.Run("defaults page and page size", func(t *testing.T) {
		page := query(t, newFixture(rows...), Query{})

		assert.Equal(t, 1, page.Page)
		assert.Equal(t, listview.DefaultPageSize, page.PageSize)
	})
}

func TestQueryFocus(t *testing.T) {
	t.Run("un-excludes the status clears the search and returns the page of the row", func(t *testing.T) {
		f := newFixture(rowA(), rowB(), rowD())

		page := query(t, f, Query{Search: "zzz", Sort: SortNRE, Dir: listview.DirAsc, Page: 1, PageSize: 1, Focus: nreB})

		assert.Equal(t, &listview.Focus{NRE: nreB, Found: true}, page.Focus)
		assert.Equal(t, "", page.Search)
		assert.Equal(t, []string{"done_after_cancel", "cancelled"}, page.ExcludedStatuses)
		assert.Equal(t, 2, page.Page)
		assert.Equal(t, []string{nreB}, nres(page.Rows))
	})

	t.Run("focus is case-insensitive", func(t *testing.T) {
		page := query(t, newFixture(rowB()), Query{Focus: "0900a0000000002"})

		assert.True(t, page.Focus.Found)
		assert.Equal(t, []string{nreB}, nres(page.Rows))
	})

	t.Run("an unknown focus is reported as not found and filters stay", func(t *testing.T) {
		page := query(t, newFixture(rowA(), rowB()), Query{Search: "alfa", Focus: "0900A0000000777"})

		assert.Equal(t, &listview.Focus{NRE: "0900A0000000777"}, page.Focus)
		assert.Equal(t, "alfa", page.Search)
		assert.Equal(t, []string{nreA}, nres(page.Rows))
	})

	t.Run("no focus leaves the field nil", func(t *testing.T) {
		page := query(t, newFixture(rowA()), Query{})

		assert.Nil(t, page.Focus)
	})
}

func TestQueryOverrides(t *testing.T) {
	t.Run("a manual status replaces the status", func(t *testing.T) {
		f := newFixture(rowA())
		f.overrides = []localrecords.Override{{NRE: "0900a0000000001", Status: 8}}

		page := query(t, f, Query{ExcludeStatuses: []string{}})

		assert.Equal(t, statusOf(8), page.Rows[0].Status)
	})

	t.Run("an override applies to the row endpoint", func(t *testing.T) {
		f := newFixture(rowA())
		f.overrides = []localrecords.Override{{NRE: nreA, Status: 4}}

		result, err := f.service().Row(t.Context(), nreA)

		require.NoError(t, err)
		assert.Equal(t, "cancelled", result.Row.Status.Key)
	})

	t.Run("does not mutate the snapshot", func(t *testing.T) {
		f := newFixture(rowA())
		f.overrides = []localrecords.Override{{NRE: nreA, Status: 8}}

		query(t, f, Query{})

		assert.Equal(t, 3, f.snapshot.Rows[0].FSE.Status)
	})
}

func TestQueryRefresh(t *testing.T) {
	t.Run("kicks the lists job when the list is stale", func(t *testing.T) {
		f := newFixture(rowA())
		f.snapshot.FSEList.Stale = true

		query(t, f, Query{})

		require.Len(t, f.kicks.KickCalls(), 1)
		assert.Equal(t, refresh.JobLists, f.kicks.KickCalls()[0].Job)
	})

	t.Run("kicks the lists job when the list is missing", func(t *testing.T) {
		f := newFixture()
		f.snapshot.FSEList = agenda.ListState{Missing: true}

		query(t, f, Query{})

		assert.Len(t, f.kicks.KickCalls(), 1)
	})

	t.Run("does not kick a fresh list", func(t *testing.T) {
		f := newFixture(rowA())

		query(t, f, Query{})

		assert.Empty(t, f.kicks.KickCalls())
	})

	t.Run("refreshing follows the lists job", func(t *testing.T) {
		f := newFixture(rowA())
		f.status = refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobLists}}

		page := query(t, f, Query{})

		assert.True(t, page.Refreshing)
	})

	t.Run("another job does not make the view refreshing", func(t *testing.T) {
		f := newFixture(rowA())
		f.status = refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobVisits}}

		page := query(t, f, Query{})

		assert.False(t, page.Refreshing)
	})

	t.Run("issue defaults to none and follows the lists issue", func(t *testing.T) {
		f := newFixture(rowA())
		assert.Equal(t, refresh.IssueNone, query(t, f, Query{}).Issue)

		f.status = refresh.Status{Issues: refresh.Issues{Lists: refresh.IssueLoginRequired}}

		assert.Equal(t, refresh.IssueLoginRequired, query(t, f, Query{}).Issue)
	})

	t.Run("updated comes from the fse list", func(t *testing.T) {
		page := query(t, newFixture(rowA()), Query{})

		assert.Equal(t, day3, page.Updated.At)
		assert.Equal(t, "Aggiornato alle 10:00", page.Updated.Label)
	})

	t.Run("updated is empty without a list", func(t *testing.T) {
		f := newFixture()
		f.snapshot.FSEList = agenda.ListState{Missing: true}

		page := query(t, f, Query{})

		assert.True(t, page.Updated.At.IsZero())
		assert.Equal(t, "", page.Updated.Label)
	})
}

func TestQueryErrors(t *testing.T) {
	t.Run("snapshot error is wrapped", func(t *testing.T) {
		ag := &MockAgenda{SnapshotFunc: func(context.Context) (agenda.Snapshot, error) { return agenda.Snapshot{}, errBoom }}
		svc := New(ag, &MockLocal{}, &MockRefresher{})

		_, err := svc.Query(t.Context(), Query{})

		require.ErrorIs(t, err, errBoom)
	})

	t.Run("overrides error is wrapped", func(t *testing.T) {
		ag := &MockAgenda{SnapshotFunc: func(context.Context) (agenda.Snapshot, error) { return agenda.Snapshot{}, nil }}
		local := &MockLocal{OverridesFunc: func(context.Context) ([]localrecords.Override, error) { return nil, errBoom }}
		svc := New(ag, local, &MockRefresher{})

		_, err := svc.Query(t.Context(), Query{})

		require.ErrorIs(t, err, errBoom)
	})
}

func TestRow(t *testing.T) {
	t.Run("returns the row and refreshing", func(t *testing.T) {
		f := newFixture(rowA())
		f.status = refresh.Status{Jobs: []refresh.Job{refresh.JobLists}}

		result, err := f.service().Row(t.Context(), "0900a0000000001")

		require.NoError(t, err)
		assert.Equal(t, nreA, result.Row.NRE)
		assert.True(t, result.Refreshing)
	})

	t.Run("unknown nre is not found", func(t *testing.T) {
		_, err := newFixture(rowA()).service().Row(t.Context(), nreB)

		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("a non drug row is not found", func(t *testing.T) {
		f := newFixture(agenda.Row{NRE: nreB, CUP: &cup.Prescription{NRE: nreB}})

		_, err := f.service().Row(t.Context(), nreB)

		require.ErrorIs(t, err, ErrNotFound)
	})
}
