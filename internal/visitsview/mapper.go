package visitsview

import (
	"net/url"
	"strings"
	"time"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/textutil"
)

const (
	mapsSearchBase     = "https://www.google.com/maps/search/?api=1&query="
	mapsDirectionsBase = "https://www.google.com/maps/dir/?api=1&destination="
	defaultEventTitle  = "Visita / esame"
	eventDetails       = "Prenotazione CUP Regione Toscana"
)

func escapeComponent(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func displayName(u cup.Unit) string {
	if u.Description != "" {
		return u.Description
	}
	return u.Name
}

func fullAddress(u cup.Unit) string {
	locality := textutil.JoinNonEmpty(" ", u.PostalCode, u.City, u.Province)
	return textutil.JoinNonEmpty(", ", u.Address, locality)
}

func mapsSearchURL(u cup.Unit) string {
	return mapsSearchBase + escapeComponent(textutil.JoinNonEmpty(", ", u.Place, fullAddress(u)))
}

func mapsDirectionsURL(u cup.Unit) string {
	return mapsDirectionsBase + escapeComponent(fullAddress(u))
}

func toUnit(u cup.Unit) Unit {
	return Unit{
		Name:              u.Name,
		Description:       u.Description,
		DisplayName:       displayName(u),
		Place:             u.Place,
		Address:           u.Address,
		PostalCode:        u.PostalCode,
		City:              u.City,
		Province:          u.Province,
		FullAddress:       fullAddress(u),
		MapsSearchURL:     mapsSearchURL(u),
		MapsDirectionsURL: mapsDirectionsURL(u),
	}
}

func calendarFields(u cup.Unit, title string) CalendarFields {
	if title == "" {
		title = defaultEventTitle
	}
	place := textutil.JoinNonEmpty(", ", u.Address, u.City, u.Province)
	return CalendarFields{
		Title:    title,
		Location: textutil.JoinNonEmpty(", ", displayName(u), place),
		Details:  eventDetails,
	}
}

func toAppointment(a cup.Appointment, title string, now time.Time) *Appointment {
	out := &Appointment{
		ID:        a.ID,
		BookingID: a.BookingID,
		Status:    a.Status,
		At:        a.At,
		Unit:      toUnit(a.Unit),
		Calendar:  calendarFields(a.Unit, title),
	}
	if a.At.IsZero() {
		return out
	}
	out.RelativeDay = itfmt.RelativeDay(a.At, now)
	out.Upcoming = itfmt.Day(a.At) >= itfmt.Day(now)
	return out
}
