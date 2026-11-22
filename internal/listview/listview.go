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

type Window struct {
	Page     int
	PageSize int
	Start    int
	End      int
}

func PageSizeOrDefault(pageSize int) int {
	if pageSize < 1 {
		return DefaultPageSize
	}
	return pageSize
}

func Paginate(total, page, pageSize int) Window {
	pageSize = PageSizeOrDefault(pageSize)
	last := max((total+pageSize-1)/pageSize, 1)
	page = min(max(page, DefaultPage), last)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	return Window{Page: page, PageSize: pageSize, Start: start, End: end}
}

func Directed(c int, dir Dir) int {
	if dir == DirDesc {
		return -c
	}
	return c
}

func MissingLast(aMissing, bMissing bool) int {
	switch {
	case aMissing && bMissing:
		return 0
	case aMissing:
		return 1
	}
	return -1
}
