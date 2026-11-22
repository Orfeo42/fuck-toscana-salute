package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/referti"
)

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
	Origin     string     `json:"origin" enum:"remote,local"`
	Title      string     `json:"title,omitempty"`
	NREs       []string   `json:"nres"`
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

func registerReports(api huma.API, reports Reports) {
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
		contentType, extension := "application/pdf", "pdf"
		switch detected := http.DetectContentType(pdf); detected {
		case "image/jpeg":
			contentType, extension = detected, "jpg"
		case "image/png":
			contentType, extension = detected, "png"
		}
		return &prescriptionPDFOutput{
			ContentType:        contentType,
			ContentDisposition: `inline; filename="referto.` + extension + `"`,
			Body:               pdf,
		}, nil
	})
}
