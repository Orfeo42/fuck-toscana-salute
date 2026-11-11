package vault

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestKeyringBackend(t *testing.T) {
	t.Run("set then get round trips", func(t *testing.T) {
		keyring.MockInit()
		b := KeyringBackend{}
		require.NoError(t, b.Set(testService, testUser, "secret"))

		got, err := b.Get(testService, testUser)
		require.NoError(t, err)
		assert.Equal(t, "secret", got)
	})

	t.Run("get missing maps to ErrNotFound", func(t *testing.T) {
		keyring.MockInit()
		_, err := KeyringBackend{}.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("delete removes the entry", func(t *testing.T) {
		keyring.MockInit()
		b := KeyringBackend{}
		require.NoError(t, b.Set(testService, testUser, "secret"))
		require.NoError(t, b.Delete(testService, testUser))

		_, err := b.Get(testService, testUser)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("delete missing maps to ErrNotFound", func(t *testing.T) {
		keyring.MockInit()
		err := KeyringBackend{}.Delete(testService, testUser)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
