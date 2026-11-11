package dates

import (
	"fmt"
	"strings"
	"time"
)

type datesError string

func (e datesError) Error() string { return string(e) }

const (
	ErrEmpty         datesError = "dates: empty value"
	ErrUnknownFormat datesError = "dates: unknown format"
)

var layouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02/01/2006 15:04:05",
	"02/01/2006",
	"20060102",
}

func Parse(value string, loc *time.Location) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, ErrEmpty
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, v, loc)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: %q", ErrUnknownFormat, value)
}
