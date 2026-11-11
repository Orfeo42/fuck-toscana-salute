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
	Configured bool `json:"configured"`
	Connected  bool `json:"connected"`
}

type googleConnectOutput struct {
	Body GoogleConnectBody
}

type GoogleConnectBody struct {
	AuthURL string `json:"authUrl"`
}

type redirectInput struct {
	redirectURL string
}

func (i *redirectInput) Resolve(ctx huma.Context) []error {
	scheme := "http"
	if ctx.Header("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	i.redirectURL = scheme + "://" + ctx.Host() + google.CallbackPath
	return nil
}

type googleConnectInput struct {
	redirectInput
}

type googleSettingsInput struct {
	redirectInput
}

type googleSettingsOutput struct {
	Body GoogleSettingsBody
}

type GoogleSettingsBody struct {
	ClientID      string `json:"clientId"`
	CalendarID    string `json:"calendarId"`
	DriveFolderID string `json:"driveFolderId"`
	HasSecret     bool   `json:"hasSecret"`
	RedirectURI   string `json:"redirectUri"`
}

type saveGoogleSettingsInput struct {
	Body GoogleSettingsRequest
}

type GoogleSettingsRequest struct {
	ClientID      string `json:"clientId" required:"true" maxLength:"200"`
	ClientSecret  string `json:"clientSecret,omitempty" maxLength:"200"`
	CalendarID    string `json:"calendarId,omitempty" maxLength:"200"`
	DriveFolderID string `json:"driveFolderId,omitempty" maxLength:"100"`
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
		return &googleStatusOutput{Body: GoogleStatusBody{Configured: status.Configured, Connected: status.Connected}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "start-google-connect",
		Method:        http.MethodPost,
		Path:          "/api/google/connect",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *googleConnectInput) (*googleConnectOutput, error) {
		authURL, err := g.AuthURL(ctx, in.redirectURL)
		if err != nil {
			return nil, toHTTPError(ctx, "start-google-connect", err)
		}
		return &googleConnectOutput{Body: GoogleConnectBody{AuthURL: authURL}}, nil
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
		return &googleSettingsOutput{Body: GoogleSettingsBody{
			ClientID:      settings.ClientID,
			CalendarID:    settings.CalendarID,
			DriveFolderID: settings.DriveFolderID,
			HasSecret:     settings.HasSecret,
			RedirectURI:   in.redirectURL,
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
		result, err := g.AddEvent(ctx, google.Event{
			Key:      req.AppointmentID,
			Title:    req.Title,
			Start:    req.Start,
			Duration: time.Duration(req.DurationMinutes) * time.Minute,
			Location: req.Location,
			Details:  req.Details,
		}, pdf)
		if err != nil {
			return nil, toHTTPError(ctx, "create-calendar-event", err)
		}
		return &calendarEventOutput{Body: CalendarEventBody{HTMLLink: result.HTMLLink, Created: result.Created}}, nil
	})
}
