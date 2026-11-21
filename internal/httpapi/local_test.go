package httpapi

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/referti"
)

const (
	testOrigin = "Origin: http://example.com"
	testHost   = "Host: example.com"
	testNRE    = "0900A0000000001"
	testID     = "0123456789abcdef0123456789abcdef"
)

func newLocalAPI(t *testing.T, local LocalRecords, g Google) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerLocal(api, local, g)
	return api
}

func multipartBody(t *testing.T, fields map[string]string, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	part, err := w.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for name, value := range fields {
		require.NoError(t, w.WriteField(name, value))
	}
	require.NoError(t, w.Close())
	return &buf, "Content-Type: " + w.FormDataContentType()
}

func createdReport() localrecords.Report {
	return localrecords.Report{
		ID:        testID,
		Meta:      localrecords.Meta{Title: "Visita", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Provider: "ASL"},
		Filename:  "referto.pdf",
		MimeType:  "application/pdf",
		Origin:    localrecords.OriginUpload,
		CreatedAt: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestRegisterUploadLocalReport(t *testing.T) {
	t.Run("creates the report with nre and returns 201", func(t *testing.T) {
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return createdReport(), nil
		}}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, map[string]string{"title": "Visita", "date": "2026-10-01", "provider": "ASL", "nre": strings.ToLower(testNRE)}, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		require.Equal(t, http.StatusCreated, resp.Code)
		assert.JSONEq(t, `{"kind":"local","kindLabel":"Caricato","abstractId":"`+testID+`","detailId":"","repCode":"","date":"2026-10-01T00:00:00Z","updatedAt":"2026-11-01T10:00:00Z","provider":"ASL","unread":false,"hidden":false,"origin":"local","title":"Visita","nres":["`+testNRE+`"]}`, resp.Body.String())
		require.Len(t, local.CreateCalls(), 1)
		got := local.CreateCalls()[0].Nr
		assert.Equal(t, localrecords.OriginUpload, got.Origin)
		assert.Equal(t, "referto.pdf", got.Filename)
		assert.Equal(t, testNRE, got.NRE)
		assert.Equal(t, []byte("%PDF-1.7"), got.Data)
		assert.Equal(t, localrecords.Meta{Title: "Visita", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Provider: "ASL"}, got.Meta)
	})

	t.Run("creates the report without nre", func(t *testing.T) {
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return createdReport(), nil
		}}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, nil, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		require.Equal(t, http.StatusCreated, resp.Code)
		assert.Contains(t, resp.Body.String(), `"nres":[]`)
		assert.True(t, local.CreateCalls()[0].Nr.Meta.Date.IsZero())
	})

	t.Run("invalid nre is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, map[string]string{"nre": "short"}, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.CreateCalls())
	})

	t.Run("invalid date is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, map[string]string{"date": "01/10/2026"}, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.CreateCalls())
	})

	t.Run("too large is 413", func(t *testing.T) {
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return localrecords.Report{}, localrecords.ErrTooLarge
		}}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, nil, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
	})

	t.Run("reads at most the maximum plus one byte", func(t *testing.T) {
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return localrecords.Report{}, localrecords.ErrTooLarge
		}}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, nil, "referto.pdf", "application/pdf", bytes.Repeat([]byte("a"), localrecords.MaxFileBytes+10))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
		require.Len(t, local.CreateCalls(), 1)
		assert.Len(t, local.CreateCalls()[0].Nr.Data, localrecords.MaxFileBytes+1)
	})

	t.Run("unsupported type is 422", func(t *testing.T) {
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return localrecords.Report{}, localrecords.ErrUnsupportedType
		}}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, nil, "referto.pdf", "application/pdf", []byte("not a pdf"))

		resp := api.Post("/api/reports/local", testHost, testOrigin, contentType, body)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("cross origin is forbidden", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})
		body, contentType := multipartBody(t, nil, "referto.pdf", "application/pdf", []byte("%PDF-1.7"))

		resp := api.Post("/api/reports/local", testHost, "Origin: http://evil.test", contentType, body)

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, local.CreateCalls())
	})
}

func TestRegisterImportDriveReport(t *testing.T) {
	t.Run("downloads the file and creates the report", func(t *testing.T) {
		g := &MockGoogle{DownloadFileFunc: func(context.Context, string) (google.DriveFile, error) {
			return google.DriveFile{Name: "scan.pdf", MimeType: "application/pdf", Data: []byte("%PDF-1.7")}, nil
		}}
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return createdReport(), nil
		}}
		api := newLocalAPI(t, local, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234", "title": "Visita", "date": "2026-10-01", "nre": strings.ToLower(testNRE)})

		require.Equal(t, http.StatusCreated, resp.Code)
		assert.Contains(t, resp.Body.String(), `"nres":["`+testNRE+`"]`)
		require.Len(t, g.DownloadFileCalls(), 1)
		assert.Equal(t, "drive-file-1234", g.DownloadFileCalls()[0].FileID)
		require.Len(t, local.CreateCalls(), 1)
		got := local.CreateCalls()[0].Nr
		assert.Equal(t, localrecords.OriginDrive, got.Origin)
		assert.Equal(t, "scan.pdf", got.Filename)
		assert.Equal(t, []byte("%PDF-1.7"), got.Data)
		assert.Equal(t, testNRE, got.NRE)
	})

	t.Run("bad date is 422", func(t *testing.T) {
		g := &MockGoogle{}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234", "date": "2026-13-45"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, g.DownloadFileCalls())
	})

	t.Run("unsupported drive file type is 422", func(t *testing.T) {
		g := &MockGoogle{DownloadFileFunc: func(context.Context, string) (google.DriveFile, error) {
			return google.DriveFile{}, google.ErrDriveFileType
		}}
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.CreateCalls())
	})

	t.Run("drive file too large is 413", func(t *testing.T) {
		g := &MockGoogle{DownloadFileFunc: func(context.Context, string) (google.DriveFile, error) {
			return google.DriveFile{}, google.ErrDriveFileTooLarge
		}}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234"})

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
	})

	t.Run("google not connected is 401", func(t *testing.T) {
		g := &MockGoogle{DownloadFileFunc: func(context.Context, string) (google.DriveFile, error) {
			return google.DriveFile{}, google.ErrNotConnected
		}}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234"})

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("create failure is 500", func(t *testing.T) {
		g := &MockGoogle{DownloadFileFunc: func(context.Context, string) (google.DriveFile, error) { return google.DriveFile{Name: "a.pdf"}, nil }}
		local := &MockLocalRecords{CreateFunc: func(context.Context, localrecords.NewReport) (localrecords.Report, error) {
			return localrecords.Report{}, errors.New("boom")
		}}
		api := newLocalAPI(t, local, g)

		resp := api.Post("/api/reports/local/drive", testHost, testOrigin, map[string]string{"fileId": "drive-file-1234"})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestRegisterUpdateLocalReport(t *testing.T) {
	t.Run("updates the metadata and returns 204", func(t *testing.T) {
		local := &MockLocalRecords{UpdateFunc: func(context.Context, string, localrecords.Meta) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/reports/local", testHost, testOrigin, map[string]string{"id": testID, "title": "Nuovo", "date": "2026-10-01", "provider": "ASL"})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.UpdateCalls(), 1)
		assert.Equal(t, testID, local.UpdateCalls()[0].ID)
		assert.Equal(t, localrecords.Meta{Title: "Nuovo", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Provider: "ASL"}, local.UpdateCalls()[0].Meta)
	})

	t.Run("not found is 404", func(t *testing.T) {
		local := &MockLocalRecords{UpdateFunc: func(context.Context, string, localrecords.Meta) error { return localrecords.ErrNotFound }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/reports/local", testHost, testOrigin, map[string]string{"id": testID, "title": "Nuovo"})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("bad date is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/reports/local", testHost, testOrigin, map[string]string{"id": testID, "title": "Nuovo", "date": "2026-02-31"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.UpdateCalls())
	})
}

func TestRegisterDeleteLocalReport(t *testing.T) {
	t.Run("deletes and returns 204", func(t *testing.T) {
		local := &MockLocalRecords{DeleteFunc: func(context.Context, string) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/reports/local", testHost, testOrigin, map[string]string{"id": testID})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.DeleteCalls(), 1)
		assert.Equal(t, testID, local.DeleteCalls()[0].ID)
	})

	t.Run("not found is 404", func(t *testing.T) {
		local := &MockLocalRecords{DeleteFunc: func(context.Context, string) error { return localrecords.ErrNotFound }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/reports/local", testHost, testOrigin, map[string]string{"id": testID})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("cross origin is forbidden", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/reports/local", testHost, "Origin: http://evil.test", map[string]string{"id": testID})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, local.DeleteCalls())
	})
}

func TestRegisterLinkReport(t *testing.T) {
	t.Run("links with an upper case nre", func(t *testing.T) {
		local := &MockLocalRecords{LinkFunc: func(context.Context, string, referti.Ref) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/links", testHost, testOrigin, map[string]any{"nre": strings.ToLower(testNRE), "report": map[string]string{"kind": "lab", "abstractId": "A1", "detailId": "D1", "repCode": "R1"}})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.LinkCalls(), 1)
		assert.Equal(t, testNRE, local.LinkCalls()[0].Nre)
		assert.Equal(t, referti.Ref{Kind: referti.KindLab, AbstractID: "A1", DetailID: "D1", RepCode: "R1"}, local.LinkCalls()[0].Ref)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/links", testHost, testOrigin, map[string]any{"nre": testNRE, "report": map[string]string{"kind": "xyz"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.LinkCalls())
	})

	t.Run("not linkable kind is 422", func(t *testing.T) {
		local := &MockLocalRecords{LinkFunc: func(context.Context, string, referti.Ref) error { return localrecords.ErrInvalidReportKind }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/links", testHost, testOrigin, map[string]any{"nre": testNRE, "report": map[string]string{"kind": "fse"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterUnlinkReport(t *testing.T) {
	t.Run("unlinks with an upper case nre", func(t *testing.T) {
		local := &MockLocalRecords{UnlinkFunc: func(context.Context, string, referti.Ref) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/links", testHost, testOrigin, map[string]any{"nre": strings.ToLower(testNRE), "report": map[string]string{"kind": "local", "abstractId": testID}})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.UnlinkCalls(), 1)
		assert.Equal(t, testNRE, local.UnlinkCalls()[0].Nre)
		assert.Equal(t, localrecords.LocalRef(testID), local.UnlinkCalls()[0].Ref)
	})

	t.Run("unknown kind is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/links", testHost, testOrigin, map[string]any{"nre": testNRE, "report": map[string]string{"kind": "xyz"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.UnlinkCalls())
	})
}

func TestRegisterSetVisitStatus(t *testing.T) {
	t.Run("sets the status", func(t *testing.T) {
		local := &MockLocalRecords{SetStatusFunc: func(context.Context, string, int) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/visits/status", testHost, testOrigin, map[string]any{"nre": strings.ToLower(testNRE), "status": 8})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.SetStatusCalls(), 1)
		assert.Equal(t, testNRE, local.SetStatusCalls()[0].Nre)
		assert.Equal(t, 8, local.SetStatusCalls()[0].Status)
	})

	t.Run("status outside the enum is 422", func(t *testing.T) {
		local := &MockLocalRecords{}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/visits/status", testHost, testOrigin, map[string]any{"nre": testNRE, "status": 99})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, local.SetStatusCalls())
	})

	t.Run("invalid status from the service is 422", func(t *testing.T) {
		local := &MockLocalRecords{SetStatusFunc: func(context.Context, string, int) error { return localrecords.ErrInvalidStatus }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Put("/api/visits/status", testHost, testOrigin, map[string]any{"nre": testNRE, "status": 8})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterResetVisitStatus(t *testing.T) {
	t.Run("resets the status", func(t *testing.T) {
		local := &MockLocalRecords{ResetStatusFunc: func(context.Context, string) error { return nil }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/visits/status", testHost, testOrigin, map[string]string{"nre": strings.ToLower(testNRE)})

		require.Equal(t, http.StatusNoContent, resp.Code)
		require.Len(t, local.ResetStatusCalls(), 1)
		assert.Equal(t, testNRE, local.ResetStatusCalls()[0].Nre)
	})

	t.Run("store failure is 500", func(t *testing.T) {
		local := &MockLocalRecords{ResetStatusFunc: func(context.Context, string) error { return errors.New("boom") }}
		api := newLocalAPI(t, local, &MockGoogle{})

		resp := api.Delete("/api/visits/status", testHost, testOrigin, map[string]string{"nre": testNRE})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestRegisterGooglePicker(t *testing.T) {
	t.Run("returns the picker config", func(t *testing.T) {
		g := &MockGoogle{PickerConfigFunc: func(context.Context) (google.PickerConfig, error) {
			return google.PickerConfig{AccessToken: "tok", APIKey: "key", AppID: "123456789"}, nil
		}}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/google/picker", testHost, testOrigin)

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"accessToken":"tok","apiKey":"key","appId":"123456789"}`, resp.Body.String())
	})

	t.Run("not configured is 409", func(t *testing.T) {
		g := &MockGoogle{PickerConfigFunc: func(context.Context) (google.PickerConfig, error) {
			return google.PickerConfig{}, google.ErrPickerNotConfigured
		}}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/google/picker", testHost, testOrigin)

		require.Equal(t, http.StatusConflict, resp.Code)
		assert.Contains(t, resp.Body.String(), "google_picker_not_configured")
	})

	t.Run("not connected is 401", func(t *testing.T) {
		g := &MockGoogle{PickerConfigFunc: func(context.Context) (google.PickerConfig, error) {
			return google.PickerConfig{}, google.ErrNotConnected
		}}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/google/picker", testHost, testOrigin)

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("cross origin is forbidden", func(t *testing.T) {
		g := &MockGoogle{}
		api := newLocalAPI(t, &MockLocalRecords{}, g)

		resp := api.Post("/api/google/picker", testHost, "Origin: http://evil.test")

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.Empty(t, g.PickerConfigCalls())
	})
}
