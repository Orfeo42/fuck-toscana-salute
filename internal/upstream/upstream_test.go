package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payload struct {
	Name string `json:"name"`
}

func TestPostJSON(t *testing.T) {
	t.Run("sends body and headers and decodes response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var in payload
			assert.NoError(t, json.Unmarshal(raw, &in))
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "in", in.Name)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "fuck-toscana-salute", r.Header.Get("User-Agent"))
			assert.Equal(t, "tok", r.Header.Get("X-Custom"))
			_, err = w.Write([]byte(`{"name":"out"}`))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		header := http.Header{}
		header.Set("X-Custom", "tok")
		got, err := PostJSON[payload, payload](context.Background(), New(srv.Client()), srv.URL, header, payload{Name: "in"})
		require.NoError(t, err)
		assert.Equal(t, "out", got.Name)
	})

	t.Run("non-2xx returns StatusError without body in message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte("MARKER-body"))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		_, err := PostJSON[payload, payload](context.Background(), New(srv.Client()), srv.URL, nil, payload{})
		var statusErr *StatusError
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusBadRequest, statusErr.Status)
		assert.True(t, statusErr.BodyContains("MARKER"))
		assert.NotContains(t, err.Error(), "MARKER")
	})

	t.Run("invalid JSON response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("not json"))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		_, err := PostJSON[payload, payload](context.Background(), New(srv.Client()), srv.URL, nil, payload{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("unreachable server does not leak query string", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		target := srv.URL + "/path?secret=x"
		client := srv.Client()
		srv.Close()

		_, err := PostJSON[payload, payload](context.Background(), New(client), target, nil, payload{})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret")
		assert.Contains(t, err.Error(), "/path")
	})
}

func TestGetJSON(t *testing.T) {
	t.Run("success decodes response without content type", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Empty(t, r.Header.Get("Content-Type"))
			_, err := w.Write([]byte(`{"name":"ok"}`))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		got, err := GetJSON[payload](context.Background(), New(srv.Client()), srv.URL, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", got.Name)
	})

	t.Run("500 returns StatusError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := GetJSON[payload](context.Background(), New(srv.Client()), srv.URL, nil)
		var statusErr *StatusError
		require.True(t, errors.As(err, &statusErr))
		assert.Equal(t, http.StatusInternalServerError, statusErr.Status)
		assert.Equal(t, "upstream: unexpected status 500", err.Error())
	})
}

func TestStatusError(t *testing.T) {
	statusErr := &StatusError{Status: 400, body: []byte("hello marker world")}

	t.Run("BodyContains true", func(t *testing.T) {
		assert.True(t, statusErr.BodyContains("marker"))
	})

	t.Run("BodyContains false", func(t *testing.T) {
		assert.False(t, statusErr.BodyContains("absent"))
	})
}
