package google

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
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

	t.Run("client id must end with the google suffix", func(t *testing.T) {
		s := testSettings()
		s.ClientID = "client-id"
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
		s.ClientID = "other" + clientIDSuffix

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
		s.ClientID = "other" + clientIDSuffix

		require.Error(t, newTestClient(storedSettings(t, testSettings()), tokens).SaveSettings(ctx, s))
	})

	t.Run("set failure is returned and token kept", func(t *testing.T) {
		store := storedSettings(t, testSettings())
		store.SetFunc = func(string, string, string) error { return errors.New("keyring down") }
		tokens := &MockTokenStore{}
		s := testSettings()
		s.ClientID = "other" + clientIDSuffix

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
