package itfmt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

func TestDate(t *testing.T) {
	t.Run("formats an ISO date in Rome time", func(t *testing.T) {
		assert.Equal(t, "ven 20 nov 2026", Date(mustParse(t, "2026-11-20T09:30:00+01:00")))
	})

	t.Run("uses the Rome day across the UTC boundary", func(t *testing.T) {
		assert.Equal(t, "ven 20 nov 2026", Date(mustParse(t, "2026-11-19T23:30:00Z")))
	})

	t.Run("does not pad the day", func(t *testing.T) {
		assert.Equal(t, "ven 2 gen 2026", Date(mustParse(t, "2026-01-02T10:00:00+01:00")))
	})

	t.Run("sunday and march", func(t *testing.T) {
		assert.Equal(t, "dom 8 mar 2026", Date(mustParse(t, "2026-03-08T10:00:00+01:00")))
	})
}

func TestShortDateTime(t *testing.T) {
	t.Run("formats day, month and time in Rome time", func(t *testing.T) {
		assert.Equal(t, "10/11 09:05", ShortDateTime(mustParse(t, "2026-11-10T08:05:00Z")))
	})

	t.Run("summer time", func(t *testing.T) {
		assert.Equal(t, "20/07 09:05", ShortDateTime(mustParse(t, "2026-07-20T07:05:00Z")))
	})

	t.Run("pads day and month", func(t *testing.T) {
		assert.Equal(t, "02/01 10:30", ShortDateTime(mustParse(t, "2026-01-02T10:30:00+01:00")))
	})
}

func TestDay(t *testing.T) {
	t.Run("formats the Rome calendar day", func(t *testing.T) {
		assert.Equal(t, "2026-03-11", Day(mustParse(t, "2026-03-10T23:30:00Z")))
	})

	t.Run("rome midnight stays on its own day", func(t *testing.T) {
		assert.Equal(t, "2026-03-10", Day(mustParse(t, "2026-03-10T00:00:00+01:00")))
	})
}

func TestUpdatedLabel(t *testing.T) {
	now := mustParse(t, "2026-11-10T12:00:00Z")

	t.Run("shows only the time for today", func(t *testing.T) {
		assert.Equal(t, "Aggiornato alle 09:05", UpdatedLabel(mustParse(t, "2026-11-10T08:05:00Z"), now))
	})

	t.Run("shows day, month and time for another day", func(t *testing.T) {
		assert.Equal(t, "Aggiornato il 09/11 09:05", UpdatedLabel(mustParse(t, "2026-11-09T08:05:00Z"), now))
	})

	t.Run("uses the Rome day across the UTC boundary", func(t *testing.T) {
		assert.Equal(t, "Aggiornato alle 00:30", UpdatedLabel(mustParse(t, "2026-11-09T23:30:00Z"), now))
	})

	t.Run("zero time gives an empty label", func(t *testing.T) {
		assert.Equal(t, "", UpdatedLabel(time.Time{}, now))
	})
}

func TestRelativeDay(t *testing.T) {
	now := mustParse(t, "2026-03-10T09:00:00+01:00")

	t.Run("says oggi for the same Rome day", func(t *testing.T) {
		assert.Equal(t, "oggi", RelativeDay(mustParse(t, "2026-03-10T23:30:00+01:00"), now))
	})

	t.Run("says domani for the next day", func(t *testing.T) {
		assert.Equal(t, "domani", RelativeDay(mustParse(t, "2026-03-11T08:00:00+01:00"), now))
	})

	t.Run("counts future days", func(t *testing.T) {
		assert.Equal(t, "tra 5 giorni", RelativeDay(mustParse(t, "2026-03-15T08:00:00+01:00"), now))
	})

	t.Run("says ieri for the previous day", func(t *testing.T) {
		assert.Equal(t, "ieri", RelativeDay(mustParse(t, "2026-03-09T08:00:00+01:00"), now))
	})

	t.Run("counts past days", func(t *testing.T) {
		assert.Equal(t, "9 giorni fa", RelativeDay(mustParse(t, "2026-03-01T08:00:00+01:00"), now))
	})

	t.Run("uses the Rome calendar day across midnight UTC", func(t *testing.T) {
		assert.Equal(t, "domani", RelativeDay(mustParse(t, "2026-03-10T23:30:00Z"), now))
	})

	t.Run("counts across the daylight saving change", func(t *testing.T) {
		assert.Equal(t, "tra 7 giorni", RelativeDay(mustParse(t, "2026-03-31T08:00:00+02:00"), mustParse(t, "2026-03-24T08:00:00+01:00")))
	})
}
