package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/reportsview"
)

type reportsQueryInput struct {
	Body ReportsQueryRequest
}

type ReportsQueryRequest struct {
	Search   string `json:"search,omitempty" maxLength:"100"`
	Kind     string `json:"kind,omitempty" enum:"lab,labtrasf,ris,ldo,ps,rsa,rap,local"`
	Sort     string `json:"sort,omitempty" enum:"date,kind,provider" default:"date"`
	Dir      string `json:"dir,omitempty" enum:"asc,desc" default:"desc"`
	Page     int    `json:"page,omitempty" minimum:"1" maximum:"10000" default:"1"`
	PageSize int    `json:"pageSize,omitempty" enum:"25,50,100" default:"50"`
}

type reportsQueryOutput struct {
	Body ReportsQueryBody
}

type ReportsQueryBody struct {
	Rows           []ReportRowBody     `json:"rows" nullable:"false"`
	Total          int                 `json:"total"`
	Page           int                 `json:"page"`
	PageSize       int                 `json:"pageSize"`
	Kinds          []KindOptionBody    `json:"kinds" nullable:"false"`
	Failures       []ReportFailureBody `json:"failures" nullable:"false"`
	FSEUnavailable bool                `json:"fseUnavailable"`
	Refreshing     bool                `json:"refreshing"`
	Updated        UpdatedBody         `json:"updated"`
	Issue          string              `json:"issue" enum:"none,login_required,unavailable"`
}

type KindOptionBody struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count" minimum:"0"`
}

type ReportRowBody struct {
	ID          string            `json:"id"`
	Ref         ReportRefBody     `json:"ref"`
	Kind        string            `json:"kind"`
	KindLabel   string            `json:"kindLabel"`
	Title       string            `json:"title"`
	CustomTitle string            `json:"customTitle,omitempty"`
	Date        *time.Time        `json:"date,omitempty"`
	EditDate    string            `json:"editDate,omitempty"`
	UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
	Provider    string            `json:"provider"`
	Unread      bool              `json:"unread"`
	Hidden      bool              `json:"hidden"`
	Origin      string            `json:"origin" enum:"remote,local"`
	Visits      []LinkedVisitBody `json:"visits" nullable:"false"`
	LinkedLabel string            `json:"linkedLabel"`
}

type LinkedVisitBody struct {
	NRE   string `json:"nre"`
	Title string `json:"title"`
}

type ReportRowRequest struct {
	Ref ReportRefRequest `json:"ref" required:"true"`
}

type reportRowInput struct {
	Body ReportRowRequest
}

type reportRowOutput struct {
	Body ReportRowResponseBody
}

type ReportRowResponseBody struct {
	Row        ReportRowBody `json:"row"`
	Refreshing bool          `json:"refreshing"`
}

type reportVisitOptionsInput struct {
	Body ReportRowRequest
}

type reportVisitOptionsOutput struct {
	Body ReportVisitOptionsBody
}

type ReportVisitOptionsBody struct {
	Options []LinkedVisitBody `json:"options" nullable:"false"`
}

func toReferti(ref ReportRefRequest) (referti.Ref, error) {
	kind, err := referti.ParseKind(ref.Kind)
	if err != nil {
		return referti.Ref{}, huma.Error422UnprocessableEntity("unknown report kind")
	}
	return referti.Ref{Kind: kind, AbstractID: ref.AbstractID, DetailID: ref.DetailID, RepCode: ref.RepCode}, nil
}

func registerReportsQuery(api huma.API, view ReportsView) {
	huma.Register(api, huma.Operation{
		OperationID:   "query-reports",
		Method:        http.MethodPost,
		Path:          "/api/reports/query",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *reportsQueryInput) (*reportsQueryOutput, error) {
		page, err := view.Query(ctx, reportsview.Query{
			Search:   in.Body.Search,
			Kind:     in.Body.Kind,
			Sort:     reportsview.Sort(in.Body.Sort),
			Dir:      listview.Dir(in.Body.Dir),
			Page:     in.Body.Page,
			PageSize: in.Body.PageSize,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "query-reports", err)
		}
		return &reportsQueryOutput{Body: toReportsQueryBody(page)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-report-row",
		Method:        http.MethodPost,
		Path:          "/api/reports/row",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *reportRowInput) (*reportRowOutput, error) {
		ref, err := toReferti(in.Body.Ref)
		if err != nil {
			return nil, err
		}
		result, err := view.Row(ctx, ref)
		if err != nil {
			return nil, toHTTPError(ctx, "get-report-row", err)
		}
		return &reportRowOutput{Body: ReportRowResponseBody{Row: toReportRowBody(result.Row), Refreshing: result.Refreshing}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "report-visit-options",
		Method:        http.MethodPost,
		Path:          "/api/reports/visit-options",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *reportVisitOptionsInput) (*reportVisitOptionsOutput, error) {
		ref, err := toReferti(in.Body.Ref)
		if err != nil {
			return nil, err
		}
		options, err := view.VisitOptions(ctx, ref)
		if err != nil {
			return nil, toHTTPError(ctx, "report-visit-options", err)
		}
		return &reportVisitOptionsOutput{Body: ReportVisitOptionsBody{Options: toLinkedVisitBodies(options)}}, nil
	})
}
