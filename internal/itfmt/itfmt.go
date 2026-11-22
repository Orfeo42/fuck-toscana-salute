package itfmt

import (
	"fmt"
	"strconv"
	"time"
	_ "time/tzdata"
)

const (
	dayLayout      = "2006-01-02"
	timeLayout     = "15:04"
	shortLayout    = "02/01 15:04"
	dayMonthLayout = "02/01"
	hoursPerDay    = 24
)

var weekdays = [...]string{"dom", "lun", "mar", "mer", "gio", "ven", "sab"}

var months = [...]string{"gen", "feb", "mar", "apr", "mag", "giu", "lug", "ago", "set", "ott", "nov", "dic"}

var rome = location()

func location() *time.Location {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return time.UTC
	}
	return loc
}

func Date(t time.Time) string {
	t = t.In(rome)
	return fmt.Sprintf("%s %d %s %d", weekdays[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
}

func ShortDateTime(t time.Time) string {
	return t.In(rome).Format(shortLayout)
}

func Day(t time.Time) string {
	return t.In(rome).Format(dayLayout)
}

func UpdatedLabel(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	at = at.In(rome)
	if Day(at) == Day(now) {
		return "Aggiornato alle " + at.Format(timeLayout)
	}
	return "Aggiornato il " + at.Format(dayMonthLayout) + " " + at.Format(timeLayout)
}

func RelativeDay(at, now time.Time) string {
	days := dayNumber(at) - dayNumber(now)
	switch {
	case days == 0:
		return "oggi"
	case days == 1:
		return "domani"
	case days == -1:
		return "ieri"
	case days < 0:
		return strconv.Itoa(-days) + " giorni fa"
	}
	return "tra " + strconv.Itoa(days) + " giorni"
}

func dayNumber(t time.Time) int {
	y, m, d := t.In(rome).Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / (hoursPerDay * 3600))
}
