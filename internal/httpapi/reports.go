package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/referti"
)

type reportsOutput struct {
	Body ReportsBody
}

type ReportsBody struct {
	Reports  []ReportBody        `json:"reports"`
	Failures []ReportFailureBody `json:"failures"`
	Updated  *time.Time          `json:"updated" nullable:"true"`
}

type ReportBody struct {
	Kind       string     `json:"kind"`
	KindLabel  string     `json:"kindLabel"`
	AbstractID string     `json:"abstractId"`
	DetailID   string     `json:"detailId"`
	RepCode    string     `json:"repCode"`
	Date       *time.Time `json:"date,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	Provider   string     `json:"provider"`
	Unread     bool       `json:"unread"`
	Hidden     bool       `json:"hidden"`
}

type ReportFailureBody struct {
	Kind      string `json:"kind"`
	KindLabel string `json:"kindLabel"`
	Message   string `json:"message"`
}

type ReportRefRequest struct {
	Kind       string `json:"kind" required:"true" maxLength:"20"`
	AbstractID string `json:"abstractId,omitempty" maxLength:"200"`
	DetailID   string `json:"detailId,omitempty" maxLength:"200"`
	RepCode    string `json:"repCode,omitempty" maxLength:"200"`
}

type reportDocumentsInput struct {
	Body ReportRefRequest
}

type reportDocumentsOutput struct {
	Body ReportDocumentsBody
}

type ReportDocumentsBody struct {
	Documents []ReportDocumentBody `json:"documents"`
}

type ReportDocumentBody struct {
	Position   string     `json:"position"`
	Label      string     `json:"label"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	Provider   string     `json:"provider,omitempty"`
}

type reportPDFInput struct {
	Body ReportPDFRequest
}

type ReportPDFRequest struct {
	Kind       string `json:"kind" required:"true" maxLength:"20"`
	AbstractID string `json:"abstractId,omitempty" maxLength:"200"`
	DetailID   string `json:"detailId,omitempty" maxLength:"200"`
	RepCode    string `json:"repCode,omitempty" maxLength:"200"`
	Position   string `json:"position,omitempty" maxLength:"200"`
}

func registerReports(api huma.API, reports Reports, fresh Freshness) {
	huma.Register(api, huma.Operation{
		OperationID:   "list-reports",
		Method:        http.MethodGet,
		Path:          "/api/reports",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*reportsOutput, error) {
		listing, err := reports.List(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list-reports", err)
		}
		body := ReportsBody{
			Reports:  make([]ReportBody, 0, len(listing.Reports)),
			Failures: make([]ReportFailureBody, 0, len(listing.Failures)),
			Updated:  listUpdated(ctx, fresh, cache.SourceFSEReports),
		}
		for _, r := range listing.Reports {
			body.Reports = append(body.Reports, ReportBody{
				Kind:       string(r.Ref.Kind),
				KindLabel:  r.Ref.Kind.Label(),
				AbstractID: r.Ref.AbstractID,
				DetailID:   r.Ref.DetailID,
				RepCode:    r.Ref.RepCode,
				Date:       timePtr(r.Date),
				UpdatedAt:  timePtr(r.UpdatedAt),
				Provider:   r.Provider,
				Unread:     r.Unread,
				Hidden:     r.Hidden,
			})
		}
		for _, f := range listing.Failures {
			body.Failures = append(body.Failures, ReportFailureBody{Kind: string(f.Kind), KindLabel: f.Kind.Label(), Message: f.Err.Error()})
		}
		return &reportsOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "list-report-documents",
		Method:        http.MethodPost,
		Path:          "/api/reports/documents",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *reportDocumentsInput) (*reportDocumentsOutput, error) {
		kind, err := referti.ParseKind(in.Body.Kind)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("unknown report kind")
		}
		ref := referti.Ref{Kind: kind, AbstractID: in.Body.AbstractID, DetailID: in.Body.DetailID, RepCode: in.Body.RepCode}
		docs, err := reports.Documents(ctx, ref)
		if err != nil {
			return nil, toHTTPError(ctx, "list-report-documents", err)
		}
		body := ReportDocumentsBody{Documents: make([]ReportDocumentBody, 0, len(docs))}
		for _, d := range docs {
			body.Documents = append(body.Documents, ReportDocumentBody{Position: d.Position, Label: d.Label, AcceptedAt: timePtr(d.AcceptedAt), Provider: d.Provider})
		}
		return &reportDocumentsOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-report-pdf",
		Method:        http.MethodPost,
		Path:          "/api/reports/pdf",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *reportPDFInput) (*prescriptionPDFOutput, error) {
		kind, err := referti.ParseKind(in.Body.Kind)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("unknown report kind")
		}
		ref := referti.Ref{Kind: kind, AbstractID: in.Body.AbstractID, DetailID: in.Body.DetailID, RepCode: in.Body.RepCode}
		pdf, err := reports.Download(ctx, ref, in.Body.Position)
		if err != nil {
			return nil, toHTTPError(ctx, "get-report-pdf", err)
		}
		return &prescriptionPDFOutput{
			ContentType:        "application/pdf",
			ContentDisposition: `inline; filename="referto.pdf"`,
			Body:               pdf,
		}, nil
	})
}
