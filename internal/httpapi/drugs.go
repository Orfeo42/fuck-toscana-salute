package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/drugsview"
	"fuck-toscana-salute/internal/listview"
)

type drugsQueryInput struct {
	Body DrugsQueryRequest
}

type DrugsQueryRequest struct {
	Search          string   `json:"search,omitempty" maxLength:"100"`
	ExcludeStatuses []string `json:"excludeStatuses,omitempty" maxItems:"9" enum:"prescribed,booked,taken_in_charge,suspended,partially_done,done,done_after_cancel,cancelled,other" nullable:"false"`
	Sort            string   `json:"sort,omitempty" enum:"date,name,status,nre" default:"date"`
	Dir             string   `json:"dir,omitempty" enum:"asc,desc" default:"desc"`
	Page            int      `json:"page,omitempty" minimum:"1" maximum:"10000" default:"1"`
	PageSize        int      `json:"pageSize,omitempty" enum:"25,50,100" default:"50"`
	Focus           string   `json:"focus,omitempty" pattern:"^[0-9A-Za-z]{10,20}$"`
}

type drugsQueryOutput struct {
	Body DrugsQueryBody
}

type DrugsQueryBody struct {
	Rows             []DrugRowBody      `json:"rows" nullable:"false"`
	Total            int                `json:"total"`
	Page             int                `json:"page"`
	PageSize         int                `json:"pageSize"`
	Statuses         []StatusOptionBody `json:"statuses" nullable:"false"`
	ExcludedStatuses []string           `json:"excludedStatuses" nullable:"false"`
	Search           string             `json:"search"`
	Focus            *FocusBody         `json:"focus,omitempty"`
	Refreshing       bool               `json:"refreshing"`
	Updated          UpdatedBody        `json:"updated"`
	Issue            string             `json:"issue" enum:"none,login_required,unavailable"`
}

type DrugRowBody struct {
	NRE         string         `json:"nre"`
	Title       string         `json:"title"`
	Date        *time.Time     `json:"date,omitempty"`
	Status      DrugStatusBody `json:"status"`
	DocumentRef string         `json:"documentRef,omitempty"`
}

type DrugStatusBody struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Tone  string `json:"tone" enum:"booked,done,tobook,pending,closed,error"`
}

type drugRowInput struct {
	Body RowRequest
}

type drugRowOutput struct {
	Body DrugRowResponseBody
}

type DrugRowResponseBody struct {
	Row        DrugRowBody `json:"row"`
	Refreshing bool        `json:"refreshing"`
}

func registerDrugs(api huma.API, drugs DrugsView) {
	huma.Register(api, huma.Operation{
		OperationID:   "query-drugs",
		Method:        http.MethodPost,
		Path:          "/api/drugs/query",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *drugsQueryInput) (*drugsQueryOutput, error) {
		page, err := drugs.Query(ctx, drugsview.Query{
			Search:          in.Body.Search,
			ExcludeStatuses: in.Body.ExcludeStatuses,
			Sort:            drugsview.Sort(in.Body.Sort),
			Dir:             listview.Dir(in.Body.Dir),
			Page:            in.Body.Page,
			PageSize:        in.Body.PageSize,
			Focus:           in.Body.Focus,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "query-drugs", err)
		}
		return &drugsQueryOutput{Body: toDrugsQueryBody(page)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-drug-row",
		Method:        http.MethodPost,
		Path:          "/api/drugs/row",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *drugRowInput) (*drugRowOutput, error) {
		result, err := drugs.Row(ctx, in.Body.NRE)
		if err != nil {
			return nil, toHTTPError(ctx, "get-drug-row", err)
		}
		return &drugRowOutput{Body: DrugRowResponseBody{Row: toDrugRowBody(result.Row), Refreshing: result.Refreshing}}, nil
	})
}
