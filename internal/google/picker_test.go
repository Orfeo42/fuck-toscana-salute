package google

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type fakeDrive struct {
	srv *httptest.Server

	metaStatus int
	mimeType   string
	metaSize   string
	body       string
}

func newFakeDrive(t *testing.T, mimeType, body string) *fakeDrive {
	t.Helper()
	f := &fakeDrive{metaStatus: http.StatusOK, mimeType: mimeType, metaSize: fmt.Sprint(len(body)), body: body}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/drive/v3/files/file1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("alt") == "media" {
			_, err := w.Write([]byte(f.body))
			assert.NoError(t, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.metaStatus)
		if f.metaStatus != http.StatusOK {
			_, err := w.Write([]byte(`{"error":{"code":401,"message":"unauthorized"}}`))
			assert.NoError(t, err)
			return
		}
		_, err := fmt.Fprintf(w, `{"name":"referto","mimeType":%q,"size":%q}`, f.mimeType, f.metaSize)
		assert.NoError(t, err)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func pickerSettings() Settings {
	s := testSettings()
	s.APIKey = fakeAPIKey
	s.AppID = fakeAppID
	return s
}

func driveClient(t *testing.T, f *fakeDrive, store TokenStore) *Client {
	t.Helper()
	c := newTestClient(storedSettings(t, pickerSettings()), store)
	c.driveEndpoint = f.srv.URL + "/drive/v3/"
	return c
}

func TestPickerConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		_, err := newTestClient(emptySettings(), &MockTokenStore{}).PickerConfig(ctx)
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("missing api key", func(t *testing.T) {
		s := pickerSettings()
		s.APIKey = ""
		_, err := newTestClient(storedSettings(t, s), connectedStore(validToken())).PickerConfig(ctx)
		require.ErrorIs(t, err, ErrPickerNotConfigured)
	})

	t.Run("missing app id", func(t *testing.T) {
		s := pickerSettings()
		s.AppID = ""
		_, err := newTestClient(storedSettings(t, s), connectedStore(validToken())).PickerConfig(ctx)
		require.ErrorIs(t, err, ErrPickerNotConfigured)
	})

	t.Run("not connected", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		_, err := newTestClient(storedSettings(t, pickerSettings()), store).PickerConfig(ctx)
		require.ErrorIs(t, err, ErrNotConnected)
	})

	t.Run("valid token is returned without saving", func(t *testing.T) {
		store := connectedStore(validToken())

		got, err := newTestClient(storedSettings(t, pickerSettings()), store).PickerConfig(ctx)
		require.NoError(t, err)
		assert.Equal(t, PickerConfig{AccessToken: "at", APIKey: fakeAPIKey, AppID: fakeAppID}, got)
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("expired token is refreshed and saved", func(t *testing.T) {
		tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"access_token":"new-at","token_type":"Bearer","expires_in":3600}`))
			assert.NoError(t, err)
		}))
		t.Cleanup(tokenSrv.Close)
		expired := &oauth2.Token{AccessToken: "old", RefreshToken: "rt", TokenType: "Bearer", Expiry: time.Now().Add(-time.Hour)}
		store := connectedStore(expired)
		c := newTestClient(storedSettings(t, pickerSettings()), store)
		c.endpoint.TokenURL = tokenSrv.URL

		got, err := c.PickerConfig(ctx)
		require.NoError(t, err)
		assert.Equal(t, "new-at", got.AccessToken)
		require.Len(t, store.SaveCalls(), 1)
		assert.Equal(t, "new-at", store.SaveCalls()[0].Token.AccessToken)
	})

	t.Run("invalid grant deletes the token", func(t *testing.T) {
		tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error":"invalid_grant"}`))
			assert.NoError(t, err)
		}))
		t.Cleanup(tokenSrv.Close)
		expired := &oauth2.Token{AccessToken: "old", RefreshToken: "rt", TokenType: "Bearer", Expiry: time.Now().Add(-time.Hour)}
		store := connectedStore(expired)
		c := newTestClient(storedSettings(t, pickerSettings()), store)
		c.endpoint.TokenURL = tokenSrv.URL

		_, err := c.PickerConfig(ctx)
		require.ErrorIs(t, err, ErrNotConnected)
		assert.Len(t, store.DeleteCalls(), 1)
	})
}

func TestDownloadFile(t *testing.T) {
	ctx := context.Background()

	t.Run("not configured", func(t *testing.T) {
		_, err := newTestClient(emptySettings(), &MockTokenStore{}).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("not connected", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		_, err := newTestClient(storedSettings(t, pickerSettings()), store).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrNotConnected)
	})

	t.Run("pdf is accepted", func(t *testing.T) {
		f := newFakeDrive(t, "application/pdf", "%PDF-fake")

		got, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.NoError(t, err)
		assert.Equal(t, DriveFile{Name: "referto", MimeType: "application/pdf", Data: []byte("%PDF-fake")}, got)
	})

	t.Run("jpeg is accepted", func(t *testing.T) {
		f := newFakeDrive(t, "image/jpeg", "jpeg-bytes")

		got, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", got.MimeType)
		assert.Equal(t, []byte("jpeg-bytes"), got.Data)
	})

	t.Run("png is accepted", func(t *testing.T) {
		f := newFakeDrive(t, "image/png", "png-bytes")

		got, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.NoError(t, err)
		assert.Equal(t, "image/png", got.MimeType)
		assert.Equal(t, []byte("png-bytes"), got.Data)
	})

	t.Run("google doc is rejected", func(t *testing.T) {
		f := newFakeDrive(t, "application/vnd.google-apps.document", "")

		_, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrDriveFileType)
	})

	t.Run("size metadata over the limit is rejected", func(t *testing.T) {
		f := newFakeDrive(t, "application/pdf", "x")
		f.metaSize = fmt.Sprint(MaxDriveFileBytes + 1)

		_, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrDriveFileTooLarge)
	})

	t.Run("body over the limit is rejected", func(t *testing.T) {
		f := newFakeDrive(t, "application/pdf", strings.Repeat("x", MaxDriveFileBytes+1))
		f.metaSize = "1"

		_, err := driveClient(t, f, connectedStore(validToken())).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrDriveFileTooLarge)
	})

	t.Run("unauthorized deletes the token", func(t *testing.T) {
		f := newFakeDrive(t, "application/pdf", "x")
		f.metaStatus = http.StatusUnauthorized
		store := connectedStore(validToken())

		_, err := driveClient(t, f, store).DownloadFile(ctx, "file1")
		require.ErrorIs(t, err, ErrNotConnected)
		assert.Len(t, store.DeleteCalls(), 1)
	})
}
