package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	maxResponseBytes = 8 << 20
	maxErrorBodyKept = 4 << 10
	userAgent        = "fuck-toscana-salute"
)

type StatusError struct {
	Status int
	body   []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("upstream: unexpected status %d", e.Status)
}

func (e *StatusError) BodyContains(substr string) bool {
	return bytes.Contains(e.body, []byte(substr))
}

type Client struct {
	httpClient *http.Client
}

func New(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

func GetJSON[Resp any](ctx context.Context, c *Client, url string, header http.Header) (Resp, error) {
	return do[Resp](ctx, c, http.MethodGet, url, header, nil)
}

func PostJSON[Req, Resp any](ctx context.Context, c *Client, url string, header http.Header, body Req) (Resp, error) {
	var zero Resp
	payload, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("upstream: encode request: %w", err)
	}
	return do[Resp](ctx, c, http.MethodPost, url, header, bytes.NewReader(payload))
}

func do[Resp any](ctx context.Context, c *Client, method, rawURL string, header http.Header, body io.Reader) (out Resp, err error) {
	var zero Resp
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return zero, fmt.Errorf("upstream: build request: %w", stripURL(err))
	}
	for k, v := range header {
		req.Header[k] = append([]string(nil), v...)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("upstream: request %s: %w", req.URL.Path, stripURL(err))
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return zero, fmt.Errorf("upstream: read %s: %w", req.URL.Path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return zero, &StatusError{Status: resp.StatusCode, body: data[:min(len(data), maxErrorBodyKept)]}
	}

	var decoded Resp
	if err := json.Unmarshal(data, &decoded); err != nil {
		return zero, fmt.Errorf("upstream: decode %s: %w", req.URL.Path, err)
	}
	return decoded, nil
}

func stripURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
