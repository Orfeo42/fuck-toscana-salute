package visitsview

import (
	"time"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

type viewError string

func (e viewError) Error() string { return string(e) }

const ErrNotFound = viewError("visitsview: visit not found")

type Sort string

const (
	SortPrescribed  Sort = "prescribed"
	SortTitle       Sort = "title"
	SortStatus      Sort = "status"
	SortAppointment Sort = "appointment"
	SortPlace       Sort = "place"
	SortNRE         Sort = "nre"
)

type Lookup string

const (
	LookupNone     Lookup = "none"
	LookupChecking Lookup = "checking"
	LookupFailed   Lookup = "failed"
	LookupDone     Lookup = "done"
)

type Kind string

const (
	KindBooked   Kind = "booked"
	KindOfficial Kind = "official"
	KindExpired  Kind = "expired"
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

type Issues struct {
	Lists        refresh.Issue
	Appointments refresh.Issue
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
	Issues           Issues
}

type Status struct {
	Key             string
	Label           string
	Tone            string
	Kind            Kind
	Code            int
	Manual          bool
	Note            string
	VerifiedAt      time.Time
	CanBook         bool
	ExpiresOn       time.Time
	ExpiryEstimated bool
	Details         []string
}

type Unit struct {
	Name              string
	Description       string
	DisplayName       string
	Place             string
	Address           string
	PostalCode        string
	City              string
	Province          string
	FullAddress       string
	MapsSearchURL     string
	MapsDirectionsURL string
}

type CalendarFields struct {
	Title    string
	Location string
	Details  string
}

type Appointment struct {
	ID          string
	BookingID   string
	Status      string
	At          time.Time
	RelativeDay string
	Upcoming    bool
	Unit        Unit
	Calendar    CalendarFields
}

type LinkedReport struct {
	Ref       referti.Ref
	Label     string
	Date      time.Time
	Provider  string
	Origin    reports.Origin
	Available bool
}

type Checked struct {
	Appointments time.Time
	Verdict      time.Time
}

type UploadDefaults struct {
	Title    string
	Date     string
	Provider string
}

type Row struct {
	NRE               string
	Title             string
	PrescribedOn      time.Time
	DocumentRef       string
	Status            Status
	Appointment       *Appointment
	AppointmentLookup Lookup
	Reports           []LinkedReport
	CheckedAt         Checked
	Upload            UploadDefaults
	Refreshing        bool
}

type ManualOption struct {
	Code  int
	Label string
}

type ManualStatus struct {
	Current *int
	Options []ManualOption
}

type RowResult struct {
	Row          Row
	ManualStatus ManualStatus
	Refreshing   bool
	Issues       Issues
}

type ReportOption struct {
	Ref      referti.Ref
	Label    string
	Date     time.Time
	Provider string
	Origin   reports.Origin
}
