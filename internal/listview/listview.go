package listview

import "time"

type Dir string

const (
	DirAsc  Dir = "asc"
	DirDesc Dir = "desc"
)

const (
	ToneBooked  = "booked"
	ToneDone    = "done"
	ToneToBook  = "tobook"
	TonePending = "pending"
	ToneClosed  = "closed"
	ToneError   = "error"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 50
)

type StatusOption struct {
	Key      string
	Label    string
	Tone     string
	Count    int
	Excluded bool
}

type Updated struct {
	At    time.Time
	Label string
}

type Focus struct {
	NRE   string
	Found bool
}
