package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"

	"fuck-toscana-salute/internal/vault"
)

const (
	ErrClientIDRequired     googleError = "google: client id is required"
	ErrClientIDInvalid      googleError = "google: client id must end with .apps.googleusercontent.com"
	ErrClientSecretRequired googleError = "google: client secret is required"
	ErrDriveFolderInvalid   googleError = "google: drive folder id is invalid"
)

const (
	settingsService = "fuck-toscana-salute"
	settingsUser    = "google-settings"
	clientIDSuffix  = ".apps.googleusercontent.com"
	defaultCalendar = "primary"
)

var driveFolderPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,}$`)

type Settings struct {
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret"`
	CalendarID    string `json:"calendarId"`
	DriveFolderID string `json:"driveFolderId"`
	HasSecret     bool   `json:"-"`
}

func (s Settings) Validate() error {
	if s.ClientID == "" {
		return ErrClientIDRequired
	}
	if !strings.HasSuffix(s.ClientID, clientIDSuffix) {
		return ErrClientIDInvalid
	}
	if s.ClientSecret == "" {
		return ErrClientSecretRequired
	}
	if s.DriveFolderID != "" && !driveFolderPattern.MatchString(s.DriveFolderID) {
		return ErrDriveFolderInvalid
	}
	return nil
}

//go:generate moq -out settingsstore_mock.go . SettingsStore:MockSettingsStore
type SettingsStore interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
}

func (c *Client) Settings(_ context.Context) (Settings, error) {
	settings, err := c.loadSettings()
	if err != nil {
		return Settings{}, err
	}
	settings.HasSecret = settings.ClientSecret != ""
	settings.ClientSecret = ""
	return settings, nil
}

func (c *Client) SaveSettings(ctx context.Context, s Settings) error {
	stored, err := c.loadSettings()
	if err != nil {
		return err
	}
	if s.ClientSecret == "" {
		s.ClientSecret = stored.ClientSecret
	}
	if s.CalendarID == "" {
		s.CalendarID = defaultCalendar
	}
	if err := s.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("google: encode settings: %w", err)
	}
	if err := c.settings.Set(settingsService, settingsUser, string(data)); err != nil {
		return fmt.Errorf("google: save settings: %w", err)
	}
	if stored.ClientID != "" && stored.ClientID != s.ClientID {
		if err := c.store.Delete(ctx); err != nil {
			return fmt.Errorf("google: delete token: %w", err)
		}
	}

	slog.InfoContext(ctx, "google settings saved")
	return nil
}

func (c *Client) loadSettings() (Settings, error) {
	raw, err := c.settings.Get(settingsService, settingsUser)
	if errors.Is(err, vault.ErrNotFound) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("google: load settings: %w", err)
	}
	var settings Settings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return Settings{}, fmt.Errorf("google: decode settings: %w", err)
	}
	return settings, nil
}

func (c *Client) configured() (Settings, error) {
	settings, err := c.loadSettings()
	if err != nil {
		return Settings{}, err
	}
	if settings.Validate() != nil {
		return Settings{}, ErrNotConfigured
	}
	if settings.CalendarID == "" {
		settings.CalendarID = defaultCalendar
	}
	return settings, nil
}

func (c *Client) oauthConfig(settings Settings, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     settings.ClientID,
		ClientSecret: settings.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{calendar.CalendarEventsScope, drive.DriveFileScope},
		Endpoint:     c.endpoint,
	}
}
