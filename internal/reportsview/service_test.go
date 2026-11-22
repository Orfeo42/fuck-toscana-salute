package reportsview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

var errBoom = errors.New("boom")

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func remote(kind referti.Kind, id, provider string, date time.Time, nres ...string) reports.Report {
	return reports.Report{
		Report: referti.Report{
			Ref:      referti.Ref{Kind: kind, AbstractID: id, DetailID: "d", RepCode: "r"},
			Date:     date,
			Provider: provider,
		},
		Origin: reports.OriginRemote,
		NREs:   nres,
	}
}

func local(id, title, provider string, date time.Time, nres ...string) reports.Report {
	return reports.Report{
		Report: referti.Report{
			Ref:      referti.Ref{Kind: referti.KindLocal, AbstractID: id},
			Date:     date,
			Provider: provider,
		},
		Origin: reports.OriginLocal,
		Title:  title,
		NREs:   nres,
	}
}

func visit(nre, description string, on time.Time) agenda.Row {
	return agenda.Row{NRE: nre, FSE: &fse.Prescription{NRE: nre, Description: description, Kind: fse.KindVisit, Date: on}}
}

type setup struct {
	cached  reports.CachedListing
	rows    []agenda.Row
	status  refresh.Status
	kicked  []refresh.Job
	repErr  error
	agErr   error
	service *Service
}

func newSetup(list ...reports.Report) *setup {
	s := &setup{cached: reports.CachedListing{Listing: reports.Listing{Reports: list}, FetchedAt: now}}
	rep := &MockReports{CachedListFunc: func(context.Context) (reports.CachedListing, error) { return s.cached, s.repErr }}
	ag := &MockAgenda{SnapshotFunc: func(context.Context) (agenda.Snapshot, error) {
		return agenda.Snapshot{Rows: s.rows}, s.agErr
	}}
	ref := &MockRefresher{
		KickFunc:   func(job refresh.Job) { s.kicked = append(s.kicked, job) },
		StatusFunc: func() refresh.Status { return s.status },
	}
	s.service = New(rep, ag, ref)
	s.service.now = func() time.Time { return now }
	return s
}

func ids(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Ref.AbstractID)
	}
	return out
}

func query(s *setup, q Query) Page {
	page, err := s.service.Query(context.Background(), q)
	if err != nil {
		panic(err)
	}
	return page
}

func TestQuerySearch(t *testing.T) {
	lab := remote(referti.KindLab, "lab", "Ospedale Esempio", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	radio := remote(referti.KindRadiology, "radio", "Clinica Prova", time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC))
	eco := local("eco", "Ecografia addome", "Altro", time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC))

	t.Run("returns everything for a blank query", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Search: "  "})

		assert.Equal(t, []string{"radio", "lab"}, ids(page.Rows))
	})

	t.Run("matches the provider case-insensitively", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Search: "OSPEDALE"})

		assert.Equal(t, []string{"lab"}, ids(page.Rows))
	})

	t.Run("matches the title of an uploaded report", func(t *testing.T) {
		page := query(newSetup(lab, eco), Query{Search: "addome"})

		assert.Equal(t, []string{"eco"}, ids(page.Rows))
	})

	t.Run("matches the kind label", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Search: "radio"})

		assert.Equal(t, []string{"radio"}, ids(page.Rows))
	})

	t.Run("ignores accents", func(t *testing.T) {
		accented := remote(referti.KindLab, "acc", "Università", time.Time{})

		page := query(newSetup(accented), Query{Search: "universita"})

		assert.Equal(t, []string{"acc"}, ids(page.Rows))
	})

	t.Run("total counts the matches not the page", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Search: "radio"})

		assert.Equal(t, 1, page.Total)
	})
}

func TestQueryKinds(t *testing.T) {
	lab := remote(referti.KindLab, "lab1", "P", time.Time{})
	lab2 := remote(referti.KindLab, "lab2", "P", time.Time{})
	radio := remote(referti.KindRadiology, "radio", "P", time.Time{})
	eco := local("eco", "Eco", "P", time.Time{})

	t.Run("lists distinct kinds with counts in label order", func(t *testing.T) {
		page := query(newSetup(radio, lab, eco, lab2), Query{})

		assert.Equal(t, []KindOption{
			{Key: "local", Label: "Caricato", Count: 1},
			{Key: "lab", Label: "Laboratorio analisi", Count: 2},
			{Key: "ris", Label: "Radiologia", Count: 1},
		}, page.Kinds)
	})

	t.Run("counts ignore search and kind filters", func(t *testing.T) {
		page := query(newSetup(radio, lab, lab2), Query{Search: "radio", Kind: "ris"})

		assert.Equal(t, []KindOption{
			{Key: "lab", Label: "Laboratorio analisi", Count: 2},
			{Key: "ris", Label: "Radiologia", Count: 1},
		}, page.Kinds)
	})

	t.Run("keeps everything for an empty kind", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{})

		assert.Len(t, page.Rows, 2)
	})

	t.Run("keeps only the chosen kind", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Kind: "ris"})

		assert.Equal(t, []string{"radio"}, ids(page.Rows))
	})

	t.Run("no reports gives empty kinds", func(t *testing.T) {
		page := query(newSetup(), Query{})

		assert.Empty(t, page.Kinds)
		assert.Empty(t, page.Rows)
	})
}

func TestQuerySort(t *testing.T) {
	lab := remote(referti.KindLab, "lab", "Ospedale Esempio", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	radio := remote(referti.KindRadiology, "radio", "Clinica Prova", time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC))
	undated := remote(referti.KindLab, "undated", "Clinica Prova", time.Time{})

	t.Run("sorts by date descending with undated last", func(t *testing.T) {
		page := query(newSetup(undated, lab, radio), Query{Sort: SortDate, Dir: listview.DirDesc})

		assert.Equal(t, []string{"radio", "lab", "undated"}, ids(page.Rows))
	})

	t.Run("sorts by date ascending with undated still last", func(t *testing.T) {
		page := query(newSetup(undated, lab, radio), Query{Sort: SortDate, Dir: listview.DirAsc})

		assert.Equal(t, []string{"lab", "radio", "undated"}, ids(page.Rows))
	})

	t.Run("sorts by provider ascending", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Sort: SortProvider, Dir: listview.DirAsc})

		assert.Equal(t, []string{"radio", "lab"}, ids(page.Rows))
	})

	t.Run("sorts by kind label descending", func(t *testing.T) {
		page := query(newSetup(lab, radio), Query{Sort: SortKind, Dir: listview.DirDesc})

		assert.Equal(t, []string{"radio", "lab"}, ids(page.Rows))
	})

	t.Run("ties break by date descending then id", func(t *testing.T) {
		older := remote(referti.KindLab, "b", "Same", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		newer := remote(referti.KindLab, "c", "Same", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		sameA := remote(referti.KindLab, "a", "Same", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

		page := query(newSetup(older, newer, sameA), Query{Sort: SortProvider, Dir: listview.DirAsc})

		assert.Equal(t, []string{"c", "a", "b"}, ids(page.Rows))
	})

	t.Run("missing provider sorts last in both directions", func(t *testing.T) {
		blank := remote(referti.KindLab, "blank", "", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))

		asc := query(newSetup(blank, lab, radio), Query{Sort: SortProvider, Dir: listview.DirAsc})
		desc := query(newSetup(blank, lab, radio), Query{Sort: SortProvider, Dir: listview.DirDesc})

		assert.Equal(t, []string{"radio", "lab", "blank"}, ids(asc.Rows))
		assert.Equal(t, []string{"lab", "radio", "blank"}, ids(desc.Rows))
	})
}

func TestQueryPaging(t *testing.T) {
	list := make([]reports.Report, 0, 7)
	for i := range 7 {
		list = append(list, remote(referti.KindLab, string(rune('a'+i)), "P", time.Date(2026, 1, 10-i, 0, 0, 0, 0, time.UTC)))
	}

	t.Run("returns the requested page", func(t *testing.T) {
		page := query(newSetup(list...), Query{Page: 2, PageSize: 3})

		assert.Equal(t, []string{"d", "e", "f"}, ids(page.Rows))
		assert.Equal(t, 7, page.Total)
		assert.Equal(t, 2, page.Page)
		assert.Equal(t, 3, page.PageSize)
	})

	t.Run("last page holds the remainder", func(t *testing.T) {
		page := query(newSetup(list...), Query{Page: 3, PageSize: 3})

		assert.Equal(t, []string{"g"}, ids(page.Rows))
	})

	t.Run("clamps a page past the end", func(t *testing.T) {
		page := query(newSetup(list...), Query{Page: 99, PageSize: 3})

		assert.Equal(t, 3, page.Page)
		assert.Equal(t, []string{"g"}, ids(page.Rows))
	})

	t.Run("clamps a page below one", func(t *testing.T) {
		page := query(newSetup(list...), Query{Page: 0, PageSize: 3})

		assert.Equal(t, 1, page.Page)
	})

	t.Run("defaults the page size", func(t *testing.T) {
		page := query(newSetup(list...), Query{})

		assert.Equal(t, listview.DefaultPageSize, page.PageSize)
		assert.Len(t, page.Rows, 7)
	})

	t.Run("empty result is page one", func(t *testing.T) {
		page := query(newSetup(), Query{Page: 4, PageSize: 25})

		assert.Equal(t, 1, page.Page)
		assert.Equal(t, 0, page.Total)
		assert.Empty(t, page.Rows)
	})
}

func TestQueryRowFields(t *testing.T) {
	t.Run("row id is the ref key", func(t *testing.T) {
		r := remote(referti.KindLab, "a", "P", time.Time{})

		page := query(newSetup(r), Query{})

		assert.Equal(t, r.Ref.Key(), page.Rows[0].ID)
		assert.Equal(t, "lab\x1fa\x1fd\x1fr", page.Rows[0].ID)
	})

	t.Run("remote title is the kind label", func(t *testing.T) {
		page := query(newSetup(remote(referti.KindRadiology, "a", "P", time.Time{})), Query{})

		assert.Equal(t, "Radiologia", page.Rows[0].Title)
		assert.Empty(t, page.Rows[0].CustomTitle)
	})

	t.Run("local title is the custom title", func(t *testing.T) {
		page := query(newSetup(local("a", "Ecografia", "P", time.Time{})), Query{})

		assert.Equal(t, "Ecografia", page.Rows[0].Title)
		assert.Equal(t, "Ecografia", page.Rows[0].CustomTitle)
	})

	t.Run("local without a title falls back to the kind label", func(t *testing.T) {
		page := query(newSetup(local("a", "", "P", time.Time{})), Query{})

		assert.Equal(t, "Caricato", page.Rows[0].Title)
	})

	t.Run("edit date is the Rome day around midnight", func(t *testing.T) {
		late := remote(referti.KindLab, "late", "P", time.Date(2026, 11, 9, 23, 30, 0, 0, time.UTC))
		early := remote(referti.KindLab, "early", "P", time.Date(2026, 11, 10, 22, 30, 0, 0, time.UTC))
		summer := remote(referti.KindLab, "summer", "P", time.Date(2026, 7, 9, 22, 30, 0, 0, time.UTC))

		page := query(newSetup(late, early, summer), Query{Sort: SortDate, Dir: listview.DirAsc})

		assert.Equal(t, []string{"summer", "late", "early"}, ids(page.Rows))
		assert.Equal(t, "2026-07-10", page.Rows[0].EditDate)
		assert.Equal(t, "2026-11-10", page.Rows[1].EditDate)
		assert.Equal(t, "2026-11-10", page.Rows[2].EditDate)
	})

	t.Run("no date has no edit date", func(t *testing.T) {
		page := query(newSetup(remote(referti.KindLab, "a", "P", time.Time{})), Query{})

		assert.Empty(t, page.Rows[0].EditDate)
	})

	t.Run("flags and origin pass through", func(t *testing.T) {
		r := remote(referti.KindLab, "a", "P", time.Time{})
		r.Unread = true
		r.Hidden = true

		page := query(newSetup(r), Query{})

		assert.True(t, page.Rows[0].Unread)
		assert.True(t, page.Rows[0].Hidden)
		assert.Equal(t, reports.OriginRemote, page.Rows[0].Origin)
	})
}

func TestQueryLinkedVisits(t *testing.T) {
	s := newSetup(
		remote(referti.KindLab, "none", "P", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		remote(referti.KindLab, "one", "P", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "NRE1"),
		remote(referti.KindLab, "two", "P", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), "NRE1", "NRE2"),
	)
	s.rows = []agenda.Row{
		visit("NRE1", "Visita cardiologica", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		visit("NRE2", "Ecografia", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}

	page := query(s, Query{})

	t.Run("no links has an empty label and list", func(t *testing.T) {
		assert.Empty(t, page.Rows[2].LinkedLabel)
		assert.Empty(t, page.Rows[2].Visits)
	})

	t.Run("one link is singular", func(t *testing.T) {
		assert.Equal(t, "Collegato a 1 visita", page.Rows[1].LinkedLabel)
		assert.Equal(t, []LinkedVisit{{NRE: "NRE1", Title: "Visita cardiologica"}}, page.Rows[1].Visits)
	})

	t.Run("several links are plural with agenda titles", func(t *testing.T) {
		assert.Equal(t, "Collegato a 2 visite", page.Rows[0].LinkedLabel)
		assert.Equal(t, []LinkedVisit{
			{NRE: "NRE1", Title: "Visita cardiologica"},
			{NRE: "NRE2", Title: "Ecografia"},
		}, page.Rows[0].Visits)
	})

	t.Run("a link missing from the agenda keeps the NRE as title", func(t *testing.T) {
		s := newSetup(remote(referti.KindLab, "one", "P", time.Time{}, "GONE"))

		page := query(s, Query{})

		assert.Equal(t, []LinkedVisit{{NRE: "GONE", Title: "GONE"}}, page.Rows[0].Visits)
	})
}

func TestQueryFailures(t *testing.T) {
	failing := func(kinds ...referti.Kind) *setup {
		s := newSetup()
		for _, k := range kinds {
			s.cached.Listing.Failures = append(s.cached.Listing.Failures, referti.Failure{Kind: k, Err: errBoom})
		}
		return s
	}

	t.Run("an fse failure sets fseUnavailable and is not listed", func(t *testing.T) {
		page := query(failing(referti.KindFSE), Query{})

		assert.True(t, page.FSEUnavailable)
		assert.Empty(t, page.Failures)
	})

	t.Run("kind failures are listed with labels", func(t *testing.T) {
		page := query(failing(referti.KindRadiology), Query{})

		assert.False(t, page.FSEUnavailable)
		assert.Equal(t, []Failure{{Kind: "ris", KindLabel: "Radiologia", Message: "boom"}}, page.Failures)
	})

	t.Run("both are split", func(t *testing.T) {
		page := query(failing(referti.KindFSE, referti.KindLab), Query{})

		assert.True(t, page.FSEUnavailable)
		require.Len(t, page.Failures, 1)
		assert.Equal(t, "lab", page.Failures[0].Kind)
	})

	t.Run("no failures is an empty list", func(t *testing.T) {
		page := query(failing(), Query{})

		assert.NotNil(t, page.Failures)
		assert.Empty(t, page.Failures)
	})
}

func TestQueryRefresh(t *testing.T) {
	t.Run("stale listing kicks the reports job", func(t *testing.T) {
		s := newSetup()
		s.cached.Stale = true

		query(s, Query{})

		assert.Equal(t, []refresh.Job{refresh.JobReports}, s.kicked)
	})

	t.Run("missing listing kicks the reports job", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true

		query(s, Query{})

		assert.Equal(t, []refresh.Job{refresh.JobReports}, s.kicked)
	})

	t.Run("fresh listing does not kick", func(t *testing.T) {
		s := newSetup()

		query(s, Query{})

		assert.Empty(t, s.kicked)
	})

	t.Run("refreshing follows an active reports job", func(t *testing.T) {
		s := newSetup()
		s.status = refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobReports}}

		assert.True(t, query(s, Query{}).Refreshing)
	})

	t.Run("another active job is not refreshing", func(t *testing.T) {
		s := newSetup()
		s.status = refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobVisits}}

		assert.False(t, query(s, Query{}).Refreshing)
	})

	t.Run("updated carries the fetch time and Italian label", func(t *testing.T) {
		s := newSetup()
		s.cached.FetchedAt = time.Date(2026, 11, 10, 8, 30, 0, 0, time.UTC)

		page := query(s, Query{})

		assert.Equal(t, s.cached.FetchedAt, page.Updated.At)
		assert.Equal(t, "Aggiornato alle 09:30", page.Updated.Label)
	})

	t.Run("updated on another day shows day month and time", func(t *testing.T) {
		s := newSetup()
		s.cached.FetchedAt = time.Date(2026, 11, 8, 14, 5, 0, 0, time.UTC)

		page := query(s, Query{})

		assert.Equal(t, "Aggiornato il 08/11 15:05", page.Updated.Label)
	})

	t.Run("missing listing has no updated", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true
		s.cached.FetchedAt = time.Time{}

		page := query(s, Query{})

		assert.Equal(t, listview.Updated{}, page.Updated)
	})
}

func TestQueryIssue(t *testing.T) {
	t.Run("cached listing has no issue", func(t *testing.T) {
		s := newSetup()
		s.status.Issues.Reports = refresh.IssueLoginRequired

		assert.Equal(t, refresh.IssueNone, query(s, Query{}).Issue)
	})

	t.Run("missing listing reports the login issue", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true
		s.status.Issues.Reports = refresh.IssueLoginRequired

		assert.Equal(t, refresh.IssueLoginRequired, query(s, Query{}).Issue)
	})

	t.Run("missing listing reports unavailable", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true
		s.status.Issues.Reports = refresh.IssueUnavailable

		assert.Equal(t, refresh.IssueUnavailable, query(s, Query{}).Issue)
	})

	t.Run("missing listing without a failure has no issue", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true

		assert.Equal(t, refresh.IssueNone, query(s, Query{}).Issue)
	})

	t.Run("an unsupported issue is none", func(t *testing.T) {
		s := newSetup()
		s.cached.Missing = true
		s.status.Issues.Reports = refresh.IssueGestioneRequired

		assert.Equal(t, refresh.IssueNone, query(s, Query{}).Issue)
	})
}

func TestQueryErrors(t *testing.T) {
	t.Run("cached list error is returned", func(t *testing.T) {
		s := newSetup()
		s.repErr = errBoom

		_, err := s.service.Query(context.Background(), Query{})

		require.ErrorIs(t, err, errBoom)
	})

	t.Run("snapshot error is returned", func(t *testing.T) {
		s := newSetup()
		s.agErr = errBoom

		_, err := s.service.Query(context.Background(), Query{})

		require.ErrorIs(t, err, errBoom)
	})
}

func TestRow(t *testing.T) {
	r := remote(referti.KindLab, "a", "P", time.Time{}, "NRE1")

	t.Run("returns the row for a known ref", func(t *testing.T) {
		s := newSetup(r)
		s.status = refresh.Status{Active: true, Jobs: []refresh.Job{refresh.JobReports}}

		result, err := s.service.Row(context.Background(), r.Ref)

		require.NoError(t, err)
		assert.Equal(t, r.Ref.Key(), result.Row.ID)
		assert.True(t, result.Refreshing)
	})

	t.Run("unknown ref is not found", func(t *testing.T) {
		s := newSetup(r)

		_, err := s.service.Row(context.Background(), referti.Ref{Kind: referti.KindLab, AbstractID: "zzz"})

		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("error from the cache is returned", func(t *testing.T) {
		s := newSetup(r)
		s.repErr = errBoom

		_, err := s.service.Row(context.Background(), r.Ref)

		require.ErrorIs(t, err, errBoom)
	})
}

func TestVisitOptions(t *testing.T) {
	r := remote(referti.KindLab, "a", "P", time.Time{}, "nre1")
	rows := []agenda.Row{
		visit("NRE1", "Linked", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
		visit("NRE2", "Older", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		visit("NRE3", "Newest", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)),
		visit("NRE0", "Same day as older", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		{NRE: "DRUG1", FSE: &fse.Prescription{NRE: "DRUG1", Description: "Farmaco", Kind: fse.KindDrug}},
	}

	t.Run("lists visits not linked newest first with NRE tie-break", func(t *testing.T) {
		s := newSetup(r)
		s.rows = rows

		got, err := s.service.VisitOptions(context.Background(), r.Ref)

		require.NoError(t, err)
		assert.Equal(t, []LinkedVisit{
			{NRE: "NRE3", Title: "Newest"},
			{NRE: "NRE0", Title: "Same day as older"},
			{NRE: "NRE2", Title: "Older"},
		}, got)
	})

	t.Run("unknown ref is not found", func(t *testing.T) {
		s := newSetup(r)

		_, err := s.service.VisitOptions(context.Background(), referti.Ref{Kind: referti.KindLab, AbstractID: "zzz"})

		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("snapshot error is returned", func(t *testing.T) {
		s := newSetup(r)
		s.agErr = errBoom

		_, err := s.service.VisitOptions(context.Background(), r.Ref)

		require.ErrorIs(t, err, errBoom)
	})
}
