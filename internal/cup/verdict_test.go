package cup

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
)

const (
	bodySuccess = `{"message":{"code":"0","level":"SUCCESS","description":"Operazione conclusa con successo"},"response":{"esito":[]}}`
	bodyHandled = `{"message":{"code":"1","level":"ERROR","description":"0900A0000000001: Impossibile recuperare la prescrizione elettronica con codice 0900A0000000001: Errore generico: La prescrizione è già stata completamente gestita tramite prenotazioni o accettazioni, non risulta possibile procedere all'acquisizione"},"response":null}`
	bodyExpired = `{"message":{"code":"1","level":"ERROR","description":"0900A0000000001: Impossibile recuperare la prescrizione elettronica con codice 0900A0000000001: Visualizzazione non consentita - ricetta scaduta"},"response":null}`
	bodyOther   = `{"message":{"code":"1","level":"ERROR","description":"Servizio non disponibile"},"response":null}`
)

func respondWith(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}

func TestVerdict(t *testing.T) {
	identity := session.Identity{CodiceFiscale: testCF}

	t.Run("sends request and returns bookable with raw body", func(t *testing.T) {
		var method, path, auth, contentType, sent string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.Path
			auth, contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			sent = string(data)
			_, _ = w.Write([]byte(bodySuccess))
		}, identity)

		v, err := c.Verdict(context.Background(), "0900a0000000001")

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, method)
		assert.Equal(t, "/list/getPrescrizioneElettronicaSecured", path)
		assert.Equal(t, "Bearer tok", auth)
		assert.Equal(t, "application/json", contentType)
		assert.JSONEq(t, `{"cf":"`+testCF+`","nre":"`+testNRE+`"}`, sent)
		assert.Equal(t, VerdictBookable, v.Reason)
		assert.Equal(t, "Operazione conclusa con successo", v.Message)
		assert.Equal(t, http.StatusOK, v.HTTPStatus)
		assert.Equal(t, bodySuccess, string(v.Raw))
	})

	t.Run("handled", func(t *testing.T) {
		c := newTestClient(t, respondWith(bodyHandled), identity)
		v, err := c.Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, VerdictHandled, v.Reason)
		assert.Equal(t, "La prescrizione è già stata completamente gestita tramite prenotazioni o accettazioni, non risulta possibile procedere all'acquisizione", v.Message)
	})

	t.Run("expired", func(t *testing.T) {
		c := newTestClient(t, respondWith(bodyExpired), identity)
		v, err := c.Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, VerdictExpired, v.Reason)
		assert.Equal(t, "Visualizzazione non consentita - ricetta scaduta", v.Message)
	})

	t.Run("unknown error is other", func(t *testing.T) {
		c := newTestClient(t, respondWith(bodyOther), identity)
		v, err := c.Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, VerdictOther, v.Reason)
		assert.Equal(t, "Servizio non disponibile", v.Message)
	})

	t.Run("unknown level is other", func(t *testing.T) {
		c := newTestClient(t, respondWith(`{"message":{"level":"WARNING","description":"x"}}`), identity)
		v, err := c.Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, VerdictOther, v.Reason)
	})

	t.Run("no codice fiscale returns ErrNotLoggedIn", func(t *testing.T) {
		c := newTestClient(t, respondWith(bodySuccess), session.Identity{})
		_, err := c.Verdict(context.Background(), testNRE)
		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})

	t.Run("token error is wrapped", func(t *testing.T) {
		c := newTestClient(t, respondWith(bodySuccess), identity)
		c.tokens = &MockTokenSource{
			AccessTokenFunc: func(ctx context.Context, portal session.Portal) (string, error) {
				return "", assert.AnError
			},
			IdentityFunc: func() session.Identity { return identity },
		}
		_, err := c.Verdict(context.Background(), testNRE)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("http 500 returns error", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}, identity)
		_, err := c.Verdict(context.Background(), testNRE)
		require.Error(t, err)
	})

	t.Run("non-json body returns error", func(t *testing.T) {
		c := newTestClient(t, respondWith("<html>nope</html>"), identity)
		_, err := c.Verdict(context.Background(), testNRE)
		require.Error(t, err)
	})
}
