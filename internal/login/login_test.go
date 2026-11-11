package login

import (
	"encoding/base64"
	"fmt"
	"net/url"
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
		got, err := ResultFromStorage(session.PortalFSE, tok, tok, "rssmra80a01h501u")
		require.NoError(t, err)
		assert.Equal(t, session.PortalFSE, got.Portal)
		assert.Equal(t, tok, got.Tokens.Access)
		assert.Equal(t, tok, got.Tokens.Refresh)
		assert.Equal(t, time.Unix(1700000000, 0), got.Tokens.AccessExpiresAt.Local())
		assert.Equal(t, time.Unix(1700000000, 0), got.Tokens.RefreshExpiresAt.Local())
		assert.Equal(t, "RSSMRA80A01H501U", got.Identity.CodiceFiscale)
	})

	t.Run("missing access", func(t *testing.T) {
		_, err := ResultFromStorage(session.PortalCUP, "", fakeJWT(`{"exp":1700000000}`), "")
		require.ErrorIs(t, err, ErrTokensMissing)
	})

	t.Run("missing refresh", func(t *testing.T) {
		_, err := ResultFromStorage(session.PortalCUP, fakeJWT(`{"exp":1700000000}`), "", "")
		require.ErrorIs(t, err, ErrTokensMissing)
	})

	t.Run("malformed access token", func(t *testing.T) {
		_, err := ResultFromStorage(session.PortalCUP, "not-a-jwt", fakeJWT(`{"exp":1700000000}`), "")
		require.Error(t, err)
	})
}

func TestValidate(t *testing.T) {
	now := time.Unix(1700000000, 0)
	build := func(portal session.Portal, iss, azp string, exp int64) Result {
		tok := fakeJWT(fmt.Sprintf(`{"iss":%q,"azp":%q,"exp":%d}`, iss, azp, exp))
		res, err := ResultFromStorage(portal, tok, tok, "RSSMRA80A01H501U")
		require.NoError(t, err)
		return res
	}
	future := now.Add(time.Hour).Unix()

	t.Run("valid cup", func(t *testing.T) {
		require.NoError(t, Validate(build(session.PortalCUP, realmIssuer, cupClientID, future), now))
	})

	t.Run("valid fse", func(t *testing.T) {
		require.NoError(t, Validate(build(session.PortalFSE, realmIssuer, fseClientID, future), now))
	})

	t.Run("wrong issuer", func(t *testing.T) {
		err := Validate(build(session.PortalCUP, "https://evil.example/realms/rt", cupClientID, future), now)
		require.ErrorIs(t, err, ErrTokenMismatch)
	})

	t.Run("fse token on cup portal", func(t *testing.T) {
		err := Validate(build(session.PortalCUP, realmIssuer, fseClientID, future), now)
		require.ErrorIs(t, err, ErrTokenMismatch)
	})

	t.Run("refresh token mismatch", func(t *testing.T) {
		res := build(session.PortalCUP, realmIssuer, cupClientID, future)
		res.Tokens.Refresh = fakeJWT(`{"iss":"x","azp":"y","exp":1}`)
		require.ErrorIs(t, Validate(res, now), ErrTokenMismatch)
	})

	t.Run("expired access token", func(t *testing.T) {
		err := Validate(build(session.PortalCUP, realmIssuer, cupClientID, now.Add(-time.Minute).Unix()), now)
		require.ErrorIs(t, err, ErrTokenExpired)
	})

	t.Run("malformed token", func(t *testing.T) {
		res := build(session.PortalCUP, realmIssuer, cupClientID, future)
		res.Tokens.Access = "not-a-jwt"
		require.Error(t, Validate(res, now))
	})

	t.Run("unknown portal", func(t *testing.T) {
		res := build(session.PortalCUP, realmIssuer, cupClientID, future)
		res.Portal = "xyz"
		require.ErrorIs(t, Validate(res, now), ErrUnknownPortal)
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

func TestLoginURLs(t *testing.T) {
	t.Run("CUP", func(t *testing.T) {
		u, err := url.Parse(cupLoginURL)
		require.NoError(t, err)
		q := u.Query()
		assert.Equal(t, "auth.regione.toscana.it", u.Host)
		assert.Equal(t, "/auth/realms/rt/protocol/openid-connect/auth", u.Path)
		assert.Equal(t, "code", q.Get("response_type"))
		assert.Equal(t, "r_toscan__portale_cup2_0", q.Get("client_id"))
		assert.Equal(t, "https://prenota.sanita.toscana.it", q.Get("redirect_uri"))
		assert.Equal(t, "default", q.Get("scope"))
	})

	t.Run("FSE", func(t *testing.T) {
		u, err := url.Parse(fseLoginURL)
		require.NoError(t, err)
		q := u.Query()
		assert.Equal(t, "auth.regione.toscana.it", u.Host)
		assert.Equal(t, "/auth/realms/rt/protocol/openid-connect/auth", u.Path)
		assert.Equal(t, "code", q.Get("response_type"))
		assert.Equal(t, "r_toscan__fse_web", q.Get("client_id"))
		assert.Equal(t, "https://fse.sanita.toscana.it/", q.Get("redirect_uri"))
		assert.Equal(t, "profile sdm-rs/cittadino sisorchestrator-rs/cittadino vaccini-rs/cittadino", q.Get("scope"))
	})
}
