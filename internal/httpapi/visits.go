package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/visitsview"
)

type visitsQueryInput struct {
	Body VisitsQueryRequest
}

type VisitsQueryRequest struct {
	Search          string   `json:"search,omitempty" maxLength:"100"`
	ExcludeStatuses []string `json:"excludeStatuses,omitempty" maxItems:"12" enum:"booked,prescribed,unverified,handled,taken_in_charge,partially_done,suspended,done,done_after_cancel,cancelled,expired,other" nullable:"false"`
	Sort            string   `json:"sort,omitempty" enum:"prescribed,title,status,appointment,place,nre" default:"prescribed"`
	Dir             string   `json:"dir,omitempty" enum:"asc,desc" default:"desc"`
	Page            int      `json:"page,omitempty" minimum:"1" maximum:"10000" default:"1"`
	PageSize        int      `json:"pageSize,omitempty" enum:"25,50,100" default:"50"`
	Focus           string   `json:"focus,omitempty" pattern:"^[0-9A-Za-z]{10,20}$"`
}

type visitsQueryOutput struct {
	Body VisitsQueryBody
}

type VisitsQueryBody struct {
	Rows             []VisitRowBody     `json:"rows" nullable:"false"`
	Total            int                `json:"total"`
	Page             int                `json:"page"`
	PageSize         int                `json:"pageSize"`
	Statuses         []StatusOptionBody `json:"statuses" nullable:"false"`
	ExcludedStatuses []string           `json:"excludedStatuses" nullable:"false"`
	Search           string             `json:"search"`
	Focus            *FocusBody         `json:"focus,omitempty"`
	Refreshing       bool               `json:"refreshing"`
	Updated          UpdatedBody        `json:"updated"`
	Issues           VisitsIssuesBody   `json:"issues"`
}

type VisitsIssuesBody struct {
	Lists        string `json:"lists" enum:"none,login_required,unavailable"`
	Appointments string `json:"appointments" enum:"none,login_required,gestione_required,unavailable"`
}

type VisitRowBody struct {
	NRE               string                `json:"nre"`
	Title             string                `json:"title"`
	PrescribedOn      *time.Time            `json:"prescribedOn,omitempty"`
	DocumentRef       string                `json:"documentRef,omitempty"`
	Status            VisitStatusBody       `json:"status"`
	Appointment       *VisitAppointmentBody `json:"appointment,omitempty"`
	AppointmentLookup string                `json:"appointmentLookup" enum:"none,checking,failed,done"`
	Reports           []LinkedReportBody    `json:"reports" nullable:"false"`
	CheckedAt         VisitCheckedBody      `json:"checkedAt"`
	Upload            UploadDefaultsBody    `json:"upload"`
	Refreshing        bool                  `json:"refreshing"`
}

type VisitStatusBody struct {
	Key             string     `json:"key"`
	Label           string     `json:"label"`
	Tone            string     `json:"tone" enum:"booked,done,tobook,pending,closed,error"`
	Kind            string     `json:"kind" enum:"booked,official,expired"`
	Code            int        `json:"code,omitempty"`
	Manual          bool       `json:"manual"`
	Note            string     `json:"note,omitempty"`
	VerifiedAt      *time.Time `json:"verifiedAt,omitempty"`
	CanBook         bool       `json:"canBook"`
	ExpiresOn       *time.Time `json:"expiresOn,omitempty"`
	ExpiryEstimated bool       `json:"expiryEstimated"`
	Details         []string   `json:"details" nullable:"false"`
}

type VisitAppointmentBody struct {
	ID          string             `json:"id"`
	BookingID   string             `json:"bookingId,omitempty"`
	Status      string             `json:"status"`
	At          *time.Time         `json:"at,omitempty"`
	RelativeDay string             `json:"relativeDay,omitempty"`
	Upcoming    bool               `json:"upcoming"`
	Unit        VisitUnitBody      `json:"unit"`
	Calendar    CalendarFieldsBody `json:"calendar"`
}

type VisitUnitBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	DisplayName       string `json:"displayName"`
	Place             string `json:"place,omitempty"`
	Address           string `json:"address"`
	PostalCode        string `json:"postalCode,omitempty"`
	City              string `json:"city"`
	Province          string `json:"province"`
	FullAddress       string `json:"fullAddress"`
	MapsSearchURL     string `json:"mapsSearchUrl"`
	MapsDirectionsURL string `json:"mapsDirectionsUrl"`
}

type CalendarFieldsBody struct {
	Title    string `json:"title"`
	Location string `json:"location"`
	Details  string `json:"details"`
}

type LinkedReportBody struct {
	Ref       ReportRefBody `json:"ref"`
	Label     string        `json:"label"`
	Date      *time.Time    `json:"date,omitempty"`
	Provider  string        `json:"provider"`
	Origin    string        `json:"origin" enum:"remote,local"`
	Available bool          `json:"available"`
}

type VisitCheckedBody struct {
	Appointments *time.Time `json:"appointments" nullable:"true"`
	Verdict      *time.Time `json:"verdict" nullable:"true"`
}

type UploadDefaultsBody struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	Provider string `json:"provider"`
}

type visitRowInput struct {
	Body RowRequest
}

type visitRowOutput struct {
	Body VisitRowResponseBody
}

type VisitRowResponseBody struct {
	Row          VisitRowBody     `json:"row"`
	ManualStatus ManualStatusBody `json:"manualStatus"`
	Refreshing   bool             `json:"refreshing"`
	Issues       VisitsIssuesBody `json:"issues"`
}

type ManualStatusBody struct {
	Current *int                     `json:"current" nullable:"true"`
	Options []ManualStatusOptionBody `json:"options" nullable:"false"`
}

type ManualStatusOptionBody struct {
	Code  int    `json:"code"`
	Label string `json:"label"`
}

type visitReportOptionsInput struct {
	Body RowRequest
}

type visitReportOptionsOutput struct {
	Body VisitReportOptionsBody
}

type VisitReportOptionsBody struct {
	Options []ReportOptionBody `json:"options" nullable:"false"`
}

type ReportOptionBody struct {
	Ref      ReportRefBody `json:"ref"`
	Label    string        `json:"label"`
	Date     *time.Time    `json:"date,omitempty"`
	Provider string        `json:"provider"`
	Origin   string        `json:"origin" enum:"remote,local"`
}

func registerVisits(api huma.API, visits VisitsView) {
	huma.Register(api, huma.Operation{
		OperationID:   "query-visits",
		Method:        http.MethodPost,
		Path:          "/api/visits/query",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *visitsQueryInput) (*visitsQueryOutput, error) {
		page, err := visits.Query(ctx, visitsview.Query{
			Search:          in.Body.Search,
			ExcludeStatuses: in.Body.ExcludeStatuses,
			Sort:            visitsview.Sort(in.Body.Sort),
			Dir:             listview.Dir(in.Body.Dir),
			Page:            in.Body.Page,
			PageSize:        in.Body.PageSize,
			Focus:           in.Body.Focus,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "query-visits", err)
		}
		return &visitsQueryOutput{Body: toVisitsQueryBody(page)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-visit-row",
		Method:        http.MethodPost,
		Path:          "/api/visits/row",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *visitRowInput) (*visitRowOutput, error) {
		result, err := visits.Row(ctx, in.Body.NRE)
		if err != nil {
			return nil, toHTTPError(ctx, "get-visit-row", err)
		}
		return &visitRowOutput{Body: toVisitRowResponseBody(result)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "visit-report-options",
		Method:        http.MethodPost,
		Path:          "/api/visits/report-options",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *visitReportOptionsInput) (*visitReportOptionsOutput, error) {
		options, err := visits.ReportOptions(ctx, in.Body.NRE)
		if err != nil {
			return nil, toHTTPError(ctx, "visit-report-options", err)
		}
		return &visitReportOptionsOutput{Body: toVisitReportOptionsBody(options)}, nil
	})
}
