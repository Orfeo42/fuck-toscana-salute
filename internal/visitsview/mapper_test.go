package visitsview

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuck-toscana-salute/internal/cup"
)

var (
	noPostalCode = cup.Unit{Name: "Unita esempio", Address: "Via Roma 1", City: "Firenze", Province: "FI"}
	withPostal   = cup.Unit{Name: "Unita esempio", Address: "Via Roma 1", PostalCode: "50100", City: "Firenze", Province: "FI"}
)

func TestFullAddress(t *testing.T) {
	t.Run("joins all parts", func(t *testing.T) {
		assert.Equal(t, "Via Roma 1, 50100 Firenze FI", fullAddress(withPostal))
	})

	t.Run("skips a missing postal code", func(t *testing.T) {
		assert.Equal(t, "Via Roma 1, Firenze FI", fullAddress(noPostalCode))
	})

	t.Run("skips an empty province", func(t *testing.T) {
		u := withPostal
		u.Province = ""
		assert.Equal(t, "Via Roma 1, 50100 Firenze", fullAddress(u))
	})

	t.Run("skips an empty address without a dangling comma", func(t *testing.T) {
		u := withPostal
		u.Address = ""
		assert.Equal(t, "50100 Firenze FI", fullAddress(u))
	})
}

func TestMapsDirectionsURL(t *testing.T) {
	t.Run("uses the full address only, even when a place is set", func(t *testing.T) {
		u := withPostal
		u.Place = "Ospedale Esempio"
		assert.Equal(t, "https://www.google.com/maps/dir/?api=1&destination=Via%20Roma%201%2C%2050100%20Firenze%20FI", mapsDirectionsURL(u))
	})

	t.Run("encodes special characters", func(t *testing.T) {
		u := withPostal
		u.Address = "Via A&B 2/c"
		assert.Equal(t, "https://www.google.com/maps/dir/?api=1&destination=Via%20A%26B%202%2Fc%2C%2050100%20Firenze%20FI", mapsDirectionsURL(u))
	})
}

func TestMapsSearchURL(t *testing.T) {
	t.Run("uses the full address when no place is set", func(t *testing.T) {
		assert.Equal(t, "https://www.google.com/maps/search/?api=1&query=Via%20Roma%201%2C%2050100%20Firenze%20FI", mapsSearchURL(withPostal))
	})

	t.Run("prefixes the place when set", func(t *testing.T) {
		u := withPostal
		u.Place = "Ospedale Esempio"
		assert.Equal(t, "https://www.google.com/maps/search/?api=1&query=Ospedale%20Esempio%2C%20Via%20Roma%201%2C%2050100%20Firenze%20FI", mapsSearchURL(u))
	})

	t.Run("handles a missing postal code and province", func(t *testing.T) {
		u := noPostalCode
		u.Province = ""
		assert.Equal(t, "https://www.google.com/maps/search/?api=1&query=Via%20Roma%201%2C%20Firenze", mapsSearchURL(u))
	})
}

func TestDisplayName(t *testing.T) {
	t.Run("prefers the description", func(t *testing.T) {
		assert.Equal(t, "Descrizione", displayName(cup.Unit{Name: "Nome", Description: "Descrizione"}))
	})

	t.Run("falls back to the name", func(t *testing.T) {
		assert.Equal(t, "Nome", displayName(cup.Unit{Name: "Nome"}))
	})
}

func TestCalendarFields(t *testing.T) {
	unit := cup.Unit{Name: "OSP ESEMPIO", Description: "Ospedale Esempio", Address: "Via Roma 1", City: "Firenze", Province: "FI"}

	t.Run("builds title, location and details", func(t *testing.T) {
		assert.Equal(t, CalendarFields{
			Title:    "Visita cardiologica",
			Location: "Ospedale Esempio, Via Roma 1, Firenze, FI",
			Details:  "Prenotazione CUP Regione Toscana",
		}, calendarFields(unit, "Visita cardiologica"))
	})

	t.Run("falls back to the generic title", func(t *testing.T) {
		assert.Equal(t, "Visita / esame", calendarFields(unit, "").Title)
	})

	t.Run("uses the unit name when the description is empty", func(t *testing.T) {
		assert.Equal(t, "OSP ESEMPIO", calendarFields(cup.Unit{Name: "OSP ESEMPIO"}, "x").Location)
	})

	t.Run("does not leak the NRE", func(t *testing.T) {
		f := calendarFields(unit, "Visita")
		assert.NotContains(t, f.Title+f.Location+f.Details, "0900A0000000001")
	})
}

func TestToAppointment(t *testing.T) {
	now := at("2026-03-10T09:00:00+01:00")

	t.Run("says oggi for the same Rome day", func(t *testing.T) {
		a := cup.Appointment{At: at("2026-03-10T23:30:00+01:00")}
		got := toAppointment(a, "t", now)
		assert.Equal(t, "oggi", got.RelativeDay)
		assert.True(t, got.Upcoming)
	})

	t.Run("says domani for the next day", func(t *testing.T) {
		assert.Equal(t, "domani", toAppointment(cup.Appointment{At: at("2026-03-11T08:00:00+01:00")}, "t", now).RelativeDay)
	})

	t.Run("counts future days", func(t *testing.T) {
		assert.Equal(t, "tra 5 giorni", toAppointment(cup.Appointment{At: at("2026-03-15T08:00:00+01:00")}, "t", now).RelativeDay)
	})

	t.Run("says ieri for the previous day and is not upcoming", func(t *testing.T) {
		got := toAppointment(cup.Appointment{At: at("2026-03-09T08:00:00+01:00")}, "t", now)
		assert.Equal(t, "ieri", got.RelativeDay)
		assert.False(t, got.Upcoming)
	})

	t.Run("counts past days", func(t *testing.T) {
		assert.Equal(t, "9 giorni fa", toAppointment(cup.Appointment{At: at("2026-03-01T08:00:00+01:00")}, "t", now).RelativeDay)
	})

	t.Run("uses the Rome calendar day across midnight UTC", func(t *testing.T) {
		assert.Equal(t, "domani", toAppointment(cup.Appointment{At: at("2026-03-10T23:30:00Z")}, "t", now).RelativeDay)
	})

	t.Run("leaves the relative day empty without a time", func(t *testing.T) {
		got := toAppointment(cup.Appointment{}, "t", now)
		assert.Empty(t, got.RelativeDay)
		assert.False(t, got.Upcoming)
	})
}
