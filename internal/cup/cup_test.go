package cup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

const (
	testCF      = "RSSMRA80A01H501U"
	testNRE     = "0900A0000000001"
	testTessera = "80380000000000000000"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, identity session.Identity) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	tokens := &MockTokenSource{
		AccessTokenFunc: func(ctx context.Context, portal session.Portal) (string, error) {
			return "tok", nil
		},
		IdentityFunc: func() session.Identity { return identity },
	}
	c := New(upstream.New(srv.Client()), tokens)
	c.cupBe = srv.URL + "/be"
	c.cupList = srv.URL + "/list"
	c.now = func() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC) }
	return c
}

func TestNew(t *testing.T) {
	t.Run("sets default endpoints, location and clock", func(t *testing.T) {
		c := New(upstream.New(http.DefaultClient), &MockTokenSource{})
		assert.Equal(t, defaultCupBe, c.cupBe)
		assert.Equal(t, defaultCupList, c.cupList)
		assert.Equal(t, "Europe/Rome", c.loc.String())
		assert.NotNil(t, c.now)
	})
}

func TestPrescriptions(t *testing.T) {
	identity := session.Identity{CodiceFiscale: testCF}

	t.Run("paginates and maps", func(t *testing.T) {
		var pages []int
		var auth string
		var first listRequest
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/list/nre", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			auth = r.Header.Get("Authorization")
			var req listRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			if len(pages) == 0 {
				first = req
			}
			pages = append(pages, req.Page)
			if req.Page == 0 {
				_, err := w.Write([]byte(`{"codiceEsito":"000","estrazione":0,"infoRicetta":[{"nre":"` + testNRE + `","dataPrescrizione":"2026-10-01","tipoRicetta":"SSN","stato":"30","jsonPrescrizione":"{\"listaDettagli\":[{\"descrizioneProdPrest\":\"VISITA\"},{\"descrizioneProdPrest\":\"\"},{\"descrizioneProdPrest\":\"ECO\"}]}"}]}`))
				require.NoError(t, err)
				return
			}
			_, err := w.Write([]byte(`{"codiceEsito":"000","estrazione":1,"infoRicetta":[{"nre":"0900A0000000002","dataPrescrizione":"bad","tipoRicetta":"X","stato":"zz","jsonPrescrizione":"not json"}]}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Prescriptions(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []int{0, 1}, pages)
		assert.Equal(t, "Bearer tok", auth)
		assert.Equal(t, "2012-01-01", first.DataInizio)
		assert.Equal(t, "2026-11-12", first.DataFine)
		assert.Equal(t, 10, first.Size)
		assert.Equal(t, testCF, first.CF)
		require.Len(t, got, 2)
		assert.Equal(t, testNRE, got[0].NRE)
		assert.Equal(t, 2026, got[0].PrescribedOn.Year())
		assert.Equal(t, "SSN", got[0].Kind)
		assert.Equal(t, StatusBooked, got[0].Status)
		assert.Equal(t, []string{"VISITA", "ECO"}, got[0].Items)
		assert.True(t, got[1].PrescribedOn.IsZero())
		assert.Equal(t, Status(0), got[1].Status)
		assert.Nil(t, got[1].Items)
	})

	t.Run("outcome code other than 000 is an error without description", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"codiceEsito":"999","descrizioneEsito":"secret detail"}`))
			require.NoError(t, err)
		}, identity)

		_, err := c.Prescriptions(context.Background())

		require.ErrorIs(t, err, ErrUpstreamOutcome)
		assert.Contains(t, err.Error(), "999")
		assert.NotContains(t, err.Error(), "secret detail")
	})

	t.Run("stops at the page cap", func(t *testing.T) {
		calls := 0
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			_, err := w.Write([]byte(`{"codiceEsito":"000","estrazione":0,"infoRicetta":[]}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Prescriptions(context.Background())

		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Equal(t, listMaxPages, calls)
	})

	t.Run("token error is propagated", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no request expected")
		}, identity)
		c.tokens = &MockTokenSource{
			AccessTokenFunc: func(ctx context.Context, portal session.Portal) (string, error) {
				return "", session.ErrSessionExpired
			},
			IdentityFunc: func() session.Identity { return identity },
		}

		_, err := c.Prescriptions(context.Background())

		require.ErrorIs(t, err, session.ErrSessionExpired)
	})

	t.Run("empty codice fiscale is not logged in", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no request expected")
		}, session.Identity{})

		_, err := c.Prescriptions(context.Background())

		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})
}

func TestAppointments(t *testing.T) {
	identity := session.Identity{CodiceFiscale: testCF, TesseraSanitaria: testTessera}

	t.Run("sends query and headers and maps", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/be/prescrizione", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, testTessera, r.URL.Query().Get("numbersTeam"))
			assert.Equal(t, "3", r.URL.Query().Get("accessType"))
			assert.Equal(t, testNRE, r.URL.Query().Get("accessCode"))
			assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
			assert.Equal(t, testCF, r.Header.Get("cf"))
			_, err := w.Write([]byte(`{"response":{"listaPrescrizioni":{"prescrizione":[{"listaAppuntamenti":{"appuntamento":[{"idAppuntamento":"77","statoAppuntamento":"PRENOTATO","dataAppuntamento":"20261215","oraAppuntamento":"09:30","unita":{"descUnita":"Desc","nomeUnita":"Name","indirizzoUnita":"Via Roma 1","nomeComuneUnita":"Firenze","provinciaUnita":"FI"}}]}}]}}}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Appointments(context.Background(), "0900a0000000001")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "77", got[0].ID)
		assert.Equal(t, testNRE, got[0].NRE)
		assert.Equal(t, "PRENOTATO", got[0].Status)
		assert.Equal(t, time.Date(2026, 12, 15, 9, 30, 0, 0, c.loc), got[0].At)
		assert.Equal(t, Unit{Name: "Name", Description: "Desc", Address: "Via Roma 1", City: "Firenze", Province: "FI"}, got[0].Unit)
	})

	t.Run("500 with no booking marker is empty", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "java.lang.Exception: Non esistono richieste per il paziente", http.StatusInternalServerError)
		}, identity)

		got, err := c.Appointments(context.Background(), testNRE)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("500 without marker is an error", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}, identity)

		_, err := c.Appointments(context.Background(), testNRE)

		require.Error(t, err)
	})

	t.Run("missing tessera", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no request expected")
		}, session.Identity{CodiceFiscale: testCF})

		_, err := c.Appointments(context.Background(), testNRE)

		require.ErrorIs(t, err, ErrGestioneNotReady)
	})

	t.Run("missing codice fiscale", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no request expected")
		}, session.Identity{TesseraSanitaria: testTessera})

		_, err := c.Appointments(context.Background(), testNRE)

		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})

	t.Run("null containers give empty result", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"response":{"listaPrescrizioni":{"prescrizione":[{"listaAppuntamenti":null}]}}}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Appointments(context.Background(), testNRE)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("null response gives empty result", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"response":null}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Appointments(context.Background(), testNRE)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("bad date keeps the appointment with zero time", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"response":{"listaPrescrizioni":{"prescrizione":[{"listaAppuntamenti":{"appuntamento":[{"idAppuntamento":"1","dataAppuntamento":"garbage","oraAppuntamento":"0930"}]}}]}}}`))
			require.NoError(t, err)
		}, identity)

		got, err := c.Appointments(context.Background(), testNRE)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.True(t, got[0].At.IsZero())
	})
}

func TestStatusLabel(t *testing.T) {
	t.Run("prescribed", func(t *testing.T) { assert.Equal(t, "Prescritta", StatusPrescribed.Label()) })
	t.Run("booked", func(t *testing.T) { assert.Equal(t, "Prenotata", StatusBooked.Label()) })
	t.Run("cancelled", func(t *testing.T) { assert.Equal(t, "Annullata", StatusCancelled.Label()) })
	t.Run("taken in charge", func(t *testing.T) { assert.Equal(t, "Presa in carico", StatusTakenInCharge.Label()) })
	t.Run("suspended", func(t *testing.T) { assert.Equal(t, "Sospesa", StatusSuspended.Label()) })
	t.Run("partially done", func(t *testing.T) { assert.Equal(t, "Erogata parziale", StatusPartiallyDone.Label()) })
	t.Run("done", func(t *testing.T) { assert.Equal(t, "Erogata", StatusDone.Label()) })
	t.Run("done after cancel", func(t *testing.T) {
		assert.Equal(t, "Erogata dopo annullamento", StatusDoneAfterCancel.Label())
	})
	t.Run("unknown", func(t *testing.T) { assert.Equal(t, "Sconosciuto", Status(99).Label()) })
}
