package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const (
	fakeAPIKey = "AIza" + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	fakeAppID  = "123456789012"
)

func TestSettingsValidate(t *testing.T) {
	t.Run("valid settings pass", func(t *testing.T) {
		require.NoError(t, testSettings().Validate())
	})

	t.Run("client id required", func(t *testing.T) {
		s := testSettings()
		s.ClientID = ""
		require.ErrorIs(t, s.Validate(), ErrClientIDRequired)
	})

	t.Run("client id without the google suffix is rejected", func(t *testing.T) {
		s := testSettings()
		s.ClientID = "123456789012-abc"
		require.ErrorIs(t, s.Validate(), ErrClientIDInvalid)
	})

	t.Run("client id without the project number is rejected", func(t *testing.T) {
		s := testSettings()
		s.ClientID = "client-id.apps.googleusercontent.com"
		require.ErrorIs(t, s.Validate(), ErrClientIDInvalid)
	})

	t.Run("client secret required", func(t *testing.T) {
		s := testSettings()
		s.ClientSecret = ""
		require.ErrorIs(t, s.Validate(), ErrClientSecretRequired)
	})

	t.Run("calendar id is optional", func(t *testing.T) {
		s := testSettings()
		s.CalendarID = ""
		require.NoError(t, s.Validate())
	})

	t.Run("drive folder id is optional", func(t *testing.T) {
		s := testSettings()
		s.DriveFolderID = ""
		require.NoError(t, s.Validate())
	})

	t.Run("drive folder id too short is rejected", func(t *testing.T) {
		s := testSettings()
		s.DriveFolderID = "short"
		require.ErrorIs(t, s.Validate(), ErrDriveFolderInvalid)
	})

	t.Run("drive folder id with bad characters is rejected", func(t *testing.T) {
		s := testSettings()
		s.DriveFolderID = "folder/id/12345"
		require.ErrorIs(t, s.Validate(), ErrDriveFolderInvalid)
	})

	t.Run("valid drive folder id passes", func(t *testing.T) {
		s := testSettings()
		s.DriveFolderID = "1AbC_dEf-GhIjK"
		require.NoError(t, s.Validate())
	})
}

func TestSettingsValidatePicker(t *testing.T) {
	t.Run("api key is optional", func(t *testing.T) {
		s := testSettings()
		s.APIKey = ""
		require.NoError(t, s.Validate())
	})

	t.Run("valid api key passes", func(t *testing.T) {
		s := testSettings()
		s.APIKey = fakeAPIKey
		require.NoError(t, s.Validate())
	})

	t.Run("api key with wrong prefix is rejected", func(t *testing.T) {
		s := testSettings()
		s.APIKey = "BBBB" + strings.Repeat("x", 35)
		require.ErrorIs(t, s.Validate(), ErrAPIKeyInvalid)
	})

	t.Run("api key with wrong length is rejected", func(t *testing.T) {
		s := testSettings()
		s.APIKey = fakeAPIKey + "x"
		require.ErrorIs(t, s.Validate(), ErrAPIKeyInvalid)
	})
}

func TestSettingsValidateGuests(t *testing.T) {
	t.Run("valid guests pass", func(t *testing.T) {
		s := testSettings()
		s.Guests = []string{"guest@example.com", "other@example.com"}
		require.NoError(t, s.Validate())
	})

	t.Run("invalid email is rejected", func(t *testing.T) {
		s := testSettings()
		s.Guests = []string{"not-an-email"}
		require.ErrorIs(t, s.Validate(), ErrGuestInvalid)
	})

	t.Run("display name form is rejected", func(t *testing.T) {
		s := testSettings()
		s.Guests = []string{"Guest <guest@example.com>"}
		require.ErrorIs(t, s.Validate(), ErrGuestInvalid)
	})

	t.Run("eleven guests are rejected", func(t *testing.T) {
		s := testSettings()
		for i := range 11 {
			s.Guests = append(s.Guests, fmt.Sprintf("guest%d@example.com", i))
		}
		require.ErrorIs(t, s.Validate(), ErrTooManyGuests)
	})

	t.Run("ten guests pass", func(t *testing.T) {
		s := testSettings()
		for i := range 10 {
			s.Guests = append(s.Guests, fmt.Sprintf("guest%d@example.com", i))
		}
		require.NoError(t, s.Validate())
	})
}

func TestValidateGuests(t *testing.T) {
	t.Run("trims, drops empties and dedupes case-insensitively keeping the first spelling", func(t *testing.T) {
		got, err := ValidateGuests([]string{"  Guest@Example.com ", "", "guest@example.com", "other@example.com"})
		require.NoError(t, err)
		assert.Equal(t, []string{"Guest@Example.com", "other@example.com"}, got)
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		got, err := ValidateGuests(nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("invalid entry is rejected", func(t *testing.T) {
		_, err := ValidateGuests([]string{"nope"})
		require.ErrorIs(t, err, ErrGuestInvalid)
	})
}

func TestSettingsAppID(t *testing.T) {
	t.Run("is the project number of the client id", func(t *testing.T) {
		assert.Equal(t, fakeAppID, testSettings().AppID())
	})

	t.Run("is empty without a client id", func(t *testing.T) {
		assert.Empty(t, Settings{}.AppID())
	})
}

func TestSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("missing entry returns zero settings and not configured", func(t *testing.T) {
		c := newTestClient(emptySettings(), &MockTokenStore{})

		got, err := c.Settings(ctx)
		require.NoError(t, err)
		assert.Equal(t, Settings{}, got)

		status, err := c.Status(ctx)
		require.NoError(t, err)
		assert.False(t, status.Configured)
	})

	t.Run("blanks the client secret", func(t *testing.T) {
		stored := testSettings()
		stored.DriveFolderID = "folder-id-12345"
		got, err := newTestClient(storedSettings(t, stored), &MockTokenStore{}).Settings(ctx)
		require.NoError(t, err)
		assert.Empty(t, got.ClientSecret)
		assert.True(t, got.HasSecret)
		assert.Equal(t, stored.ClientID, got.ClientID)
		assert.Equal(t, stored.CalendarID, got.CalendarID)
		assert.Equal(t, stored.DriveFolderID, got.DriveFolderID)
	})

	t.Run("stored json with the legacy app id key still decodes", func(t *testing.T) {
		raw := `{"clientId":"` + testClientID + `","clientSecret":"s","appId":"123456789"}`
		store := &MockSettingsStore{GetFunc: func(string, string) (string, error) { return raw, nil }}
		got, err := newTestClient(store, &MockTokenStore{}).Settings(ctx)
		require.NoError(t, err)
		assert.Equal(t, testClientID, got.ClientID)
	})

	t.Run("store failure is returned", func(t *testing.T) {
		store := &MockSettingsStore{GetFunc: func(string, string) (string, error) { return "", errors.New("keyring down") }}
		_, err := newTestClient(store, &MockTokenStore{}).Settings(ctx)
		require.Error(t, err)
	})

	t.Run("corrupt entry is returned as error", func(t *testing.T) {
		store := &MockSettingsStore{GetFunc: func(string, string) (string, error) { return "{", nil }}
		_, err := newTestClient(store, &MockTokenStore{}).Settings(ctx)
		require.Error(t, err)
	})
}

func TestSaveSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("stores json under the settings key", func(t *testing.T) {
		store := emptySettings()
		c := newTestClient(store, &MockTokenStore{})

		require.NoError(t, c.SaveSettings(ctx, testSettings()))
		require.Len(t, store.SetCalls(), 1)
		call := store.SetCalls()[0]
		assert.Equal(t, "fuck-toscana-salute", call.Service)
		assert.Equal(t, "google-settings", call.User)
		var saved Settings
		require.NoError(t, json.Unmarshal([]byte(call.Value), &saved))
		assert.Equal(t, testSettings(), saved)
	})

	t.Run("client id is validated", func(t *testing.T) {
		store := emptySettings()
		s := testSettings()
		s.ClientID = "nope"
		require.ErrorIs(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s), ErrClientIDInvalid)
		assert.Empty(t, store.SetCalls())
	})

	t.Run("client secret is validated when none is stored", func(t *testing.T) {
		s := testSettings()
		s.ClientSecret = ""
		require.ErrorIs(t, newTestClient(emptySettings(), &MockTokenStore{}).SaveSettings(ctx, s), ErrClientSecretRequired)
	})

	t.Run("guests are trimmed and deduped before storing", func(t *testing.T) {
		store := emptySettings()
		s := testSettings()
		s.Guests = []string{" guest@example.com ", "GUEST@example.com", ""}

		require.NoError(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s))
		var saved Settings
		require.NoError(t, json.Unmarshal([]byte(store.SetCalls()[0].Value), &saved))
		assert.Equal(t, []string{"guest@example.com"}, saved.Guests)
	})

	t.Run("invalid guest is rejected and nothing is stored", func(t *testing.T) {
		store := emptySettings()
		s := testSettings()
		s.Guests = []string{"nope"}

		require.ErrorIs(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s), ErrGuestInvalid)
		assert.Empty(t, store.SetCalls())
	})

	t.Run("drive folder id is validated", func(t *testing.T) {
		s := testSettings()
		s.DriveFolderID = "x"
		require.ErrorIs(t, newTestClient(emptySettings(), &MockTokenStore{}).SaveSettings(ctx, s), ErrDriveFolderInvalid)
	})

	t.Run("empty calendar id defaults to primary", func(t *testing.T) {
		store := emptySettings()
		s := testSettings()
		s.CalendarID = ""

		require.NoError(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s))
		var saved Settings
		require.NoError(t, json.Unmarshal([]byte(store.SetCalls()[0].Value), &saved))
		assert.Equal(t, "primary", saved.CalendarID)
	})

	t.Run("empty secret keeps the stored one", func(t *testing.T) {
		store := storedSettings(t, testSettings())
		s := testSettings()
		s.ClientSecret = ""
		s.CalendarID = "other-cal"

		require.NoError(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s))
		var saved Settings
		require.NoError(t, json.Unmarshal([]byte(store.SetCalls()[0].Value), &saved))
		assert.Equal(t, testSettings().ClientSecret, saved.ClientSecret)
		assert.Equal(t, "other-cal", saved.CalendarID)
	})

	t.Run("a given secret replaces the stored one", func(t *testing.T) {
		store := storedSettings(t, testSettings())
		s := testSettings()
		s.ClientSecret = "new-secret"

		require.NoError(t, newTestClient(store, &MockTokenStore{}).SaveSettings(ctx, s))
		var saved Settings
		require.NoError(t, json.Unmarshal([]byte(store.SetCalls()[0].Value), &saved))
		assert.Equal(t, "new-secret", saved.ClientSecret)
	})

	t.Run("client id change deletes the token", func(t *testing.T) {
		tokens := &MockTokenStore{DeleteFunc: func(context.Context) error { return nil }}
		s := testSettings()
		s.ClientID = "999999999999-other.apps.googleusercontent.com"

		require.NoError(t, newTestClient(storedSettings(t, testSettings()), tokens).SaveSettings(ctx, s))
		assert.Len(t, tokens.DeleteCalls(), 1)
	})

	t.Run("same client id keeps the token", func(t *testing.T) {
		tokens := &MockTokenStore{}

		require.NoError(t, newTestClient(storedSettings(t, testSettings()), tokens).SaveSettings(ctx, testSettings()))
		assert.Empty(t, tokens.DeleteCalls())
	})

	t.Run("token delete failure is returned", func(t *testing.T) {
		tokens := &MockTokenStore{DeleteFunc: func(context.Context) error { return errors.New("keyring down") }}
		s := testSettings()
		s.ClientID = "999999999999-other.apps.googleusercontent.com"

		require.Error(t, newTestClient(storedSettings(t, testSettings()), tokens).SaveSettings(ctx, s))
	})

	t.Run("set failure is returned and token kept", func(t *testing.T) {
		store := storedSettings(t, testSettings())
		store.SetFunc = func(string, string, string) error { return errors.New("keyring down") }
		tokens := &MockTokenStore{}
		s := testSettings()
		s.ClientID = "999999999999-other.apps.googleusercontent.com"

		require.Error(t, newTestClient(store, tokens).SaveSettings(ctx, s))
		assert.Empty(t, tokens.DeleteCalls())
	})
}

func TestStatusWithInvalidStoredSettings(t *testing.T) {
	t.Run("stored settings failing validation are not configured", func(t *testing.T) {
		stored := testSettings()
		stored.ClientSecret = ""
		store := &MockTokenStore{LoadFunc: func(context.Context) (*oauth2.Token, error) { return nil, nil }}

		got, err := newTestClient(storedSettings(t, stored), store).Status(context.Background())
		require.NoError(t, err)
		assert.Equal(t, Status{}, got)
	})
}
