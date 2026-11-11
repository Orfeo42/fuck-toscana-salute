package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchedAt(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the store times", func(t *testing.T) {
		at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
		fresh := &MockFreshness{FetchedAtFunc: func(context.Context, string) (map[string]time.Time, error) {
			return map[string]time.Time{"X": at}, nil
		}}

		got := fetchedAt(ctx, fresh, "src")

		assert.Equal(t, map[string]time.Time{"X": at}, got)
	})

	t.Run("nil map becomes empty", func(t *testing.T) {
		got := fetchedAt(ctx, emptyFreshness(), "src")

		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("error becomes empty", func(t *testing.T) {
		fresh := &MockFreshness{FetchedAtFunc: func(context.Context, string) (map[string]time.Time, error) {
			return nil, errors.New("boom")
		}}

		got := fetchedAt(ctx, fresh, "src")

		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestListUpdated(t *testing.T) {
	ctx := context.Background()
	older := time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	bySource := func(times map[string]time.Time) *MockFreshness {
		return &MockFreshness{FetchedAtFunc: func(_ context.Context, source string) (map[string]time.Time, error) {
			at, ok := times[source]
			if !ok {
				return nil, nil
			}
			return map[string]time.Time{"": at}, nil
		}}
	}

	t.Run("older of two sources", func(t *testing.T) {
		got := listUpdated(ctx, bySource(map[string]time.Time{"a": newer, "b": older}), "a", "b")

		require.NotNil(t, got)
		assert.True(t, older.Equal(*got))
	})

	t.Run("missing source is ignored", func(t *testing.T) {
		got := listUpdated(ctx, bySource(map[string]time.Time{"a": newer}), "a", "b")

		require.NotNil(t, got)
		assert.True(t, newer.Equal(*got))
	})

	t.Run("no source is nil", func(t *testing.T) {
		assert.Nil(t, listUpdated(ctx, bySource(nil), "a", "b"))
	})
}
