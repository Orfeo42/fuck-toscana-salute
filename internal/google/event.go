package google

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	statusCancelled = "cancelled"
	statusConfirmed = "confirmed"

	calendarID    = "primary"
	timeZone      = "Europe/Rome"
	pdfMimeType   = "application/pdf"
	eventIDPrefix = "fts"
	eventIDMax    = 40
	invalidGrant  = "invalid_grant"
)

var reminderMinutes = []int64{1440, 60}

type Event struct {
	Key      string
	Title    string
	Start    time.Time
	Duration time.Duration
	Location string
	Details  string
}

type EventResult struct {
	HTMLLink string
	Created  bool
}

type savingSource struct {
	ctx   context.Context
	src   oauth2.TokenSource
	store TokenStore

	mu   sync.Mutex
	last string
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	token, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if token.AccessToken == s.last {
		return token, nil
	}
	if err := s.store.Save(s.ctx, token); err != nil {
		return nil, fmt.Errorf("save refreshed token: %w", err)
	}
	s.last = token.AccessToken
	return token, nil
}

func eventID(key string) string {
	sum := sha256.Sum256([]byte(key))
	enc := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return (eventIDPrefix + strings.ToLower(enc))[:eventIDMax]
}

func (c *Client) AddEvent(ctx context.Context, ev Event, pdf []byte) (EventResult, error) {
	if c.cfg == nil {
		return EventResult{}, ErrNotConfigured
	}
	token, err := c.store.Load(ctx)
	if err != nil {
		return EventResult{}, fmt.Errorf("google: load token: %w", err)
	}
	if token == nil {
		return EventResult{}, ErrNotConnected
	}
	rome, err := time.LoadLocation(timeZone)
	if err != nil {
		return EventResult{}, fmt.Errorf("google: load timezone: %w", err)
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	ts := &savingSource{ctx: ctx, src: c.cfg.TokenSource(ctx, token), store: c.store, last: token.AccessToken}
	calOpts := []option.ClientOption{option.WithTokenSource(ts)}
	if c.calendarEndpoint != "" {
		calOpts = append(calOpts, option.WithEndpoint(c.calendarEndpoint))
	}
	cal, err := calendar.NewService(ctx, calOpts...)
	if err != nil {
		return EventResult{}, fmt.Errorf("google: calendar service: %w", err)
	}

	id := eventID(ev.Key)
	existing, err := c.getEvent(ctx, cal, id)
	if err != nil {
		return EventResult{}, c.classify(ctx, err)
	}
	if existing != nil && existing.Status != statusCancelled {
		return EventResult{HTMLLink: existing.HtmlLink}, nil
	}

	var attachments []*calendar.EventAttachment
	var svc *drive.Service
	var fileID string
	if len(pdf) > 0 {
		svc, err = c.driveService(ctx, ts)
		if err != nil {
			return EventResult{}, err
		}
		name := "promemoria-" + ev.Start.In(rome).Format(time.DateOnly) + ".pdf"
		file, err := c.upload(ctx, svc, name, pdf)
		if err != nil {
			return EventResult{}, c.classify(ctx, err)
		}
		fileID = file.Id
		attachments = []*calendar.EventAttachment{{FileUrl: file.WebViewLink, MimeType: pdfMimeType, Title: name}}
	}

	reminders := make([]*calendar.EventReminder, 0, len(reminderMinutes))
	for _, m := range reminderMinutes {
		reminders = append(reminders, &calendar.EventReminder{Method: "popup", Minutes: m})
	}
	body := &calendar.Event{
		Id:          id,
		Status:      statusConfirmed,
		Summary:     ev.Title,
		Location:    ev.Location,
		Description: ev.Details,
		Start:       &calendar.EventDateTime{DateTime: ev.Start.In(rome).Format(time.RFC3339), TimeZone: timeZone},
		End:         &calendar.EventDateTime{DateTime: ev.Start.Add(ev.Duration).In(rome).Format(time.RFC3339), TimeZone: timeZone},
		Reminders:   &calendar.EventReminders{UseDefault: false, Overrides: reminders, ForceSendFields: []string{"UseDefault"}},
		Attachments: attachments,
	}

	result, err := c.write(ctx, cal, body, existing != nil)
	if fileID != "" && (err != nil || !result.Created) {
		c.deleteFile(ctx, svc, fileID)
	}
	if err != nil {
		return EventResult{}, c.classify(ctx, err)
	}
	return result, nil
}

func (c *Client) write(ctx context.Context, cal *calendar.Service, body *calendar.Event, cancelled bool) (EventResult, error) {
	if !cancelled {
		created, err := cal.Events.Insert(calendarID, body).SupportsAttachments(true).Context(ctx).Do()
		if err == nil {
			return EventResult{HTMLLink: created.HtmlLink, Created: true}, nil
		}
		if !isStatus(err, http.StatusConflict) {
			return EventResult{}, err
		}
		raced, getErr := c.getEvent(ctx, cal, body.Id)
		if getErr != nil {
			return EventResult{}, getErr
		}
		if raced == nil {
			return EventResult{}, err
		}
		if raced.Status != statusCancelled {
			return EventResult{HTMLLink: raced.HtmlLink}, nil
		}
	}

	updated, err := cal.Events.Update(calendarID, body.Id, body).SupportsAttachments(true).Context(ctx).Do()
	if err != nil {
		return EventResult{}, err
	}
	return EventResult{HTMLLink: updated.HtmlLink, Created: true}, nil
}

func (c *Client) getEvent(ctx context.Context, cal *calendar.Service, id string) (*calendar.Event, error) {
	got, err := cal.Events.Get(calendarID, id).Context(ctx).Do()
	if isStatus(err, http.StatusNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return got, nil
}

func (c *Client) driveService(ctx context.Context, ts oauth2.TokenSource) (*drive.Service, error) {
	opts := []option.ClientOption{option.WithTokenSource(ts)}
	if c.driveEndpoint != "" {
		opts = append(opts, option.WithEndpoint(c.driveEndpoint))
	}
	svc, err := drive.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("google: drive service: %w", err)
	}
	return svc, nil
}

func (c *Client) upload(ctx context.Context, svc *drive.Service, name string, pdf []byte) (*drive.File, error) {
	file, err := svc.Files.Create(&drive.File{Name: name, MimeType: pdfMimeType}).
		Media(bytes.NewReader(pdf), googleapi.ContentType(pdfMimeType)).
		Fields("id,webViewLink").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (c *Client) deleteFile(ctx context.Context, svc *drive.Service, fileID string) {
	err := svc.Files.Delete(fileID).Context(context.WithoutCancel(ctx)).Do()
	if err == nil {
		return
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		slog.WarnContext(ctx, "google orphan file cleanup failed", slog.Int("status", apiErr.Code))
		return
	}
	slog.WarnContext(ctx, "google orphan file cleanup failed")
}

func (c *Client) classify(ctx context.Context, err error) error {
	var retrieveErr *oauth2.RetrieveError
	if isStatus(err, http.StatusUnauthorized) || (errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == invalidGrant) {
		if delErr := c.store.Delete(ctx); delErr != nil {
			return errors.Join(ErrNotConnected, fmt.Errorf("google: delete token: %w", delErr))
		}
		return ErrNotConnected
	}
	return fmt.Errorf("google: add event: %w", err)
}

func isStatus(err error, status int) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == status
}
