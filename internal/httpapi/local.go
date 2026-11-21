package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/referti"
)

const dateLayout = "2006-01-02"

var nrePattern = regexp.MustCompile(`^[0-9A-Za-z]{10,20}$`)

func parseMeta(title, date, provider string) (localrecords.Meta, error) {
	meta := localrecords.Meta{Title: title, Provider: provider}
	if date == "" {
		return meta, nil
	}
	parsed, err := time.ParseInLocation(dateLayout, date, time.UTC)
	if err != nil {
		return localrecords.Meta{}, huma.Error422UnprocessableEntity("Data non valida")
	}
	meta.Date = parsed
	return meta, nil
}

func linkRef(body LinkRequest) (referti.Ref, error) {
	kind, err := referti.ParseKind(body.Report.Kind)
	if err != nil {
		return referti.Ref{}, huma.Error422UnprocessableEntity("unknown report kind")
	}
	return referti.Ref{Kind: kind, AbstractID: body.Report.AbstractID, DetailID: body.Report.DetailID, RepCode: body.Report.RepCode}, nil
}

const uploadMaxBodyBytes = 21 << 20

type UploadReportForm struct {
	File     huma.FormFile `form:"file" contentType:"application/pdf,image/jpeg,image/png" required:"true"`
	Title    string        `form:"title" required:"false"`
	Date     string        `form:"date" required:"false"`
	Provider string        `form:"provider" required:"false"`
	NRE      string        `form:"nre" required:"false"`
}

type uploadReportInput struct {
	RawBody huma.MultipartFormFiles[UploadReportForm]
}

type localReportOutput struct {
	Body ReportBody
}

type importDriveInput struct {
	Body ImportDriveRequest
}

type ImportDriveRequest struct {
	FileID   string `json:"fileId" required:"true" minLength:"10" maxLength:"200" pattern:"^[A-Za-z0-9_-]+$"`
	Title    string `json:"title,omitempty" maxLength:"200"`
	Date     string `json:"date,omitempty" format:"date"`
	Provider string `json:"provider,omitempty" maxLength:"200"`
	NRE      string `json:"nre,omitempty" pattern:"^[0-9A-Za-z]{10,20}$"`
}

type updateLocalReportInput struct {
	Body UpdateLocalReportRequest
}

type UpdateLocalReportRequest struct {
	ID       string `json:"id" required:"true" pattern:"^[0-9a-f]{32}$"`
	Title    string `json:"title" required:"true" minLength:"1" maxLength:"200"`
	Date     string `json:"date,omitempty" format:"date"`
	Provider string `json:"provider,omitempty" maxLength:"200"`
}

type deleteLocalReportInput struct {
	Body LocalReportIDRequest
}

type LocalReportIDRequest struct {
	ID string `json:"id" required:"true" pattern:"^[0-9a-f]{32}$"`
}

type linkInput struct {
	Body LinkRequest
}

type LinkRequest struct {
	NRE    string           `json:"nre" required:"true" pattern:"^[0-9A-Za-z]{10,20}$"`
	Report ReportRefRequest `json:"report" required:"true"`
}

type setVisitStatusInput struct {
	Body VisitStatusRequest
}

type VisitStatusRequest struct {
	NRE    string `json:"nre" required:"true" pattern:"^[0-9A-Za-z]{10,20}$"`
	Status int    `json:"status" required:"true" enum:"3,30,5,6,7,8,9,4"`
}

type resetVisitStatusInput struct {
	Body VisitNRERequest
}

type VisitNRERequest struct {
	NRE string `json:"nre" required:"true" pattern:"^[0-9A-Za-z]{10,20}$"`
}

type pickerOutput struct {
	Body PickerBody
}

type PickerBody struct {
	AccessToken string `json:"accessToken"`
	APIKey      string `json:"apiKey"`
	AppID       string `json:"appId"`
}

func registerLocal(api huma.API, local LocalRecords, g Google) {
	huma.Register(api, huma.Operation{
		OperationID:   "upload-local-report",
		Method:        http.MethodPost,
		Path:          "/api/reports/local",
		DefaultStatus: http.StatusCreated,
		MaxBodyBytes:  uploadMaxBodyBytes,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *uploadReportInput) (*localReportOutput, error) {
		form := in.RawBody.Data()
		meta, err := parseMeta(form.Title, form.Date, form.Provider)
		if err != nil {
			return nil, err
		}
		nre := strings.ToUpper(form.NRE)
		if nre != "" && !nrePattern.MatchString(nre) {
			return nil, huma.Error422UnprocessableEntity("NRE non valido")
		}
		data, err := io.ReadAll(io.LimitReader(form.File, localrecords.MaxFileBytes+1))
		if err != nil {
			return nil, toHTTPError(ctx, "upload-local-report", err)
		}
		if err := form.File.Close(); err != nil {
			return nil, toHTTPError(ctx, "upload-local-report", err)
		}
		created, err := local.Create(ctx, localrecords.NewReport{Meta: meta, Filename: form.File.Filename, Origin: localrecords.OriginUpload, Data: data, NRE: nre})
		if err != nil {
			return nil, toHTTPError(ctx, "upload-local-report", err)
		}
		slog.InfoContext(ctx, "local report created", slog.String("origin", string(localrecords.OriginUpload)))
		return &localReportOutput{Body: toLocalReportBody(created, nre)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "import-drive-report",
		Method:        http.MethodPost,
		Path:          "/api/reports/local/drive",
		DefaultStatus: http.StatusCreated,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *importDriveInput) (*localReportOutput, error) {
		meta, err := parseMeta(in.Body.Title, in.Body.Date, in.Body.Provider)
		if err != nil {
			return nil, err
		}
		nre := strings.ToUpper(in.Body.NRE)
		file, err := g.DownloadFile(ctx, in.Body.FileID)
		if err != nil {
			return nil, toHTTPError(ctx, "import-drive-report", err)
		}
		created, err := local.Create(ctx, localrecords.NewReport{Meta: meta, Filename: file.Name, Origin: localrecords.OriginDrive, Data: file.Data, NRE: nre})
		if err != nil {
			return nil, toHTTPError(ctx, "import-drive-report", err)
		}
		slog.InfoContext(ctx, "local report created", slog.String("origin", string(localrecords.OriginDrive)))
		return &localReportOutput{Body: toLocalReportBody(created, nre)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "update-local-report",
		Method:        http.MethodPut,
		Path:          "/api/reports/local",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *updateLocalReportInput) (*emptyOutput, error) {
		meta, err := parseMeta(in.Body.Title, in.Body.Date, in.Body.Provider)
		if err != nil {
			return nil, err
		}
		if err := local.Update(ctx, in.Body.ID, meta); err != nil {
			return nil, toHTTPError(ctx, "update-local-report", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-local-report",
		Method:        http.MethodDelete,
		Path:          "/api/reports/local",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *deleteLocalReportInput) (*emptyOutput, error) {
		if err := local.Delete(ctx, in.Body.ID); err != nil {
			return nil, toHTTPError(ctx, "delete-local-report", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "link-report",
		Method:        http.MethodPut,
		Path:          "/api/links",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *linkInput) (*emptyOutput, error) {
		ref, err := linkRef(in.Body)
		if err != nil {
			return nil, err
		}
		if err := local.Link(ctx, strings.ToUpper(in.Body.NRE), ref); err != nil {
			return nil, toHTTPError(ctx, "link-report", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "unlink-report",
		Method:        http.MethodDelete,
		Path:          "/api/links",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *linkInput) (*emptyOutput, error) {
		ref, err := linkRef(in.Body)
		if err != nil {
			return nil, err
		}
		if err := local.Unlink(ctx, strings.ToUpper(in.Body.NRE), ref); err != nil {
			return nil, toHTTPError(ctx, "unlink-report", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "set-visit-status",
		Method:        http.MethodPut,
		Path:          "/api/visits/status",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *setVisitStatusInput) (*emptyOutput, error) {
		if err := local.SetStatus(ctx, strings.ToUpper(in.Body.NRE), in.Body.Status); err != nil {
			return nil, toHTTPError(ctx, "set-visit-status", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "reset-visit-status",
		Method:        http.MethodDelete,
		Path:          "/api/visits/status",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *resetVisitStatusInput) (*emptyOutput, error) {
		if err := local.ResetStatus(ctx, strings.ToUpper(in.Body.NRE)); err != nil {
			return nil, toHTTPError(ctx, "reset-visit-status", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "google-picker",
		Method:        http.MethodPost,
		Path:          "/api/google/picker",
		DefaultStatus: http.StatusOK,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, _ *struct{}) (*pickerOutput, error) {
		cfg, err := g.PickerConfig(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "google-picker", err)
		}
		return &pickerOutput{Body: PickerBody{AccessToken: cfg.AccessToken, APIKey: cfg.APIKey, AppID: cfg.AppID}}, nil
	})
}
