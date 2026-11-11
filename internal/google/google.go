package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleError string

func (e googleError) Error() string { return string(e) }

const (
	ErrNotConfigured googleError = "google: not configured"
	ErrNotConnected  googleError = "google: not connected"
	ErrBadState      googleError = "google: unknown or expired state"
)

const (
	stateBytes = 32
	stateTTL   = 10 * time.Minute
	revokeURL  = "https://oauth2.googleapis.com/revoke"
)

type Status struct {
	Configured bool
	Connected  bool
}

type pendingAuth struct {
	verifier    string
	redirectURL string
	expiresAt   time.Time
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

	mu      sync.Mutex
	pending map[string]pendingAuth
}

func New(settings SettingsStore, tokens TokenStore, httpClient *http.Client) *Client {
	return &Client{
		settings:   settings,
		store:      tokens,
		endpoint:   google.Endpoint,
		httpClient: httpClient,
		now:        time.Now,
		revokeURL:  revokeURL,
		pending:    map[string]pendingAuth{},
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
	return Status{Configured: true, Connected: token != nil}, nil
}

func (c *Client) AuthURL(_ context.Context, redirectURL string) (string, error) {
	settings, err := c.configured()
	if err != nil {
		return "", err
	}

	raw := make([]byte, stateBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("google: random state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	verifier := oauth2.GenerateVerifier()

	now := c.now()
	c.mu.Lock()
	for key, p := range c.pending {
		if !now.Before(p.expiresAt) {
			delete(c.pending, key)
		}
	}
	c.pending[state] = pendingAuth{verifier: verifier, redirectURL: redirectURL, expiresAt: now.Add(stateTTL)}
	c.mu.Unlock()

	return c.oauthConfig(settings, redirectURL).AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier)), nil
}

func (c *Client) Exchange(ctx context.Context, state, code string) error {
	settings, err := c.configured()
	if err != nil {
		return err
	}

	c.mu.Lock()
	p, ok := c.pending[state]
	delete(c.pending, state)
	c.mu.Unlock()
	if !ok || !c.now().Before(p.expiresAt) {
		return ErrBadState
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	token, err := c.oauthConfig(settings, p.redirectURL).Exchange(ctx, code, oauth2.VerifierOption(p.verifier))
	if err != nil {
		return fmt.Errorf("google: exchange: %w", err)
	}
	if err := c.store.Save(ctx, token); err != nil {
		return fmt.Errorf("google: save token: %w", err)
	}
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
