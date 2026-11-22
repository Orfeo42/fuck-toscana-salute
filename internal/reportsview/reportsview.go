package reportsview

import (
	"time"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

type viewError string

func (e viewError) Error() string { return string(e) }

const ErrNotFound = viewError("reportsview: report not found")

type Sort string

const (
	SortDate     Sort = "date"
	SortKind     Sort = "kind"
	SortProvider Sort = "provider"
)

type Query struct {
	Search   string
	Kind     string
	Sort     Sort
	Dir      listview.Dir
	Page     int
	PageSize int
}

type KindOption struct {
	Key   string
	Label string
	Count int
}

type Failure struct {
	Kind      string
	KindLabel string
	Message   string
}

type LinkedVisit struct {
	NRE   string
	Title string
}

type Row struct {
	ID          string
	Ref         referti.Ref
	Kind        string
	KindLabel   string
	Title       string
	CustomTitle string
	Date        time.Time
	EditDate    string
	UpdatedAt   time.Time
	Provider    string
	Unread      bool
	Hidden      bool
	Origin      reports.Origin
	Visits      []LinkedVisit
	LinkedLabel string
}

type Page struct {
	Rows           []Row
	Total          int
	Page           int
	PageSize       int
	Kinds          []KindOption
	Failures       []Failure
	FSEUnavailable bool
	Refreshing     bool
	Updated        listview.Updated
	Issue          refresh.Issue
}

type RowResult struct {
	Row        Row
	Refreshing bool
}
