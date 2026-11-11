package fse

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type recordedRequest struct {
	Path   string
	Auth   string
	Method string
	Body   string
}

func newTokens(token string, err error) *MockTokenSource {
	return &MockTokenSource{
		AccessTokenFunc: func(ctx context.Context, portal session.Portal) (string, error) {
			return token, err
		},
	}
}

func newTestClient(t *testing.T, tokens TokenSource, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(upstream.New(srv.Client()), tokens)
	c.visitBase = srv.URL + "/visite"
	c.drugBase = srv.URL + "/farmaci"
	return c
}

func recordInto(t *testing.T, rec *[]recordedRequest, respond http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*rec = append(*rec, recordedRequest{
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Method: r.Method,
			Body:   string(body),
		})
		respond(w, r)
	}
}

func TestNew(t *testing.T) {
	t.Run("sets default bases and Rome location", func(t *testing.T) {
		c := New(upstream.New(http.DefaultClient), newTokens("t", nil))

		assert.Equal(t, "https://apit.sanita.toscana.it/C06/fse-ricette-visite/v1", c.visitBase)
		assert.Equal(t, "https://apit.sanita.toscana.it/C06/fse-ricette-farmaci/v1", c.drugBase)
		assert.Equal(t, "Europe/Rome", c.loc.String())
	})
}

func TestPrescriptions(t *testing.T) {
	t.Run("merges visits then drugs with kind and mapped fields", func(t *testing.T) {
		var rec []recordedRequest
		c := newTestClient(t, newTokens("tok", nil), recordInto(t, &rec, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/visite/listanew" {
				_, err := w.Write([]byte(`{"content":[{"id":"v1","nre":"NRE1","description":"visit one","stato":"8","dataPrestazione":"15/03/2024","actInternalIdAbstract":"REF1_X"}],"totalElements":1,"last":true}`))
				require.NoError(t, err)
				return
			}
			_, err := w.Write([]byte(`{"content":[{"id":"d1","nre":"NRE2","description":"drug one","stato":"bad","dataPrestazione":"bad","actInternalIdAbstract":"REF2_PREFARM"}],"totalElements":1,"last":true}`))
			require.NoError(t, err)
		}))

		got, err := c.Prescriptions(context.Background())

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, Prescription{
			ID:          "v1",
			NRE:         "NRE1",
			Description: "visit one",
			Kind:        KindVisit,
			Status:      8,
			Date:        time.Date(2024, 3, 15, 0, 0, 0, 0, c.loc),
			DocumentRef: "REF1_X",
		}, got[0])
		assert.Equal(t, Prescription{
			ID:          "d1",
			NRE:         "NRE2",
			Description: "drug one",
			Kind:        KindDrug,
			DocumentRef: "REF2_PREFARM",
		}, got[1])
	})

	t.Run("sends bearer header and empty json body to both lists", func(t *testing.T) {
		var rec []recordedRequest
		c := newTestClient(t, newTokens("tok", nil), recordInto(t, &rec, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"content":[],"totalElements":0,"last":true}`))
			require.NoError(t, err)
		}))

		_, err := c.Prescriptions(context.Background())

		require.NoError(t, err)
		require.Len(t, rec, 2)
		assert.Equal(t, "/visite/listanew", rec[0].Path)
		assert.Equal(t, "/farmaci/listanew", rec[1].Path)
		assert.Equal(t, http.MethodPost, rec[0].Method)
		assert.Equal(t, "Bearer tok", rec[0].Auth)
		assert.Equal(t, "Bearer tok", rec[1].Auth)
		assert.JSONEq(t, `{}`, rec[0].Body)
		assert.JSONEq(t, `{}`, rec[1].Body)
	})

	t.Run("error from one list is propagated", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/farmaci/listanew" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, err := w.Write([]byte(`{"content":[]}`))
			require.NoError(t, err)
		})

		got, err := c.Prescriptions(context.Background())

		require.Error(t, err)
		assert.Nil(t, got)
		var statusErr *upstream.StatusError
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusBadGateway, statusErr.Status)
	})

	t.Run("token error is propagated", func(t *testing.T) {
		c := newTestClient(t, newTokens("", session.ErrNotLoggedIn), func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("upstream must not be called without a token")
		})

		got, err := c.Prescriptions(context.Background())

		require.Error(t, err)
		assert.Nil(t, got)
		assert.True(t, errors.Is(err, session.ErrNotLoggedIn))
	})
}

func TestPrescriptionPDF(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("%PDF-fake"))

	t.Run("visit ref hits visit base with nre and null cf fields", func(t *testing.T) {
		var rec []recordedRequest
		c := newTestClient(t, newTokens("tok", nil), recordInto(t, &rec, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"other","valoreCampo":"x"},{"nomeCampo":"ricettaBase64","valoreCampo":"` + encoded + `"}]}}}`))
			require.NoError(t, err)
		}))

		got, err := c.PrescriptionPDF(context.Background(), "0900A0000000001_X")

		require.NoError(t, err)
		assert.Equal(t, []byte("%PDF-fake"), got)
		require.Len(t, rec, 1)
		assert.Equal(t, "/visite/nre/download", rec[0].Path)
		assert.Equal(t, "Bearer tok", rec[0].Auth)
		assert.Contains(t, rec[0].Body, `"cfDelegante":null`)
		assert.Contains(t, rec[0].Body, `"cfAssistito":null`)
		assert.JSONEq(t, `{"cfDelegante":null,"nre":"0900A0000000001","cfAssistito":null}`, rec[0].Body)
	})

	t.Run("ref without underscore uses whole string as nre", func(t *testing.T) {
		var rec []recordedRequest
		c := newTestClient(t, newTokens("tok", nil), recordInto(t, &rec, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"ricettaBase64","valoreCampo":"` + encoded + `"}]}}}`))
			require.NoError(t, err)
		}))

		_, err := c.PrescriptionPDF(context.Background(), "0900A0000000002")

		require.NoError(t, err)
		assert.JSONEq(t, `{"cfDelegante":null,"nre":"0900A0000000002","cfAssistito":null}`, rec[0].Body)
	})

	t.Run("prefarm ref hits drug base", func(t *testing.T) {
		var rec []recordedRequest
		c := newTestClient(t, newTokens("tok", nil), recordInto(t, &rec, func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"ricettaBase64","valoreCampo":"` + encoded + `"}]}}}`))
			require.NoError(t, err)
		}))

		_, err := c.PrescriptionPDF(context.Background(), "0900A0000000003_PREFARM")

		require.NoError(t, err)
		assert.Equal(t, "/farmaci/nre/download", rec[0].Path)
		assert.JSONEq(t, `{"cfDelegante":null,"nre":"0900A0000000003","cfAssistito":null}`, rec[0].Body)
	})

	t.Run("accepts refertoCodificatoBase64", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"refertoCodificatoBase64","valoreCampo":"` + encoded + `"}]}}}`))
			require.NoError(t, err)
		})

		got, err := c.PrescriptionPDF(context.Background(), "0900A0000000001_X")

		require.NoError(t, err)
		assert.Equal(t, []byte("%PDF-fake"), got)
	})

	t.Run("missing field returns ErrNoDocument", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"other","valoreCampo":"secret-value"}]}}}`))
			require.NoError(t, err)
		})

		got, err := c.PrescriptionPDF(context.Background(), "0900A0000000001_X")

		require.ErrorIs(t, err, ErrNoDocument)
		assert.ErrorContains(t, err, "fields [other]")
		assert.NotContains(t, err.Error(), "secret-value")
		assert.Nil(t, got)
	})

	t.Run("bad base64 returns error", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"risposta":{"contenutoRisposta":{"datoSingolo":[{"nomeCampo":"ricettaBase64","valoreCampo":"!!!not-base64"}]}}}`))
			require.NoError(t, err)
		})

		got, err := c.PrescriptionPDF(context.Background(), "0900A0000000001_X")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNoDocument)
		assert.Nil(t, got)
	})

	t.Run("empty ref returns ErrNoDocument", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("upstream must not be called for an empty ref")
		})

		got, err := c.PrescriptionPDF(context.Background(), "")

		require.ErrorIs(t, err, ErrNoDocument)
		assert.Nil(t, got)
	})

	t.Run("token error is propagated", func(t *testing.T) {
		c := newTestClient(t, newTokens("", session.ErrSessionExpired), func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("upstream must not be called without a token")
		})

		got, err := c.PrescriptionPDF(context.Background(), "0900A0000000001_X")

		require.ErrorIs(t, err, session.ErrSessionExpired)
		assert.Nil(t, got)
	})
}
