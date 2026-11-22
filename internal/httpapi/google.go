package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fuck-toscana-salute/internal/google"
)

type googleStatusOutput struct {
	Body GoogleStatusBody
}

type GoogleStatusBody struct {
	Configured bool   `json:"configured"`
	Connected  bool   `json:"connected"`
	ClientID   string `json:"clientId"`
	Drive      bool   `json:"drive"`
}

type originInput struct {
	origin string
}

func (i *originInput) Resolve(ctx huma.Context) []error {
	scheme := "http"
	if ctx.Header("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	i.origin = scheme + "://" + ctx.Host()
	return nil
}

type googleSettingsInput struct {
	originInput
}

type exchangeGoogleCodeInput struct {
	originInput
	RequestedWith string `header:"X-Requested-With" required:"true" enum:"XmlHttpRequest"`
	Body          GoogleCodeRequest
}

type GoogleCodeRequest struct {
	Code string `json:"code" required:"true" minLength:"1" maxLength:"2048"`
}

type googleSettingsOutput struct {
	Body GoogleSettingsBody
}

type GoogleSettingsBody struct {
	ClientID         string   `json:"clientId"`
	CalendarID       string   `json:"calendarId"`
	DriveFolderID    string   `json:"driveFolderId"`
	APIKey           string   `json:"apiKey"`
	Guests           []string `json:"guests" nullable:"false"`
	HasSecret        bool     `json:"hasSecret"`
	JavaScriptOrigin string   `json:"javascriptOrigin"`
}

type saveGoogleSettingsInput struct {
	Body GoogleSettingsRequest
}

type GoogleSettingsRequest struct {
	ClientID      string   `json:"clientId" required:"true" maxLength:"200"`
	ClientSecret  string   `json:"clientSecret,omitempty" maxLength:"200"`
	CalendarID    string   `json:"calendarId,omitempty" maxLength:"200"`
	DriveFolderID string   `json:"driveFolderId,omitempty" maxLength:"100"`
	APIKey        string   `json:"apiKey,omitempty" maxLength:"100"`
	Guests        []string `json:"guests,omitempty" maxItems:"10" nullable:"false"`
}

type createCalendarEventInput struct {
	Body CalendarEventRequest
}

type CalendarEventRequest struct {
	AppointmentID    string    `json:"appointmentId" required:"true" minLength:"1" maxLength:"64" pattern:"^[A-Za-z0-9_-]+$"`
	Title            string    `json:"title" required:"true" minLength:"1" maxLength:"200"`
	Start            time.Time `json:"start" required:"true" format:"date-time"`
	DurationMinutes  int       `json:"durationMinutes,omitempty" default:"60" minimum:"5" maximum:"480"`
	Location         string    `json:"location,omitempty" maxLength:"300"`
	Details          string    `json:"details,omitempty" maxLength:"1000"`
	BookingID        string    `json:"bookingId,omitempty" maxLength:"32" pattern:"^[A-Za-z0-9]+$"`
	AttachPromemoria *bool     `json:"attachPromemoria,omitempty" default:"true"`
	Regenerate       bool      `json:"regenerate,omitempty"`
	Guests           *[]string `json:"guests,omitempty" maxItems:"10" nullable:"false"`
}

type lookupCalendarEventInput struct {
	Body CalendarEventLookupRequest
}

type CalendarEventLookupRequest struct {
	AppointmentID string `json:"appointmentId" required:"true" minLength:"1" maxLength:"64" pattern:"^[A-Za-z0-9_-]+$"`
}

type calendarEventLookupOutput struct {
	Body CalendarEventLookupBody
}

type CalendarEventLookupBody struct {
	Exists   bool   `json:"exists"`
	HTMLLink string `json:"htmlLink"`
}

type calendarEventOutput struct {
	Body CalendarEventBody
}

type CalendarEventBody struct {
	HTMLLink string `json:"htmlLink"`
	Created  bool   `json:"created"`
}

func registerGoogle(api huma.API, g Google, docs Documents) {
	huma.Register(api, huma.Operation{
		OperationID:   "get-google-status",
		Method:        http.MethodGet,
		Path:          "/api/google",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*googleStatusOutput, error) {
		status, err := g.Status(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "get-google-status", err)
		}
		return &googleStatusOutput{Body: GoogleStatusBody{Configured: status.Configured, Connected: status.Connected, ClientID: status.ClientID, Drive: status.Drive}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "exchange-google-code",
		Method:        http.MethodPost,
		Path:          "/api/google/code",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *exchangeGoogleCodeInput) (*emptyOutput, error) {
		if err := g.ExchangeCode(ctx, in.Body.Code, in.origin); err != nil {
			return nil, toHTTPError(ctx, "exchange-google-code", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "get-google-settings",
		Method:        http.MethodGet,
		Path:          "/api/google/settings",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *googleSettingsInput) (*googleSettingsOutput, error) {
		settings, err := g.Settings(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "get-google-settings", err)
		}
		guests := settings.Guests
		if guests == nil {
			guests = []string{}
		}
		return &googleSettingsOutput{Body: GoogleSettingsBody{
			ClientID:         settings.ClientID,
			CalendarID:       settings.CalendarID,
			DriveFolderID:    settings.DriveFolderID,
			APIKey:           settings.APIKey,
			Guests:           guests,
			HasSecret:        settings.HasSecret,
			JavaScriptOrigin: in.origin,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "save-google-settings",
		Method:        http.MethodPut,
		Path:          "/api/google/settings",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *saveGoogleSettingsInput) (*emptyOutput, error) {
		err := g.SaveSettings(ctx, google.Settings{
			ClientID:      in.Body.ClientID,
			ClientSecret:  in.Body.ClientSecret,
			CalendarID:    in.Body.CalendarID,
			DriveFolderID: in.Body.DriveFolderID,
			APIKey:        in.Body.APIKey,
			Guests:        in.Body.Guests,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "save-google-settings", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "google-disconnect",
		Method:        http.MethodDelete,
		Path:          "/api/google",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		if err := g.Disconnect(ctx); err != nil {
			return nil, toHTTPError(ctx, "google-disconnect", err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-calendar-event",
		Method:        http.MethodPost,
		Path:          "/api/calendar/events",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *createCalendarEventInput) (*calendarEventOutput, error) {
		req := in.Body
		var pdf []byte
		if req.AttachPromemoria != nil && *req.AttachPromemoria && req.BookingID != "" {
			var err error
			pdf, err = docs.Promemoria(ctx, req.BookingID)
			if err != nil {
				return nil, toHTTPError(ctx, "create-calendar-event", err)
			}
		}
		var guests []string
		if req.Guests == nil {
			settings, err := g.Settings(ctx)
			if err != nil {
				return nil, toHTTPError(ctx, "create-calendar-event", err)
			}
			guests = settings.Guests
		} else {
			var err error
			guests, err = google.ValidateGuests(*req.Guests)
			if err != nil {
				return nil, toHTTPError(ctx, "create-calendar-event", err)
			}
		}
		result, err := g.AddEvent(ctx, google.Event{
			Key:        req.AppointmentID,
			Title:      req.Title,
			Start:      req.Start,
			Duration:   time.Duration(req.DurationMinutes) * time.Minute,
			Location:   req.Location,
			Details:    req.Details,
			Guests:     guests,
			Regenerate: req.Regenerate,
		}, pdf)
		if err != nil {
			return nil, toHTTPError(ctx, "create-calendar-event", err)
		}
		return &calendarEventOutput{Body: CalendarEventBody{HTMLLink: result.HTMLLink, Created: result.Created}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "lookup-calendar-event",
		Method:        http.MethodPost,
		Path:          "/api/calendar/events/lookup",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *lookupCalendarEventInput) (*calendarEventLookupOutput, error) {
		found, err := g.LookupEvent(ctx, in.Body.AppointmentID)
		if err != nil {
			return nil, toHTTPError(ctx, "lookup-calendar-event", err)
		}
		return &calendarEventLookupOutput{Body: CalendarEventLookupBody{Exists: found.Exists, HTMLLink: found.HTMLLink}}, nil
	})
}
