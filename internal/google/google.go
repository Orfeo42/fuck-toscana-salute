package google

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
)

type googleError string

func (e googleError) Error() string { return string(e) }

const (
	ErrNotConfigured  googleError = "google: not configured"
	ErrNotConnected   googleError = "google: not connected"
	ErrNoRefreshToken googleError = "google: no refresh token returned, grant reset"
	ErrScopesMissing  googleError = "google: required scopes not granted"
)

const revokeURL = "https://oauth2.googleapis.com/revoke"

type Status struct {
	Configured bool
	Connected  bool
	ClientID   string
	Drive      bool
}

//go:generate moq -out tokenstore_mock.go . TokenStore:MockTokenStore
type TokenStore interface {
	Load(ctx context.Context) (*oauth2.Token, error)
	Save(ctx context.Context, token *oauth2.Token) error
	Delete(ctx context.Context) error
}

type Client struct {
	settings         SettingsStore
	store            TokenStore
	endpoint         oauth2.Endpoint
	httpClient       *http.Client
	now              func() time.Time
	calendarEndpoint string
	driveEndpoint    string
	revokeURL        string
}

func New(settings SettingsStore, tokens TokenStore, httpClient *http.Client) *Client {
	return &Client{
		settings:   settings,
		store:      tokens,
		endpoint:   google.Endpoint,
		httpClient: httpClient,
		now:        time.Now,
		revokeURL:  revokeURL,
	}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	settings, err := c.loadSettings()
	if err != nil {
		return Status{}, err
	}
	if settings.Validate() != nil {
		return Status{}, nil
	}
	token, err := c.store.Load(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("google: status: %w", err)
	}
	return Status{Configured: true, Connected: token != nil, ClientID: settings.ClientID, Drive: settings.APIKey != ""}, nil
}

func (c *Client) ExchangeCode(ctx context.Context, code, origin string) error {
	settings, err := c.configured()
	if err != nil {
		return err
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	token, err := c.oauthConfig(settings, origin).Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("google: exchange: %w", err)
	}
	if token.RefreshToken == "" {
		c.revoke(ctx, token)
		return ErrNoRefreshToken
	}
	raw, ok := token.Extra("scope").(string)
	granted := strings.Fields(raw)
	if !ok || !slices.Contains(granted, calendar.CalendarEventsScope) || !slices.Contains(granted, drive.DriveFileScope) {
		c.revoke(ctx, token)
		return ErrScopesMissing
	}
	if err := c.store.Save(ctx, token); err != nil {
		return fmt.Errorf("google: save token: %w", err)
	}
	slog.InfoContext(ctx, "google connected")
	return nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	token, err := c.store.Load(ctx)
	if err != nil {
		slog.WarnContext(ctx, "google revoke skipped", slog.String("error", err.Error()))
	}
	if token != nil {
		c.revoke(ctx, token)
	}
	if err := c.store.Delete(ctx); err != nil {
		return fmt.Errorf("google: delete token: %w", err)
	}
	return nil
}

func (c *Client) revoke(ctx context.Context, token *oauth2.Token) {
	value := token.RefreshToken
	if value == "" {
		value = token.AccessToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.revokeURL, strings.NewReader(url.Values{"token": {value}}.Encode()))
	if err != nil {
		slog.WarnContext(ctx, "google revoke failed", slog.String("error", "build request"))
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "google revoke failed", slog.String("error", "request failed"))
		return
	}
	if err := resp.Body.Close(); err != nil {
		slog.WarnContext(ctx, "google revoke close failed", slog.String("error", err.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		slog.WarnContext(ctx, "google revoke failed", slog.Int("status", resp.StatusCode))
	}
}
