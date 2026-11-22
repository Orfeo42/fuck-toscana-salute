package store

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fakeNRE2   = "0900A0000000002"
	metaMarker = "META_MARKER_XYZ"
)

func countTable(t *testing.T, table string) int {
	t.Helper()
	var n int
	require.NoError(t, testDB.GetContext(context.Background(), &n, "SELECT COUNT(*) FROM "+table))
	return n
}

func localReport(id string, at time.Time) LocalReport {
	return LocalReport{ID: id, Meta: []byte(`{"title":"` + metaMarker + `"}`), Data: []byte("DATA_MARKER_XYZ"), CreatedAt: at}
}

func TestInsertLocalReport(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)

	t.Run("round trip with links and override", func(t *testing.T) {
		s := openTest(t)
		in := localReport("r1", at)
		links := []Link{{NRE: fakeNRE, ReportKey: "k1"}, {NRE: fakeNRE2, ReportKey: "k1"}}
		ov := &Override{NRE: fakeNRE, Status: 3, UpdatedAt: at}
		require.NoError(t, s.InsertLocalReport(ctx, in, links, ov))

		got, err := s.LocalReportData(ctx, "r1")
		require.NoError(t, err)
		assert.Equal(t, in.Meta, got.Meta)
		assert.Equal(t, in.Data, got.Data)
		assert.True(t, at.Equal(got.CreatedAt))
		gotLinks, err := s.Links(ctx)
		require.NoError(t, err)
		assert.ElementsMatch(t, links, gotLinks)
		gotOv, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, gotOv, 1)
		assert.Equal(t, fakeNRE, gotOv[0].NRE)
		assert.Equal(t, 3, gotOv[0].Status)
	})

	t.Run("nil override and no links", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), nil, nil))
		assert.Equal(t, 1, countTable(t, "local_reports"))
		assert.Equal(t, 0, countTable(t, "report_links"))
		assert.Equal(t, 0, countTable(t, "visit_overrides"))
	})

	t.Run("duplicate link ignored", func(t *testing.T) {
		s := openTest(t)
		links := []Link{{NRE: fakeNRE, ReportKey: "k1"}, {NRE: fakeNRE, ReportKey: "k1"}}
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), links, nil))
		assert.Equal(t, 1, countTable(t, "report_links"))
	})

	t.Run("existing override replaced", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 1, UpdatedAt: at}))
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), nil, &Override{NRE: fakeNRE, Status: 2, UpdatedAt: at}))
		got, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, 2, got[0].Status)
	})

	t.Run("duplicate id rolls back links and override", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), nil, nil))
		err := s.InsertLocalReport(ctx, localReport("r1", at), []Link{{NRE: fakeNRE, ReportKey: "k1"}}, &Override{NRE: fakeNRE, Status: 1, UpdatedAt: at})
		require.Error(t, err)
		assert.Equal(t, 1, countTable(t, "local_reports"))
		assert.Equal(t, 0, countTable(t, "report_links"))
		assert.Equal(t, 0, countTable(t, "visit_overrides"))
	})

	t.Run("nothing clear in database", func(t *testing.T) {
		s := openTest(t)
		links := []Link{{NRE: fakeNRE, ReportKey: "REPORT_KEY_XYZ"}}
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), links, &Override{NRE: fakeNRE, Status: 1, UpdatedAt: at}))

		var lr localReportRow
		require.NoError(t, testDB.GetContext(ctx, &lr, "SELECT id, meta_enc, data_enc, created_at FROM local_reports"))
		assert.False(t, bytes.Contains(lr.MetaEnc, []byte(metaMarker)))
		assert.False(t, bytes.Contains(lr.DataEnc, []byte("DATA_MARKER_XYZ")))

		var l linkRow
		require.NoError(t, testDB.GetContext(ctx, &l, linksQuery))
		assert.NotContains(t, l.VisitMAC, fakeNRE)
		assert.NotContains(t, l.ReportMAC, "REPORT_KEY_XYZ")
		assert.False(t, bytes.Contains(l.PayloadEnc, []byte(fakeNRE)))
		assert.False(t, bytes.Contains(l.PayloadEnc, []byte("REPORT_KEY_XYZ")))

		var o overrideRow
		require.NoError(t, testDB.GetContext(ctx, &o, overridesQuery))
		assert.NotContains(t, o.VisitMAC, fakeNRE)
		assert.False(t, bytes.Contains(o.PayloadEnc, []byte(fakeNRE)))
	})
}

func TestUpdateLocalReportMeta(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)

	t.Run("updates meta only", func(t *testing.T) {
		s := openTest(t)
		in := localReport("r1", at)
		require.NoError(t, s.InsertLocalReport(ctx, in, nil, nil))
		require.NoError(t, s.UpdateLocalReportMeta(ctx, "r1", []byte(`{"title":"new"}`)))
		got, err := s.LocalReportData(ctx, "r1")
		require.NoError(t, err)
		assert.JSONEq(t, `{"title":"new"}`, string(got.Meta))
		assert.Equal(t, in.Data, got.Data)
	})

	t.Run("missing returns not found", func(t *testing.T) {
		s := openTest(t)
		err := s.UpdateLocalReportMeta(ctx, "nope", []byte("x"))
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestDeleteLocalReport(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)

	t.Run("removes report and only its links", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), []Link{{NRE: fakeNRE, ReportKey: "k1"}}, nil))
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r2", at), []Link{{NRE: fakeNRE, ReportKey: "k2"}}, nil))
		require.NoError(t, s.DeleteLocalReport(ctx, "r1", "k1"))
		_, err := s.LocalReportData(ctx, "r1")
		require.ErrorIs(t, err, ErrNotFound)
		_, err = s.LocalReportData(ctx, "r2")
		require.NoError(t, err)
		links, err := s.Links(ctx)
		require.NoError(t, err)
		assert.Equal(t, []Link{{NRE: fakeNRE, ReportKey: "k2"}}, links)
	})

	t.Run("missing report rolls back link delete", func(t *testing.T) {
		s := openTest(t)
		putLink(t, s, Link{NRE: fakeNRE, ReportKey: "k1"})
		err := s.DeleteLocalReport(ctx, "nope", "k1")
		require.ErrorIs(t, err, ErrNotFound)
		assert.Equal(t, 1, countTable(t, "report_links"))
	})
}

func TestLocalReports(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)

	t.Run("newest first with decrypted meta", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.InsertLocalReport(ctx, localReport("old", at), nil, nil))
		require.NoError(t, s.InsertLocalReport(ctx, localReport("new", at.Add(time.Hour)), nil, nil))
		got, err := s.LocalReports(ctx)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "new", got[0].ID)
		assert.Equal(t, "old", got[1].ID)
		assert.Equal(t, localReport("x", at).Meta, got[0].Meta)
		assert.True(t, at.Add(time.Hour).Equal(got[0].CreatedAt))
	})

	t.Run("empty", func(t *testing.T) {
		s := openTest(t)
		got, err := s.LocalReports(ctx)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("wrong id AAD detected", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.InsertLocalReport(ctx, localReport("r1", at), nil, nil))
		_, err := s.db.ExecContext(ctx, `UPDATE local_reports SET id = 'r2'`)
		require.NoError(t, err)
		_, err = s.LocalReports(ctx)
		require.ErrorIs(t, err, ErrDecrypt)
	})
}

func TestLocalReportData(t *testing.T) {
	ctx := context.Background()

	t.Run("missing returns not found", func(t *testing.T) {
		s := openTest(t)
		_, err := s.LocalReportData(ctx, "nope")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func putLink(t *testing.T, s *Store, l Link) {
	t.Helper()
	o := Override{NRE: l.NRE, Status: 8, UpdatedAt: time.UnixMilli(1_700_000_000_000)}
	require.NoError(t, s.PutLinkWithOverride(context.Background(), l, o))
}

func TestPutLinkWithOverride(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)
	l := Link{NRE: fakeNRE, ReportKey: "k1"}
	o := Override{NRE: fakeNRE, Status: 8, UpdatedAt: at}

	t.Run("writes link and override", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutLinkWithOverride(ctx, l, o))
		links, err := s.Links(ctx)
		require.NoError(t, err)
		assert.Equal(t, []Link{l}, links)
		overrides, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, overrides, 1)
		assert.Equal(t, fakeNRE, overrides[0].NRE)
		assert.Equal(t, 8, overrides[0].Status)
	})

	t.Run("idempotent", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutLinkWithOverride(ctx, l, o))
		require.NoError(t, s.PutLinkWithOverride(ctx, l, o))
		assert.Equal(t, 1, countTable(t, "report_links"))
		assert.Equal(t, 1, countTable(t, "visit_overrides"))
	})

	t.Run("overwrites previous override", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 3, UpdatedAt: at}))
		require.NoError(t, s.PutLinkWithOverride(ctx, l, o))
		overrides, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, overrides, 1)
		assert.Equal(t, 8, overrides[0].Status)
	})
}

func TestDeleteLink(t *testing.T) {
	ctx := context.Background()
	l := Link{NRE: fakeNRE, ReportKey: "k1"}

	t.Run("deletes only that link", func(t *testing.T) {
		s := openTest(t)
		other := Link{NRE: fakeNRE2, ReportKey: "k1"}
		putLink(t, s, l)
		putLink(t, s, other)
		require.NoError(t, s.DeleteLink(ctx, l))
		got, err := s.Links(ctx)
		require.NoError(t, err)
		assert.Equal(t, []Link{other}, got)
	})

	t.Run("missing is nil", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.DeleteLink(ctx, l))
	})
}

func TestLinks(t *testing.T) {
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		s := openTest(t)
		got, err := s.Links(ctx)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("payload swap detected", func(t *testing.T) {
		s := openTest(t)
		putLink(t, s, Link{NRE: fakeNRE, ReportKey: "k1"})
		putLink(t, s, Link{NRE: fakeNRE2, ReportKey: "k1"})
		_, err := s.db.ExecContext(ctx, `UPDATE report_links SET payload_enc = (SELECT payload_enc FROM report_links LIMIT 1)`)
		require.NoError(t, err)
		_, err = s.Links(ctx)
		require.ErrorIs(t, err, ErrDecrypt)
	})
}

func TestPutOverride(t *testing.T) {
	ctx := context.Background()
	at := time.UnixMilli(1_700_000_000_000)

	t.Run("round trip", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 2, UpdatedAt: at}))
		got, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, fakeNRE, got[0].NRE)
		assert.Equal(t, 2, got[0].Status)
		assert.True(t, at.Equal(got[0].UpdatedAt))
	})

	t.Run("upsert replaces", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 1, UpdatedAt: at}))
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 4, UpdatedAt: at.Add(time.Hour)}))
		got, err := s.Overrides(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, 4, got[0].Status)
		assert.True(t, at.Add(time.Hour).Equal(got[0].UpdatedAt))
	})
}

func TestDeleteOverride(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 1, UpdatedAt: time.Now()}))
		require.NoError(t, s.DeleteOverride(ctx, fakeNRE))
		assert.Equal(t, 0, countTable(t, "visit_overrides"))
	})

	t.Run("missing is nil", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.DeleteOverride(ctx, fakeNRE))
	})
}

func TestOverrides(t *testing.T) {
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		s := openTest(t)
		got, err := s.Overrides(ctx)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("payload swap detected", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE, Status: 1, UpdatedAt: time.Now()}))
		require.NoError(t, s.PutOverride(ctx, Override{NRE: fakeNRE2, Status: 2, UpdatedAt: time.Now()}))
		_, err := s.db.ExecContext(ctx, `UPDATE visit_overrides SET payload_enc = (SELECT payload_enc FROM visit_overrides LIMIT 1)`)
		require.NoError(t, err)
		_, err = s.Overrides(ctx)
		require.ErrorIs(t, err, ErrDecrypt)
	})
}

func TestOpenLocalTables(t *testing.T) {
	ctx := context.Background()

	t.Run("reopen keeps local data", func(t *testing.T) {
		s1 := openTest(t)
		require.NoError(t, s1.InsertLocalReport(ctx, localReport("r1", time.Now()), []Link{{NRE: fakeNRE, ReportKey: "k1"}}, &Override{NRE: fakeNRE, Status: 1, UpdatedAt: time.Now()}))
		s2, err := Open(ctx, testDSN, testKey(1))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s2.Close()) })
		_, err = s2.LocalReportData(ctx, "r1")
		require.NoError(t, err)
		links, err := s2.Links(ctx)
		require.NoError(t, err)
		assert.Len(t, links, 1)
	})
}
