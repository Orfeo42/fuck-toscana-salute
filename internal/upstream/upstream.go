package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	maxResponseBytes = 8 << 20
	maxErrorBodyKept = 4 << 10
	maxShapeKeys     = 20
	userAgent        = "fuck-toscana-salute"
)

var exceptionClassPattern = regexp.MustCompile(`\b(?:[a-z_][a-z0-9_]*\.)+[A-Z][A-Za-z0-9_$]*(?:Exception|Error|Fault)\b`)

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

func (e *StatusError) ExceptionClass() string {
	return string(exceptionClassPattern.Find(e.body))
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

func (e *StatusError) Shape() string {
	if len(e.body) == 0 {
		return "empty"
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(e.body, &object); err == nil && object != nil {
		keys := slices.Sorted(maps.Keys(object))
		if len(keys) > maxShapeKeys {
			return "json{" + strings.Join(keys[:maxShapeKeys], ",") + ",…}"
		}
		return "json{" + strings.Join(keys, ",") + "}"
	}
	var array []json.RawMessage
	if err := json.Unmarshal(e.body, &array); err == nil && array != nil {
		return "json[" + strconv.Itoa(len(array)) + "]"
	}
	if json.Valid(e.body) {
		return "json:scalar"
	}
	shape := "text len=" + strconv.Itoa(len(e.body))
	if bytes.Contains(bytes.ToLower(e.body), []byte("<html")) {
		shape += " html"
	}
	return shape
}
