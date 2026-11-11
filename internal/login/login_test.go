package login

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
)

func fakeJWT(payload string) string {
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"none"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".sig"
}

func TestResultFromStorage(t *testing.T) {
	t.Run("valid snapshot", func(t *testing.T) {
		tok := fakeJWT(`{"exp":1700000000}`)
		got, err := resultFromStorage(session.PortalFSE, storageSnapshot{Access: tok, Refresh: tok, CF: "rssmra80a01h501u"})
		require.NoError(t, err)
		assert.Equal(t, session.PortalFSE, got.Portal)
		assert.Equal(t, tok, got.Tokens.Access)
		assert.Equal(t, tok, got.Tokens.Refresh)
		assert.Equal(t, time.Unix(1700000000, 0), got.Tokens.AccessExpiresAt.Local())
		assert.Equal(t, time.Unix(1700000000, 0), got.Tokens.RefreshExpiresAt.Local())
		assert.Equal(t, "RSSMRA80A01H501U", got.Identity.CodiceFiscale)
	})

	t.Run("missing access", func(t *testing.T) {
		_, err := resultFromStorage(session.PortalCUP, storageSnapshot{Refresh: fakeJWT(`{"exp":1700000000}`)})
		require.ErrorIs(t, err, ErrTokensMissing)
	})

	t.Run("missing refresh", func(t *testing.T) {
		_, err := resultFromStorage(session.PortalCUP, storageSnapshot{Access: fakeJWT(`{"exp":1700000000}`)})
		require.ErrorIs(t, err, ErrTokensMissing)
	})

	t.Run("malformed access token", func(t *testing.T) {
		_, err := resultFromStorage(session.PortalCUP, storageSnapshot{Access: "not-a-jwt", Refresh: fakeJWT(`{"exp":1700000000}`)})
		require.Error(t, err)
	})
}

func TestTesseraFromURL(t *testing.T) {
	t.Run("param present", func(t *testing.T) {
		got, err := tesseraFromURL("https://prenota.sanita.toscana.it/sis-portale-prenotazione/verificaTs?numbersTeam=80380000000000000000")
		require.NoError(t, err)
		assert.Equal(t, "80380000000000000000", got)
	})

	t.Run("param missing", func(t *testing.T) {
		_, err := tesseraFromURL("https://prenota.sanita.toscana.it/sis-portale-prenotazione/verificaTs?x=1")
		require.ErrorIs(t, err, ErrNoTessera)
	})

	t.Run("param empty", func(t *testing.T) {
		_, err := tesseraFromURL("https://prenota.sanita.toscana.it/sis-portale-prenotazione/verificaTs?numbersTeam=")
		require.ErrorIs(t, err, ErrNoTessera)
	})

	t.Run("unparsable url", func(t *testing.T) {
		_, err := tesseraFromURL("http://[::1")
		require.ErrorIs(t, err, ErrNoTessera)
	})
}
