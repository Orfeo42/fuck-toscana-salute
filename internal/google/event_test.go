package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const (
	fakeBooking = "ABC123456789"
	fakeKey     = "APP1"
)

type fakeGoogle struct {
	srv *httptest.Server

	getStatus    int
	getBody      string
	insertStatus int
	uploadStatus int

	deleteStatus   int
	updateStatus   int
	laterGetStatus int
	laterGetBody   string

	mu           sync.Mutex
	gets         int
	inserts      []calendar.Event
	updates      []calendar.Event
	updateQuery  []string
	deletedFiles []string
	insertQuery  []string
	insertPaths  []string
	uploadMeta   []drive.File
	uploadMedia  [][]byte
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{getStatus: http.StatusNotFound, getBody: `{"error":{"code":404,"message":"not found"}}`, insertStatus: http.StatusOK, uploadStatus: http.StatusOK, deleteStatus: http.StatusNoContent, updateStatus: http.StatusOK}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle(t)))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGoogle) handle(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/") && strings.Contains(r.URL.Path, "/events/"):
			f.gets++
			status, body := f.getStatus, f.getBody
			if f.gets > 1 && f.laterGetStatus != 0 {
				status, body = f.laterGetStatus, f.laterGetBody
			}
			w.WriteHeader(status)
			_, err := w.Write([]byte(body))
			assert.NoError(t, err)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/") && strings.Contains(r.URL.Path, "/events/"):
			var ev calendar.Event
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ev))
			f.updates = append(f.updates, ev)
			f.updateQuery = append(f.updateQuery, r.URL.RawQuery)
			if f.updateStatus != http.StatusOK {
				w.WriteHeader(f.updateStatus)
				_, err := w.Write([]byte(`{"error":{"code":500,"message":"boom"}}`))
				assert.NoError(t, err)
				return
			}
			ev.HtmlLink = "https://calendar.example.test/updated"
			assert.NoError(t, json.NewEncoder(w).Encode(ev))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/drive/v3/files/"):
			f.deletedFiles = append(f.deletedFiles, strings.TrimPrefix(r.URL.Path, "/drive/v3/files/"))
			w.WriteHeader(f.deleteStatus)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/") && strings.HasSuffix(r.URL.Path, "/events"):
			var ev calendar.Event
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ev))
			f.inserts = append(f.inserts, ev)
			f.insertQuery = append(f.insertQuery, r.URL.RawQuery)
			f.insertPaths = append(f.insertPaths, r.URL.Path)
			w.WriteHeader(f.insertStatus)
			if f.insertStatus != http.StatusOK {
				_, err := fmt.Fprintf(w, `{"error":{"code":%d,"message":"insert failed"}}`, f.insertStatus)
				assert.NoError(t, err)
				return
			}
			ev.HtmlLink = "https://calendar.example.test/event"
			assert.NoError(t, json.NewEncoder(w).Encode(ev))
		case r.Method == http.MethodPost && r.URL.Path == "/upload/drive/v3/files":
			f.recordUpload(t, r)
			w.WriteHeader(f.uploadStatus)
			if f.uploadStatus != http.StatusOK {
				_, err := w.Write([]byte(`{"error":{"code":401,"message":"unauthorized"}}`))
				assert.NoError(t, err)
				return
			}
			_, err := w.Write([]byte(`{"id":"file1","webViewLink":"https://drive.example.test/file1"}`))
			assert.NoError(t, err)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (f *fakeGoogle) recordUpload(t *testing.T, r *http.Request) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/related", mediaType)
	reader := multipart.NewReader(r.Body, params["boundary"])

	metaPart, err := reader.NextPart()
	require.NoError(t, err)
	var meta drive.File
	require.NoError(t, json.NewDecoder(metaPart).Decode(&meta))
	f.uploadMeta = append(f.uploadMeta, meta)

	mediaPart, err := reader.NextPart()
	require.NoError(t, err)
	media, err := io.ReadAll(mediaPart)
	require.NoError(t, err)
	f.uploadMedia = append(f.uploadMedia, media)
}

func connectedStore(token *oauth2.Token) *MockTokenStore {
	return &MockTokenStore{
		LoadFunc:   func(context.Context) (*oauth2.Token, error) { return token, nil },
		SaveFunc:   func(context.Context, *oauth2.Token) error { return nil },
		DeleteFunc: func(context.Context) error { return nil },
	}
}

func validToken() *oauth2.Token {
	return &oauth2.Token{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}
}

func fakeClient(t *testing.T, f *fakeGoogle, store TokenStore) *Client {
	t.Helper()
	return fakeClientWith(t, f, store, testSettings())
}

func fakeClientWith(t *testing.T, f *fakeGoogle, store TokenStore, settings Settings) *Client {
	t.Helper()
	c := newTestClient(storedSettings(t, settings), store)
	c.calendarEndpoint = f.srv.URL + "/calendar/v3/"
	c.driveEndpoint = f.srv.URL + "/drive/v3/"
	return c
}

func sampleEvent() Event {
	return Event{
		Key:      fakeKey,
		Title:    "Visita cardiologica",
		Start:    time.Date(2026, 3, 10, 8, 30, 0, 0, time.UTC),
		Duration: 45 * time.Minute,
		Location: "Ospedale Test",
		Details:  "Piano 2",
	}
}

func TestEventID(t *testing.T) {
	t.Run("is deterministic, 40 chars, lowercase base32hex with prefix", func(t *testing.T) {
		got := eventID(fakeKey)
		assert.Equal(t, got, eventID(fakeKey))
		assert.Len(t, got, 40)
		assert.True(t, strings.HasPrefix(got, "fts"))
		assert.Regexp(t, `^[a-v0-9]{40}$`, got)
	})

	t.Run("differs per key", func(t *testing.T) {
		assert.NotEqual(t, eventID("APP1"), eventID("APP2"))
	})
}

func TestAddEvent(t *testing.T) {
	ctx := context.Background()
	pdf := []byte("%PDF-fake")

	t.Run("not configured", func(t *testing.T) {
		_, err := newTestClient(emptySettings(), &MockTokenStore{}).AddEvent(ctx, sampleEvent(), nil)
		require.ErrorIs(t, err, ErrNotConfigured)
	})

	t.Run("not connected", func(t *testing.T) {
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}
		_, err := newTestClient(storedSettings(t, testSettings()), store).AddEvent(ctx, sampleEvent(), nil)
		require.ErrorIs(t, err, ErrNotConnected)
	})

	t.Run("existing event is returned without upload or insert", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.getStatus = http.StatusOK
		f.getBody = `{"id":"x","status":"confirmed","htmlLink":"https://calendar.example.test/existing"}`

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.Equal(t, EventResult{HTMLLink: "https://calendar.example.test/existing", Created: false}, got)
		assert.Empty(t, f.uploadMeta)
		assert.Empty(t, f.inserts)
	})

	t.Run("new event with pdf uploads once and attaches it", func(t *testing.T) {
		f := newFakeGoogle(t)

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.Equal(t, EventResult{HTMLLink: "https://calendar.example.test/event", Created: true}, got)

		require.Len(t, f.uploadMeta, 1)
		assert.Equal(t, "promemoria-2026-03-10.pdf", f.uploadMeta[0].Name)
		assert.Equal(t, "application/pdf", f.uploadMeta[0].MimeType)
		assert.NotContains(t, f.uploadMeta[0].Name, fakeBooking)
		assert.Equal(t, pdf, f.uploadMedia[0])

		require.Len(t, f.inserts, 1)
		ev := f.inserts[0]
		assert.Equal(t, eventID(fakeKey), ev.Id)
		assert.Equal(t, "Visita cardiologica", ev.Summary)
		assert.Equal(t, "Ospedale Test", ev.Location)
		assert.Equal(t, "Piano 2", ev.Description)
		assert.Equal(t, "2026-03-10T09:30:00+01:00", ev.Start.DateTime)
		assert.Equal(t, "2026-03-10T10:15:00+01:00", ev.End.DateTime)
		assert.Equal(t, "Europe/Rome", ev.Start.TimeZone)
		assert.Equal(t, "Europe/Rome", ev.End.TimeZone)
		require.Len(t, ev.Reminders.Overrides, 2)
		assert.Equal(t, int64(1440), ev.Reminders.Overrides[0].Minutes)
		assert.Equal(t, int64(60), ev.Reminders.Overrides[1].Minutes)
		assert.Equal(t, "popup", ev.Reminders.Overrides[0].Method)
		require.Len(t, ev.Attachments, 1)
		assert.Equal(t, "https://drive.example.test/file1", ev.Attachments[0].FileUrl)
		assert.Equal(t, "application/pdf", ev.Attachments[0].MimeType)
		assert.Equal(t, "promemoria-2026-03-10.pdf", ev.Attachments[0].Title)
		assert.Contains(t, f.insertQuery[0], "supportsAttachments=true")
	})

	t.Run("uses the calendar id and drive folder from settings", func(t *testing.T) {
		f := newFakeGoogle(t)
		settings := testSettings()
		settings.CalendarID = "team-cal@example.test"
		settings.DriveFolderID = "folder-id-12345"

		_, err := fakeClientWith(t, f, connectedStore(validToken()), settings).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		require.Len(t, f.insertPaths, 1)
		assert.Equal(t, "/calendar/v3/calendars/team-cal@example.test/events", f.insertPaths[0])
		require.Len(t, f.uploadMeta, 1)
		assert.Equal(t, []string{"folder-id-12345"}, f.uploadMeta[0].Parents)
	})

	t.Run("without a drive folder the upload has no parents", func(t *testing.T) {
		f := newFakeGoogle(t)

		_, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		require.Len(t, f.uploadMeta, 1)
		assert.Empty(t, f.uploadMeta[0].Parents)
		assert.Equal(t, "/calendar/v3/calendars/primary/events", f.insertPaths[0])
	})

	t.Run("without pdf nothing is uploaded", func(t *testing.T) {
		f := newFakeGoogle(t)

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), nil)
		require.NoError(t, err)
		assert.True(t, got.Created)
		assert.Empty(t, f.uploadMeta)
		require.Len(t, f.inserts, 1)
		assert.Empty(t, f.inserts[0].Attachments)
	})

	t.Run("cancelled event is uploaded and updated to confirmed", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.getStatus = http.StatusOK
		f.getBody = `{"id":"x","status":"cancelled"}`

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.Equal(t, EventResult{HTMLLink: "https://calendar.example.test/updated", Created: true}, got)
		assert.Len(t, f.uploadMeta, 1)
		assert.Empty(t, f.inserts)
		require.Len(t, f.updates, 1)
		assert.Equal(t, "confirmed", f.updates[0].Status)
		assert.Equal(t, eventID(fakeKey), f.updates[0].Id)
		assert.Equal(t, "Visita cardiologica", f.updates[0].Summary)
		require.Len(t, f.updates[0].Attachments, 1)
		assert.Contains(t, f.updateQuery[0], "supportsAttachments=true")
		assert.Empty(t, f.deletedFiles)
	})

	t.Run("insert conflict followed by cancelled get takes the update path", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.insertStatus = http.StatusConflict
		f.laterGetStatus = http.StatusOK
		f.laterGetBody = `{"id":"x","status":"cancelled"}`

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.True(t, got.Created)
		assert.Len(t, f.inserts, 1)
		require.Len(t, f.updates, 1)
		assert.Equal(t, "confirmed", f.updates[0].Status)
		assert.Empty(t, f.deletedFiles)
	})

	t.Run("insert conflict is treated as existing and the uploaded file is deleted", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.insertStatus = http.StatusConflict
		f.laterGetStatus = http.StatusOK
		f.laterGetBody = `{"id":"x","status":"confirmed","htmlLink":"https://calendar.example.test/raced"}`

		got, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.Equal(t, EventResult{HTMLLink: "https://calendar.example.test/raced", Created: false}, got)
		assert.Equal(t, []string{"file1"}, f.deletedFiles)
	})

	t.Run("insert failure after upload deletes the file and returns the original error", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.insertStatus = http.StatusInternalServerError

		_, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNotConnected)
		var apiErr *googleapi.Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.Equal(t, []string{"file1"}, f.deletedFiles)
	})

	t.Run("failed file cleanup does not replace the original error", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.insertStatus = http.StatusInternalServerError
		f.deleteStatus = http.StatusForbidden

		_, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		var apiErr *googleapi.Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.Equal(t, []string{"file1"}, f.deletedFiles)
	})

	t.Run("update failure after upload deletes the file", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.getStatus = http.StatusOK
		f.getBody = `{"id":"x","status":"cancelled"}`
		f.updateStatus = http.StatusInternalServerError

		_, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), pdf)
		require.Error(t, err)
		assert.Equal(t, []string{"file1"}, f.deletedFiles)
	})

	t.Run("no upload means no cleanup on failure", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.insertStatus = http.StatusInternalServerError

		_, err := fakeClient(t, f, connectedStore(validToken())).AddEvent(ctx, sampleEvent(), nil)
		require.Error(t, err)
		assert.Empty(t, f.deletedFiles)
	})

	t.Run("401 from calendar maps to not connected and deletes the token", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.getStatus = http.StatusUnauthorized
		f.getBody = `{"error":{"code":401,"message":"unauthorized"}}`
		store := connectedStore(validToken())

		_, err := fakeClient(t, f, store).AddEvent(ctx, sampleEvent(), pdf)
		require.ErrorIs(t, err, ErrNotConnected)
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("401 from drive maps to not connected and deletes the token", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.uploadStatus = http.StatusUnauthorized
		store := connectedStore(validToken())

		_, err := fakeClient(t, f, store).AddEvent(ctx, sampleEvent(), pdf)
		require.ErrorIs(t, err, ErrNotConnected)
		assert.Len(t, store.DeleteCalls(), 1)
		assert.Empty(t, f.inserts)
	})

	t.Run("other api errors are wrapped and keep the token", func(t *testing.T) {
		f := newFakeGoogle(t)
		f.getStatus = http.StatusInternalServerError
		f.getBody = `{"error":{"code":500,"message":"boom"}}`
		store := connectedStore(validToken())

		_, err := fakeClient(t, f, store).AddEvent(ctx, sampleEvent(), nil)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNotConnected)
		assert.Empty(t, store.DeleteCalls())
	})

	t.Run("refresh rejected with invalid_grant maps to not connected", func(t *testing.T) {
		f := newFakeGoogle(t)
		ts := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
		store := connectedStore(&oauth2.Token{AccessToken: "old", RefreshToken: "rt", Expiry: time.Now().Add(-time.Hour)})
		c := fakeClient(t, f, store)
		c.endpoint.TokenURL = ts.srv.URL

		_, err := c.AddEvent(ctx, sampleEvent(), nil)
		require.ErrorIs(t, err, ErrNotConnected)
		assert.Len(t, store.DeleteCalls(), 1)
	})

	t.Run("refreshed token is saved", func(t *testing.T) {
		f := newFakeGoogle(t)
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"fresh","token_type":"Bearer","expires_in":3600}`)
		store := connectedStore(&oauth2.Token{AccessToken: "old", RefreshToken: "rt", Expiry: time.Now().Add(-time.Hour)})
		c := fakeClient(t, f, store)
		c.endpoint.TokenURL = ts.srv.URL

		_, err := c.AddEvent(ctx, sampleEvent(), nil)
		require.NoError(t, err)
		require.Len(t, store.SaveCalls(), 1)
		assert.Equal(t, "fresh", store.SaveCalls()[0].Token.AccessToken)
		assert.Equal(t, "rt", store.SaveCalls()[0].Token.RefreshToken)
	})

	t.Run("unchanged token is not saved again", func(t *testing.T) {
		f := newFakeGoogle(t)
		store := connectedStore(validToken())

		_, err := fakeClient(t, f, store).AddEvent(ctx, sampleEvent(), pdf)
		require.NoError(t, err)
		assert.Empty(t, store.SaveCalls())
	})

	t.Run("save failure after refresh is returned", func(t *testing.T) {
		f := newFakeGoogle(t)
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"fresh","token_type":"Bearer","expires_in":3600}`)
		store := connectedStore(&oauth2.Token{AccessToken: "old", RefreshToken: "rt", Expiry: time.Now().Add(-time.Hour)})
		store.SaveFunc = func(context.Context, *oauth2.Token) error { return errors.New("keyring down") }
		c := fakeClient(t, f, store)
		c.endpoint.TokenURL = ts.srv.URL

		_, err := c.AddEvent(ctx, sampleEvent(), nil)
		require.Error(t, err)
	})
}
