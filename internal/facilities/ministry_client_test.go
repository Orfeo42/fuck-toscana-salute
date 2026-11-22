package facilities

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubTransport struct {
	hits int
}

func (st *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	st.hits++
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
}

func TestMinistryClientTLSConfig(t *testing.T) {
	client := newMinistryClient(7 * time.Second)
	require.Equal(t, 7*time.Second, client.Timeout)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	cfg := transport.TLSClientConfig
	require.False(t, cfg.InsecureSkipVerify)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Contains(t, cfg.CipherSuites, tls.TLS_RSA_WITH_AES_128_GCM_SHA256)
	require.Contains(t, cfg.CipherSuites, tls.TLS_RSA_WITH_AES_256_GCM_SHA384)
	for _, suite := range tls.CipherSuites() {
		require.Contains(t, cfg.CipherSuites, suite.ID)
	}
}

func TestMinistryClientRSAKeyExchange(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("ok"))
		require.NoError(t, err)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384}}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())

	plain := newMinistryClient(5 * time.Second)
	plainTransport, ok := plain.Transport.(*http.Transport)
	require.True(t, ok)
	plainTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	resp, err := plain.Get(server.URL)
	if err == nil {
		require.NoError(t, resp.Body.Close())
	}
	require.Error(t, err)

	client := newMinistryClient(5 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	transport.TLSClientConfig.RootCAs = roots
	resp, err = client.Get(server.URL)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "ok", string(body))
}

func TestFetchHostRouting(t *testing.T) {
	cases := map[string]struct {
		url          string
		ministryHits int
		defaultHits  int
	}{
		"ministry host":  {url: "https://" + ministryHost + "/a.csv", ministryHits: 1},
		"other host":     {url: "https://example.org/a.csv", defaultHits: 1},
		"regione host":   {url: "https://dati.toscana.it/a.csv", defaultHits: 1},
		"lookalike host": {url: "https://" + ministryHost + ".evil.example/a.csv", defaultHits: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ministry := &stubTransport{}
			other := &stubTransport{}
			s := &Service{client: &http.Client{Transport: other}, ministryClient: &http.Client{Transport: ministry}}
			_, err := s.fetch(t.Context(), tc.url)
			require.NoError(t, err)
			require.Equal(t, tc.ministryHits, ministry.hits)
			require.Equal(t, tc.defaultHits, other.hits)
		})
	}
}
