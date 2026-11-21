package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/referti"
)

var errBoom = errors.New("boom")

func day(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

func TestServiceList(t *testing.T) {
	remoteRef := referti.Ref{Kind: referti.KindLab, AbstractID: "a1", DetailID: "d1", RepCode: "r"}

	t.Run("merges sorts and attaches NREs", func(t *testing.T) {
		remote := &MockRemote{ListFunc: func(ctx context.Context) (referti.Listing, error) {
			return referti.Listing{Reports: []referti.Report{
				{Ref: remoteRef, Date: day(5)},
				{Ref: referti.Ref{Kind: referti.KindLab, AbstractID: "a2"}},
			}}, nil
		}}
		local := &MockLocal{
			ListFunc: func(ctx context.Context) ([]localrecords.Report, error) {
				return []localrecords.Report{{ID: "l1", Meta: localrecords.Meta{Title: "T", Date: day(9), Provider: "P"}, CreatedAt: day(10)}}, nil
			},
			LinksFunc: func(ctx context.Context) ([]localrecords.Link, error) {
				return []localrecords.Link{
					{NRE: "B", Ref: remoteRef},
					{NRE: "A", Ref: remoteRef},
					{NRE: "A", Ref: remoteRef},
					{NRE: "C", Ref: localrecords.LocalRef("l1")},
				}, nil
			},
		}
		got, err := New(remote, local).List(t.Context())
		require.NoError(t, err)
		require.Len(t, got.Reports, 3)
		assert.Equal(t, OriginLocal, got.Reports[0].Origin)
		assert.Equal(t, "T", got.Reports[0].Title)
		assert.Equal(t, day(10), got.Reports[0].UpdatedAt)
		assert.Equal(t, []string{"C"}, got.Reports[0].NREs)
		assert.Equal(t, []string{"A", "B"}, got.Reports[1].NREs)
		assert.Equal(t, "a2", got.Reports[2].Ref.AbstractID)
		assert.Equal(t, []string{}, got.Reports[2].NREs)
	})

	t.Run("remote error becomes failure", func(t *testing.T) {
		remote := &MockRemote{ListFunc: func(ctx context.Context) (referti.Listing, error) {
			return referti.Listing{}, errBoom
		}}
		local := &MockLocal{
			ListFunc:  func(ctx context.Context) ([]localrecords.Report, error) { return nil, nil },
			LinksFunc: func(ctx context.Context) ([]localrecords.Link, error) { return nil, nil },
		}
		got, err := New(remote, local).List(t.Context())
		require.NoError(t, err)
		assert.Empty(t, got.Reports)
		require.Len(t, got.Failures, 1)
		assert.Equal(t, referti.KindFSE, got.Failures[0].Kind)
		assert.ErrorIs(t, got.Failures[0].Err, errBoom)
	})

	t.Run("local list error is returned", func(t *testing.T) {
		remote := &MockRemote{ListFunc: func(ctx context.Context) (referti.Listing, error) { return referti.Listing{}, nil }}
		local := &MockLocal{ListFunc: func(ctx context.Context) ([]localrecords.Report, error) { return nil, errBoom }}
		_, err := New(remote, local).List(t.Context())
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("links error is returned", func(t *testing.T) {
		remote := &MockRemote{ListFunc: func(ctx context.Context) (referti.Listing, error) { return referti.Listing{}, nil }}
		local := &MockLocal{
			ListFunc:  func(ctx context.Context) ([]localrecords.Report, error) { return nil, nil },
			LinksFunc: func(ctx context.Context) ([]localrecords.Link, error) { return nil, errBoom },
		}
		_, err := New(remote, local).List(t.Context())
		require.ErrorIs(t, err, errBoom)
	})
}

func TestServiceDocuments(t *testing.T) {
	t.Run("local ref returns single document", func(t *testing.T) {
		local := &MockLocal{ListFunc: func(ctx context.Context) ([]localrecords.Report, error) {
			return []localrecords.Report{{ID: "l1", Filename: "f.pdf", Meta: localrecords.Meta{Title: "T", Provider: "P"}, CreatedAt: day(3)}}, nil
		}}
		got, err := New(&MockRemote{}, local).Documents(t.Context(), localrecords.LocalRef("l1"))
		require.NoError(t, err)
		assert.Equal(t, []referti.Document{{Position: "1", Label: "f.pdf", AcceptedAt: day(3), Provider: "P"}}, got)
	})

	t.Run("local ref falls back to title", func(t *testing.T) {
		local := &MockLocal{ListFunc: func(ctx context.Context) ([]localrecords.Report, error) {
			return []localrecords.Report{{ID: "l1", Meta: localrecords.Meta{Title: "T"}}}, nil
		}}
		got, err := New(&MockRemote{}, local).Documents(t.Context(), localrecords.LocalRef("l1"))
		require.NoError(t, err)
		assert.Equal(t, "T", got[0].Label)
	})

	t.Run("local ref missing", func(t *testing.T) {
		local := &MockLocal{ListFunc: func(ctx context.Context) ([]localrecords.Report, error) { return nil, nil }}
		_, err := New(&MockRemote{}, local).Documents(t.Context(), localrecords.LocalRef("x"))
		require.ErrorIs(t, err, localrecords.ErrNotFound)
	})

	t.Run("remote ref delegates", func(t *testing.T) {
		remote := &MockRemote{DocumentsFunc: func(ctx context.Context, ref referti.Ref) ([]referti.Document, error) {
			return []referti.Document{{Position: "7"}}, nil
		}}
		got, err := New(remote, &MockLocal{}).Documents(t.Context(), referti.Ref{Kind: referti.KindLab})
		require.NoError(t, err)
		assert.Equal(t, "7", got[0].Position)
	})
}

func TestServiceDownload(t *testing.T) {
	t.Run("local ref returns file data", func(t *testing.T) {
		local := &MockLocal{FileFunc: func(ctx context.Context, id string) (localrecords.File, error) {
			return localrecords.File{Data: []byte("pdf")}, nil
		}}
		got, err := New(&MockRemote{}, local).Download(t.Context(), localrecords.LocalRef("l1"), "1")
		require.NoError(t, err)
		assert.Equal(t, []byte("pdf"), got)
	})

	t.Run("local ref wrong position", func(t *testing.T) {
		_, err := New(&MockRemote{}, &MockLocal{}).Download(t.Context(), localrecords.LocalRef("l1"), "2")
		require.ErrorIs(t, err, referti.ErrNoDocument)
	})

	t.Run("local file error", func(t *testing.T) {
		local := &MockLocal{FileFunc: func(ctx context.Context, id string) (localrecords.File, error) {
			return localrecords.File{}, localrecords.ErrNotFound
		}}
		_, err := New(&MockRemote{}, local).Download(t.Context(), localrecords.LocalRef("l1"), "1")
		require.ErrorIs(t, err, localrecords.ErrNotFound)
	})

	t.Run("remote ref delegates", func(t *testing.T) {
		remote := &MockRemote{DownloadFunc: func(ctx context.Context, ref referti.Ref, position string) ([]byte, error) {
			return []byte("r"), nil
		}}
		got, err := New(remote, &MockLocal{}).Download(t.Context(), referti.Ref{Kind: referti.KindLab}, "3")
		require.NoError(t, err)
		assert.Equal(t, []byte("r"), got)
	})
}
