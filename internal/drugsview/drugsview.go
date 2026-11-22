package drugsview

import (
	"time"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/refresh"
)

type viewError string

func (e viewError) Error() string { return string(e) }

const ErrNotFound = viewError("drugsview: drug not found")

type Sort string

const (
	SortDate   Sort = "date"
	SortName   Sort = "name"
	SortStatus Sort = "status"
	SortNRE    Sort = "nre"
)

type Query struct {
	Search          string
	ExcludeStatuses []string
	Sort            Sort
	Dir             listview.Dir
	Page            int
	PageSize        int
	Focus           string
}

type Status struct {
	Key   string
	Label string
	Tone  string
}

type Row struct {
	NRE         string
	Title       string
	Date        time.Time
	Status      Status
	DocumentRef string
}

type Page struct {
	Rows             []Row
	Total            int
	Page             int
	PageSize         int
	Statuses         []listview.StatusOption
	ExcludedStatuses []string
	Search           string
	Focus            *listview.Focus
	Refreshing       bool
	Updated          listview.Updated
	Issue            refresh.Issue
}

type RowResult struct {
	Row        Row
	Refreshing bool
}
