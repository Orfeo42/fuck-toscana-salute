package google

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"golang.org/x/oauth2"
)

const (
	ErrPickerNotConfigured googleError = "google: picker api key missing"
	ErrDriveFileType       googleError = "google: drive file type not supported"
	ErrDriveFileTooLarge   googleError = "google: drive file too large"
	ErrAPIKeyInvalid       googleError = "google: api key is invalid"
)

const (
	MaxDriveFileBytes = 20 << 20

	jpegMimeType = "image/jpeg"
	pngMimeType  = "image/png"
)

type PickerConfig struct {
	AccessToken string
	APIKey      string
	AppID       string
}

type DriveFile struct {
	Name     string
	MimeType string
	Data     []byte
}

func (c *Client) savingTokenSource(ctx context.Context, settings Settings) (oauth2.TokenSource, error) {
	token, err := c.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("google: load token: %w", err)
	}
	if token == nil {
		return nil, ErrNotConnected
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	return &savingSource{ctx: ctx, src: c.oauthConfig(settings, "").TokenSource(ctx, token), store: c.store, last: token.AccessToken}, nil
}

func (c *Client) PickerConfig(ctx context.Context) (PickerConfig, error) {
	settings, err := c.configured()
	if err != nil {
		return PickerConfig{}, err
	}
	if settings.APIKey == "" {
		return PickerConfig{}, ErrPickerNotConfigured
	}
	ts, err := c.savingTokenSource(ctx, settings)
	if err != nil {
		return PickerConfig{}, err
	}
	token, err := ts.Token()
	if err != nil {
		return PickerConfig{}, c.classify(ctx, err)
	}
	slog.InfoContext(ctx, "google picker config issued")
	return PickerConfig{AccessToken: token.AccessToken, APIKey: settings.APIKey, AppID: settings.AppID()}, nil
}

func (c *Client) DownloadFile(ctx context.Context, fileID string) (DriveFile, error) {
	settings, err := c.configured()
	if err != nil {
		return DriveFile{}, err
	}
	ts, err := c.savingTokenSource(ctx, settings)
	if err != nil {
		return DriveFile{}, err
	}
	svc, err := c.driveService(ctx, ts)
	if err != nil {
		return DriveFile{}, err
	}

	meta, err := svc.Files.Get(fileID).Fields("name,mimeType,size").Context(ctx).Do()
	if err != nil {
		return DriveFile{}, c.classify(ctx, err)
	}
	if meta.MimeType != pdfMimeType && meta.MimeType != jpegMimeType && meta.MimeType != pngMimeType {
		return DriveFile{}, ErrDriveFileType
	}
	if meta.Size > MaxDriveFileBytes {
		return DriveFile{}, ErrDriveFileTooLarge
	}

	resp, err := svc.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return DriveFile{}, c.classify(ctx, err)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxDriveFileBytes+1))
	if closeErr := resp.Body.Close(); closeErr != nil {
		slog.WarnContext(ctx, "drive file body close failed")
	}
	if err != nil {
		return DriveFile{}, fmt.Errorf("google: read drive file: %w", err)
	}
	if len(data) > MaxDriveFileBytes {
		return DriveFile{}, ErrDriveFileTooLarge
	}

	slog.InfoContext(ctx, "drive file downloaded", slog.String("mime", meta.MimeType), slog.Int("size", len(data)))
	return DriveFile{Name: meta.Name, MimeType: meta.MimeType, Data: data}, nil
}
