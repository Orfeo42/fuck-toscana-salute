package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
	"fuck-toscana-salute/internal/visitsview"
)

const validNRE = "0900A0000000001"

func newVisitsAPI(t *testing.T, view VisitsView) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerVisits(api, view)
	return api
}

func emptyVisits() *MockVisitsView {
	return &MockVisitsView{QueryFunc: func(context.Context, visitsview.Query) (visitsview.Page, error) {
		return visitsview.Page{}, nil
	}}
}

func TestRegisterQueryVisits(t *testing.T) {
	t.Run("applies defaults for an empty body", func(t *testing.T) {
		view := emptyVisits()
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		require.Len(t, view.QueryCalls(), 1)
		q := view.QueryCalls()[0].Q
		assert.Equal(t, visitsview.SortPrescribed, q.Sort)
		assert.Equal(t, listview.DirDesc, q.Dir)
		assert.Equal(t, 1, q.Page)
		assert.Equal(t, 50, q.PageSize)
		assert.Empty(t, q.Search)
		assert.Empty(t, q.Focus)
	})

	t.Run("absent excludeStatuses reaches the service as nil", func(t *testing.T) {
		view := emptyVisits()
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Nil(t, view.QueryCalls()[0].Q.ExcludeStatuses)
	})

	t.Run("empty excludeStatuses reaches the service as empty not nil", func(t *testing.T) {
		view := emptyVisits()
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{"excludeStatuses": []string{}})

		require.Equal(t, http.StatusOK, resp.Code)
		q := view.QueryCalls()[0].Q
		assert.NotNil(t, q.ExcludeStatuses)
		assert.Empty(t, q.ExcludeStatuses)
	})

	t.Run("passes the filters through", func(t *testing.T) {
		view := emptyVisits()
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{
			"search":          "rx",
			"excludeStatuses": []string{"done", "other"},
			"sort":            "appointment",
			"dir":             "asc",
			"page":            3,
			"pageSize":        25,
			"focus":           validNRE,
		})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, visitsview.Query{
			Search:          "rx",
			ExcludeStatuses: []string{"done", "other"},
			Sort:            visitsview.SortAppointment,
			Dir:             listview.DirAsc,
			Page:            3,
			PageSize:        25,
			Focus:           validNRE,
		}, view.QueryCalls()[0].Q)
	})

	t.Run("unknown sort is rejected", func(t *testing.T) {
		view := emptyVisits()
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{"sort": "color"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, view.QueryCalls())
	})

	t.Run("unknown dir is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"dir": "up"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("unknown excluded status is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"excludeStatuses": []string{"bogus"}})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("page size 25 is accepted", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"pageSize": 25})

		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("page size 100 is accepted", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"pageSize": 100})

		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("page size 30 is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"pageSize": 30})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("page 0 is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"page": 0})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("search of 100 characters is accepted", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"search": strings.Repeat("a", 100)})

		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("search over 100 characters is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"search": strings.Repeat("a", 101)})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("malformed focus is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{"focus": "../x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("maps the page and serializes empty arrays as lists", func(t *testing.T) {
		at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
		view := &MockVisitsView{QueryFunc: func(context.Context, visitsview.Query) (visitsview.Page, error) {
			return visitsview.Page{
				Rows: []visitsview.Row{{
					NRE:               validNRE,
					Title:             "Visita",
					AppointmentLookup: visitsview.LookupDone,
					Status:            visitsview.Status{Key: "booked", Label: "Prenotata", Tone: listview.ToneBooked, Kind: visitsview.KindBooked},
					Appointment:       &visitsview.Appointment{ID: "a1", At: at, Unit: visitsview.Unit{DisplayName: "Ospedale"}},
					Reports:           []visitsview.LinkedReport{{Ref: referti.Ref{Kind: referti.KindLab, AbstractID: "1"}, Label: "Laboratorio", Origin: reports.OriginRemote, Available: true}},
				}},
				Total:    1,
				Page:     1,
				PageSize: 50,
				Focus:    &listview.Focus{NRE: validNRE, Found: true},
				Updated:  listview.Updated{At: at, Label: "Aggiornato alle 10:00"},
				Issues:   visitsview.Issues{Lists: refresh.IssueNone, Appointments: refresh.IssueGestioneRequired},
			}, nil
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		body := resp.Body.String()
		assert.Contains(t, body, `"statuses":[]`)
		assert.Contains(t, body, `"excludedStatuses":[]`)
		assert.Contains(t, body, `"details":[]`)
		assert.Contains(t, body, `"appointments":"gestione_required"`)
		assert.Contains(t, body, `"focus":{"nre":"`+validNRE+`","found":true}`)
		assert.Contains(t, body, `"label":"Aggiornato alle 10:00"`)
		assert.Contains(t, body, `"mapsSearchUrl"`)
		assert.Contains(t, body, `"origin":"remote"`)
	})

	t.Run("an empty page has empty rows not null", func(t *testing.T) {
		api := newVisitsAPI(t, emptyVisits())

		resp := api.Post("/api/visits/query", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"rows":[]`)
		assert.Contains(t, resp.Body.String(), `"at":null`)
	})

	t.Run("failure is 500", func(t *testing.T) {
		view := &MockVisitsView{QueryFunc: func(context.Context, visitsview.Query) (visitsview.Page, error) {
			return visitsview.Page{}, assert.AnError
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/query", map[string]any{})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestRegisterGetVisitRow(t *testing.T) {
	t.Run("returns the row with manual status options", func(t *testing.T) {
		current := 8
		view := &MockVisitsView{RowFunc: func(context.Context, string) (visitsview.RowResult, error) {
			return visitsview.RowResult{
				Row:          visitsview.Row{NRE: validNRE, Title: "Visita", Status: visitsview.Status{Key: "done", Tone: listview.ToneDone, Kind: visitsview.KindOfficial}, AppointmentLookup: visitsview.LookupNone},
				ManualStatus: visitsview.ManualStatus{Current: &current, Options: []visitsview.ManualOption{{Code: 3, Label: "Da prenotare"}}},
				Refreshing:   true,
			}, nil
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/row", map[string]any{"nre": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, validNRE, view.RowCalls()[0].Nre)
		assert.Contains(t, resp.Body.String(), `"manualStatus":{"current":8,"options":[{"code":3,"label":"Da prenotare"}]}`)
		assert.Contains(t, resp.Body.String(), `"refreshing":true`)
	})

	t.Run("maps every row field", func(t *testing.T) {
		when := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
		view := &MockVisitsView{RowFunc: func(context.Context, string) (visitsview.RowResult, error) {
			return visitsview.RowResult{Row: visitsview.Row{
				NRE:               validNRE,
				Title:             "Visita",
				PrescribedOn:      when,
				DocumentRef:       "ref-1",
				AppointmentLookup: visitsview.LookupDone,
				Status: visitsview.Status{
					Key: "unverified", Label: "Da verificare", Tone: listview.TonePending, Kind: visitsview.KindOfficial,
					Code: 30, Manual: true, Note: "nota", VerifiedAt: when, CanBook: true, ExpiresOn: when, ExpiryEstimated: true, Details: []string{"d1"},
				},
				Appointment: &visitsview.Appointment{
					ID: "a1", BookingID: "b1", Status: "PRENOTATO", At: when, RelativeDay: "domani", Upcoming: true,
					Unit:     visitsview.Unit{DisplayName: "Ospedale", FullAddress: "Via Roma 1", MapsDirectionsURL: "https://maps/dir"},
					Calendar: visitsview.CalendarFields{Title: "Visita", Location: "Ospedale", Details: "Prenotazione"},
				},
				Reports:   []visitsview.LinkedReport{{Ref: referti.Ref{Kind: referti.KindLab, AbstractID: "1"}, Label: "Lab", Date: when, Provider: "P", Origin: reports.OriginLocal, Available: true}},
				CheckedAt: visitsview.Checked{Appointments: when},
				Upload:    visitsview.UploadDefaults{Title: "Visita", Date: "2026-11-12", Provider: "Ospedale"},
			}}, nil
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/row", map[string]any{"nre": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		body := resp.Body.String()
		for _, want := range []string{
			`"documentRef":"ref-1"`,
			`"appointmentLookup":"done"`,
			`"code":30`,
			`"manual":true`,
			`"canBook":true`,
			`"expiryEstimated":true`,
			`"bookingId":"b1"`,
			`"relativeDay":"domani"`,
			`"upcoming":true`,
			`"mapsDirectionsUrl":"https://maps/dir"`,
			`"calendar":{"title":"Visita","location":"Ospedale","details":"Prenotazione"}`,
			`"origin":"local"`,
			`"available":true`,
			`"verdict":null`,
			`"upload":{"title":"Visita","date":"2026-11-12","provider":"Ospedale"}`,
		} {
			assert.Contains(t, body, want)
		}
	})

	t.Run("unknown visit is 404", func(t *testing.T) {
		view := &MockVisitsView{RowFunc: func(context.Context, string) (visitsview.RowResult, error) {
			return visitsview.RowResult{}, visitsview.ErrNotFound
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/row", map[string]any{"nre": validNRE})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("malformed nre is rejected", func(t *testing.T) {
		view := &MockVisitsView{}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/row", map[string]any{"nre": "../x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, view.RowCalls())
	})

	t.Run("missing nre is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, &MockVisitsView{})

		resp := api.Post("/api/visits/row", map[string]any{})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestRegisterVisitReportOptions(t *testing.T) {
	t.Run("returns the options", func(t *testing.T) {
		at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
		view := &MockVisitsView{ReportOptionsFunc: func(context.Context, string) ([]visitsview.ReportOption, error) {
			return []visitsview.ReportOption{{Ref: referti.Ref{Kind: referti.KindLocal, AbstractID: "7"}, Label: "Caricato", Date: at, Provider: "ASL", Origin: reports.OriginLocal}}, nil
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/report-options", map[string]any{"nre": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, validNRE, view.ReportOptionsCalls()[0].Nre)
		assert.Contains(t, resp.Body.String(), `"origin":"local"`)
		assert.Contains(t, resp.Body.String(), `"abstractId":"7"`)
	})

	t.Run("no options is an empty list", func(t *testing.T) {
		view := &MockVisitsView{ReportOptionsFunc: func(context.Context, string) ([]visitsview.ReportOption, error) { return nil, nil }}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/report-options", map[string]any{"nre": validNRE})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.JSONEq(t, `{"options":[]}`, resp.Body.String())
	})

	t.Run("unknown visit is 404", func(t *testing.T) {
		view := &MockVisitsView{ReportOptionsFunc: func(context.Context, string) ([]visitsview.ReportOption, error) {
			return nil, visitsview.ErrNotFound
		}}
		api := newVisitsAPI(t, view)

		resp := api.Post("/api/visits/report-options", map[string]any{"nre": validNRE})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("malformed nre is rejected", func(t *testing.T) {
		api := newVisitsAPI(t, &MockVisitsView{})

		resp := api.Post("/api/visits/report-options", map[string]any{"nre": "x"})

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}
