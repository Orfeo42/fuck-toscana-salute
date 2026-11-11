package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fuck-toscana-salute/internal/session"
)

const (
	TokenURL    = "https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/token"
	ClientIDCUP = "r_toscan__portale_cup2_0"
	ClientIDFSE = "r_toscan__fse_web"
)

const maxBodyBytes = 1 << 20

type keycloakError string

func (e keycloakError) Error() string { return string(e) }

const (
	ErrUnknownPortal     = keycloakError("keycloak: unknown portal")
	ErrUnexpectedStatus  = keycloakError("keycloak: unexpected status")
	ErrMalformedResponse = keycloakError("keycloak: malformed response")
)

var _ session.Refresher = (*Client)(nil)

type Client struct {
	httpClient *http.Client
	tokenURL   string
	clientIDs  map[session.Portal]string
	now        func() time.Time
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func New(httpClient *http.Client, tokenURL string) *Client {
	return &Client{
		httpClient: httpClient,
		tokenURL:   tokenURL,
		clientIDs: map[session.Portal]string{
			session.PortalCUP: ClientIDCUP,
			session.PortalFSE: ClientIDFSE,
		},
		now: time.Now,
	}
}

func (c *Client) Refresh(ctx context.Context, portal session.Portal, refreshToken string) (session.Tokens, error) {
	clientID, ok := c.clientIDs[portal]
	if !ok {
		return session.Tokens{}, ErrUnknownPortal
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return session.Tokens{}, fmt.Errorf("keycloak: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fuck-toscana-salute")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return session.Tokens{}, fmt.Errorf("keycloak: refresh request: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return session.Tokens{}, fmt.Errorf("keycloak: read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return c.parseTokens(body)
	case http.StatusBadRequest, http.StatusUnauthorized:
		var e errorResponse
		if json.Unmarshal(body, &e) == nil && e.Error == "invalid_grant" {
			return session.Tokens{}, fmt.Errorf("%w: %s", session.ErrRefreshRejected, e.ErrorDescription)
		}
	}
	return session.Tokens{}, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
}

func (c *Client) parseTokens(body []byte) (session.Tokens, error) {
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return session.Tokens{}, fmt.Errorf("%w: invalid json", ErrMalformedResponse)
	}
	if tr.AccessToken == "" || tr.RefreshToken == "" {
		return session.Tokens{}, fmt.Errorf("%w: missing token", ErrMalformedResponse)
	}
	now := c.now()
	return session.Tokens{
		Access:           tr.AccessToken,
		Refresh:          tr.RefreshToken,
		AccessExpiresAt:  now.Add(time.Duration(tr.ExpiresIn) * time.Second),
		RefreshExpiresAt: now.Add(time.Duration(tr.RefreshExpiresIn) * time.Second),
	}, nil
}
