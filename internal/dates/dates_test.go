package dates

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	rome, err := time.LoadLocation("Europe/Rome")
	require.NoError(t, err)

	t.Run("RFC3339 with offset", func(t *testing.T) {
		got, err := Parse("2026-03-04T10:20:30+01:00", time.UTC)
		require.NoError(t, err)
		assert.True(t, time.Date(2026, 3, 4, 9, 20, 30, 0, time.UTC).Equal(got))
	})

	t.Run("ISO datetime with T", func(t *testing.T) {
		got, err := Parse("2026-03-04T10:20:30", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 10, 20, 30, 0, time.UTC), got)
	})

	t.Run("ISO datetime with space", func(t *testing.T) {
		got, err := Parse("2026-03-04 10:20:30", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 10, 20, 30, 0, time.UTC), got)
	})

	t.Run("ISO date", func(t *testing.T) {
		got, err := Parse("2026-03-04", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), got)
	})

	t.Run("Italian datetime", func(t *testing.T) {
		got, err := Parse("04/03/2026 10:20:30", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 10, 20, 30, 0, time.UTC), got)
	})

	t.Run("Italian date", func(t *testing.T) {
		got, err := Parse("04/03/2026", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), got)
	})

	t.Run("compact date", func(t *testing.T) {
		got, err := Parse("20260304", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), got)
	})

	t.Run("surrounding spaces trimmed", func(t *testing.T) {
		got, err := Parse("  2026-03-04  ", time.UTC)
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), got)
	})

	t.Run("empty returns ErrEmpty", func(t *testing.T) {
		_, err := Parse("   ", time.UTC)
		require.ErrorIs(t, err, ErrEmpty)
	})

	t.Run("unknown format returns ErrUnknownFormat", func(t *testing.T) {
		_, err := Parse("not a date", time.UTC)
		require.ErrorIs(t, err, ErrUnknownFormat)
	})

	t.Run("location applied to date-only value", func(t *testing.T) {
		got, err := Parse("2026-07-01", rome)
		require.NoError(t, err)
		_, offset := got.Zone()
		assert.Equal(t, 2*3600, offset)
		assert.Equal(t, rome, got.Location())
	})
}
