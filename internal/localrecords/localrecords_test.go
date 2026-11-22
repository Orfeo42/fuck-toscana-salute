package localrecords

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/store"
)

const testNRE = "0900A0000000001"

var (
	fixedNow = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	pdfData  = []byte("%PDF-1.7 fake")
	pngData  = []byte("\x89PNG\r\n\x1a\n fake")
	jpegData = []byte("\xff\xd8\xff\xe0 fake")
)

func newService(st *MockStore) *Service {
	s := New(st)
	s.now = func() time.Time { return fixedNow }
	return s
}

func metaJSON(t *testing.T, r Report) []byte {
	t.Helper()
	b, err := encodeMeta(r)
	require.NoError(t, err)
	return b
}

func TestCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("empty file", func(t *testing.T) {
		_, err := newService(&MockStore{}).Create(ctx, NewReport{})
		assert.ErrorIs(t, err, ErrEmptyFile)
	})

	t.Run("too large", func(t *testing.T) {
		_, err := newService(&MockStore{}).Create(ctx, NewReport{Data: make([]byte, MaxFileBytes+1)})
		assert.ErrorIs(t, err, ErrTooLarge)
	})

	t.Run("unsupported type", func(t *testing.T) {
		_, err := newService(&MockStore{}).Create(ctx, NewReport{Data: []byte("just some text")})
		assert.ErrorIs(t, err, ErrUnsupportedType)
	})

	t.Run("pdf accepted", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: pdfData, Filename: "a.pdf", Origin: OriginUpload})
		require.NoError(t, err)
		assert.Equal(t, "application/pdf", r.MimeType)
		assert.Len(t, r.ID, 32)
		assert.Equal(t, fixedNow, r.CreatedAt)
		assert.Equal(t, OriginUpload, r.Origin)
	})

	t.Run("png accepted", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: pngData})
		require.NoError(t, err)
		assert.Equal(t, "image/png", r.MimeType)
	})

	t.Run("jpeg accepted", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: jpegData})
		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", r.MimeType)
	})

	t.Run("title defaults to filename without extension", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: pdfData, Filename: "esame.pdf", Meta: Meta{Title: "  "}})
		require.NoError(t, err)
		assert.Equal(t, "esame", r.Meta.Title)
	})

	t.Run("title defaults to Referto without filename", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: pdfData})
		require.NoError(t, err)
		assert.Equal(t, "Referto", r.Meta.Title)
	})

	t.Run("title and provider trimmed", func(t *testing.T) {
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return nil }}
		r, err := newService(st).Create(ctx, NewReport{Data: pdfData, Meta: Meta{Title: " T ", Provider: " P "}})
		require.NoError(t, err)
		assert.Equal(t, "T", r.Meta.Title)
		assert.Equal(t, "P", r.Meta.Provider)
	})

	t.Run("nre links and overrides in one insert", func(t *testing.T) {
		var gotRec store.LocalReport
		var gotLinks []store.Link
		var gotOverride *store.Override
		st := &MockStore{InsertLocalReportFunc: func(_ context.Context, rec store.LocalReport, links []store.Link, o *store.Override) error {
			gotRec, gotLinks, gotOverride = rec, links, o
			return nil
		}}
		date := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
		r, err := newService(st).Create(ctx, NewReport{Data: pdfData, NRE: " " + strings.ToLower(testNRE) + " ", Meta: Meta{Title: "t", Date: date}})
		require.NoError(t, err)
		assert.Equal(t, []store.Link{{NRE: testNRE, ReportKey: LocalRef(r.ID).Key()}}, gotLinks)
		assert.Equal(t, &store.Override{NRE: testNRE, Status: DoneStatus, UpdatedAt: fixedNow}, gotOverride)
		assert.Equal(t, pdfData, gotRec.Data)
		assert.Contains(t, string(gotRec.Meta), `"date":"2026-05-03"`)
	})

	t.Run("no link or override without nre", func(t *testing.T) {
		var gotLinks []store.Link
		var gotOverride *store.Override
		st := &MockStore{InsertLocalReportFunc: func(_ context.Context, _ store.LocalReport, links []store.Link, o *store.Override) error {
			gotLinks, gotOverride = links, o
			return nil
		}}
		_, err := newService(st).Create(ctx, NewReport{Data: pdfData})
		require.NoError(t, err)
		assert.Empty(t, gotLinks)
		assert.Nil(t, gotOverride)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{InsertLocalReportFunc: func(context.Context, store.LocalReport, []store.Link, *store.Override) error { return boom }}
		_, err := newService(st).Create(ctx, NewReport{Data: pdfData})
		assert.ErrorIs(t, err, boom)
	})
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	current := Report{ID: "abc", Meta: Meta{Title: "old"}, Filename: "f.pdf", MimeType: "application/pdf", Origin: OriginDrive}

	t.Run("missing report", func(t *testing.T) {
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) { return nil, nil }}
		assert.ErrorIs(t, newService(st).Update(ctx, "abc", Meta{}), ErrNotFound)
	})

	t.Run("keeps file fields and replaces meta", func(t *testing.T) {
		var gotID string
		var gotMeta []byte
		st := &MockStore{
			LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
				return []store.LocalReportMeta{{ID: "abc", Meta: metaJSON(t, current), CreatedAt: fixedNow}}, nil
			},
			UpdateLocalReportMetaFunc: func(_ context.Context, id string, meta []byte) error {
				gotID, gotMeta = id, meta
				return nil
			},
		}
		require.NoError(t, newService(st).Update(ctx, "abc", Meta{Title: " new ", Provider: " asl "}))
		assert.Equal(t, "abc", gotID)
		r, err := decodeReport("abc", gotMeta, fixedNow)
		require.NoError(t, err)
		assert.Equal(t, "new", r.Meta.Title)
		assert.Equal(t, "asl", r.Meta.Provider)
		assert.Equal(t, "f.pdf", r.Filename)
		assert.Equal(t, "application/pdf", r.MimeType)
		assert.Equal(t, OriginDrive, r.Origin)
	})

	t.Run("empty title defaults to stored filename", func(t *testing.T) {
		var gotMeta []byte
		st := &MockStore{
			LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
				return []store.LocalReportMeta{{ID: "abc", Meta: metaJSON(t, current)}}, nil
			},
			UpdateLocalReportMetaFunc: func(_ context.Context, _ string, meta []byte) error {
				gotMeta = meta
				return nil
			},
		}
		require.NoError(t, newService(st).Update(ctx, "abc", Meta{}))
		r, err := decodeReport("abc", gotMeta, fixedNow)
		require.NoError(t, err)
		assert.Equal(t, "f", r.Meta.Title)
	})

	t.Run("store not found mapped", func(t *testing.T) {
		st := &MockStore{
			LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
				return []store.LocalReportMeta{{ID: "abc", Meta: metaJSON(t, current)}}, nil
			},
			UpdateLocalReportMetaFunc: func(context.Context, string, []byte) error { return store.ErrNotFound },
		}
		assert.ErrorIs(t, newService(st).Update(ctx, "abc", Meta{}), ErrNotFound)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) { return nil, boom }}
		assert.ErrorIs(t, newService(st).Update(ctx, "abc", Meta{}), boom)
	})
}

func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("passes id and local key", func(t *testing.T) {
		var gotID, gotKey string
		st := &MockStore{DeleteLocalReportFunc: func(_ context.Context, id, key string) error {
			gotID, gotKey = id, key
			return nil
		}}
		require.NoError(t, newService(st).Delete(ctx, "abc"))
		assert.Equal(t, "abc", gotID)
		assert.Equal(t, LocalRef("abc").Key(), gotKey)
	})

	t.Run("not found mapped", func(t *testing.T) {
		st := &MockStore{DeleteLocalReportFunc: func(context.Context, string, string) error { return store.ErrNotFound }}
		assert.ErrorIs(t, newService(st).Delete(ctx, "abc"), ErrNotFound)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{DeleteLocalReportFunc: func(context.Context, string, string) error { return boom }}
		assert.ErrorIs(t, newService(st).Delete(ctx, "abc"), boom)
	})
}

func TestList(t *testing.T) {
	ctx := context.Background()

	t.Run("sorted by date desc zero last then created desc", func(t *testing.T) {
		d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		d2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
			return []store.LocalReportMeta{
				{ID: "nodate-old", Meta: metaJSON(t, Report{}), CreatedAt: fixedNow.Add(-time.Hour)},
				{ID: "d1", Meta: metaJSON(t, Report{Meta: Meta{Date: d1}}), CreatedAt: fixedNow},
				{ID: "nodate-new", Meta: metaJSON(t, Report{}), CreatedAt: fixedNow},
				{ID: "d2", Meta: metaJSON(t, Report{Meta: Meta{Date: d2}}), CreatedAt: fixedNow},
			}, nil
		}}
		got, err := newService(st).List(ctx)
		require.NoError(t, err)
		ids := []string{got[0].ID, got[1].ID, got[2].ID, got[3].ID}
		assert.Equal(t, []string{"d2", "d1", "nodate-new", "nodate-old"}, ids)
		assert.True(t, got[0].Meta.Date.Equal(d2))
	})

	t.Run("invalid meta errors", func(t *testing.T) {
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
			return []store.LocalReportMeta{{ID: "x", Meta: []byte("not json")}}, nil
		}}
		_, err := newService(st).List(ctx)
		assert.Error(t, err)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) { return nil, boom }}
		_, err := newService(st).List(ctx)
		assert.ErrorIs(t, err, boom)
	})
}

func TestFile(t *testing.T) {
	ctx := context.Background()

	t.Run("returns data with meta fields", func(t *testing.T) {
		st := &MockStore{LocalReportDataFunc: func(context.Context, string) (store.LocalReport, error) {
			return store.LocalReport{ID: "abc", Data: pdfData, Meta: metaJSON(t, Report{Filename: "f.pdf", MimeType: "application/pdf"})}, nil
		}}
		f, err := newService(st).File(ctx, "abc")
		require.NoError(t, err)
		assert.Equal(t, File{Data: pdfData, MimeType: "application/pdf", Filename: "f.pdf"}, f)
	})

	t.Run("not found mapped", func(t *testing.T) {
		st := &MockStore{LocalReportDataFunc: func(context.Context, string) (store.LocalReport, error) {
			return store.LocalReport{}, store.ErrNotFound
		}}
		_, err := newService(st).File(ctx, "abc")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{LocalReportDataFunc: func(context.Context, string) (store.LocalReport, error) {
			return store.LocalReport{}, boom
		}}
		_, err := newService(st).File(ctx, "abc")
		assert.ErrorIs(t, err, boom)
	})

	t.Run("invalid meta errors", func(t *testing.T) {
		st := &MockStore{LocalReportDataFunc: func(context.Context, string) (store.LocalReport, error) {
			return store.LocalReport{Meta: []byte("bad")}, nil
		}}
		_, err := newService(st).File(ctx, "abc")
		assert.Error(t, err)
	})
}

func TestLink(t *testing.T) {
	ctx := context.Background()
	labRef := referti.Ref{Kind: referti.KindLab, AbstractID: "1", DetailID: "2", RepCode: "3"}

	t.Run("unknown kind rejected", func(t *testing.T) {
		err := newService(&MockStore{}).Link(ctx, testNRE, referti.Ref{Kind: "zzz"})
		assert.ErrorIs(t, err, ErrInvalidReportKind)
	})

	t.Run("fse kind rejected", func(t *testing.T) {
		err := newService(&MockStore{}).Link(ctx, testNRE, referti.Ref{Kind: referti.KindFSE})
		assert.ErrorIs(t, err, ErrInvalidReportKind)
	})

	t.Run("portal kind stored with upper nre", func(t *testing.T) {
		var gotLink store.Link
		var gotOverride store.Override
		st := &MockStore{PutLinkWithOverrideFunc: func(_ context.Context, l store.Link, o store.Override) error {
			gotLink = l
			gotOverride = o
			return nil
		}}
		require.NoError(t, newService(st).Link(ctx, " "+strings.ToLower(testNRE), labRef))
		assert.Equal(t, store.Link{NRE: testNRE, ReportKey: labRef.Key()}, gotLink)
		assert.Equal(t, store.Override{NRE: testNRE, Status: DoneStatus, UpdatedAt: fixedNow.UTC()}, gotOverride)
	})

	t.Run("local ref must exist", func(t *testing.T) {
		st := &MockStore{LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) { return nil, nil }}
		err := newService(st).Link(ctx, testNRE, LocalRef("abc"))
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("existing local ref linked", func(t *testing.T) {
		st := &MockStore{
			LocalReportsFunc: func(context.Context) ([]store.LocalReportMeta, error) {
				return []store.LocalReportMeta{{ID: "abc", Meta: metaJSON(t, Report{})}}, nil
			},
			PutLinkWithOverrideFunc: func(context.Context, store.Link, store.Override) error { return nil },
		}
		require.NoError(t, newService(st).Link(ctx, testNRE, LocalRef("abc")))
		assert.Len(t, st.PutLinkWithOverrideCalls(), 1)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{PutLinkWithOverrideFunc: func(context.Context, store.Link, store.Override) error { return boom }}
		assert.ErrorIs(t, newService(st).Link(ctx, testNRE, labRef), boom)
	})
}

func TestUnlink(t *testing.T) {
	ctx := context.Background()
	ref := LocalRef("abc")

	t.Run("deletes normalized link", func(t *testing.T) {
		var got store.Link
		st := &MockStore{DeleteLinkFunc: func(_ context.Context, l store.Link) error {
			got = l
			return nil
		}}
		require.NoError(t, newService(st).Unlink(ctx, strings.ToLower(testNRE), ref))
		assert.Equal(t, store.Link{NRE: testNRE, ReportKey: ref.Key()}, got)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{DeleteLinkFunc: func(context.Context, store.Link) error { return boom }}
		assert.ErrorIs(t, newService(st).Unlink(ctx, testNRE, ref), boom)
	})
}

func TestLinks(t *testing.T) {
	ctx := context.Background()

	t.Run("parses keys and skips malformed", func(t *testing.T) {
		ref := LocalRef("abc")
		st := &MockStore{LinksFunc: func(context.Context) ([]store.Link, error) {
			return []store.Link{{NRE: testNRE, ReportKey: ref.Key()}, {NRE: testNRE, ReportKey: "garbage"}}, nil
		}}
		got, err := newService(st).Links(ctx)
		require.NoError(t, err)
		assert.Equal(t, []Link{{NRE: testNRE, Ref: ref}}, got)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{LinksFunc: func(context.Context) ([]store.Link, error) { return nil, boom }}
		_, err := newService(st).Links(ctx)
		assert.ErrorIs(t, err, boom)
	})
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid status rejected", func(t *testing.T) {
		assert.ErrorIs(t, newService(&MockStore{}).SetStatus(ctx, testNRE, 1), ErrInvalidStatus)
	})

	t.Run("valid status stored", func(t *testing.T) {
		var got store.Override
		st := &MockStore{PutOverrideFunc: func(_ context.Context, o store.Override) error {
			got = o
			return nil
		}}
		require.NoError(t, newService(st).SetStatus(ctx, strings.ToLower(testNRE), 30))
		assert.Equal(t, store.Override{NRE: testNRE, Status: 30, UpdatedAt: fixedNow}, got)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{PutOverrideFunc: func(context.Context, store.Override) error { return boom }}
		assert.ErrorIs(t, newService(st).SetStatus(ctx, testNRE, 8), boom)
	})
}

func TestResetStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes normalized nre", func(t *testing.T) {
		var got string
		st := &MockStore{DeleteOverrideFunc: func(_ context.Context, nre string) error {
			got = nre
			return nil
		}}
		require.NoError(t, newService(st).ResetStatus(ctx, strings.ToLower(testNRE)))
		assert.Equal(t, testNRE, got)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{DeleteOverrideFunc: func(context.Context, string) error { return boom }}
		assert.ErrorIs(t, newService(st).ResetStatus(ctx, testNRE), boom)
	})
}

func TestOverrides(t *testing.T) {
	ctx := context.Background()

	t.Run("maps rows", func(t *testing.T) {
		st := &MockStore{OverridesFunc: func(context.Context) ([]store.Override, error) {
			return []store.Override{{NRE: testNRE, Status: 8, UpdatedAt: fixedNow}}, nil
		}}
		got, err := newService(st).Overrides(ctx)
		require.NoError(t, err)
		assert.Equal(t, []Override{{NRE: testNRE, Status: 8, UpdatedAt: fixedNow}}, got)
	})

	t.Run("store error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		st := &MockStore{OverridesFunc: func(context.Context) ([]store.Override, error) { return nil, boom }}
		_, err := newService(st).Overrides(ctx)
		assert.ErrorIs(t, err, boom)
	})
}

func TestNew(t *testing.T) {
	t.Run("sets store and clock", func(t *testing.T) {
		s := New(&MockStore{})
		assert.NotNil(t, s.store)
		assert.NotNil(t, s.now)
	})
}

func TestDecodeReport(t *testing.T) {
	t.Run("invalid date errors", func(t *testing.T) {
		_, err := decodeReport("x", []byte(`{"date":"nope"}`), fixedNow)
		assert.Error(t, err)
	})
}
