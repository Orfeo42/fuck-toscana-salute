package facilities

import (
	"crypto/tls"
	"net/http"
	"time"
)

func ministryCipherSuites() []uint16 {
	var ids []uint16
	for _, suite := range tls.CipherSuites() {
		ids = append(ids, suite.ID)
	}
	return append(ids, tls.TLS_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_RSA_WITH_AES_256_GCM_SHA384)
}

func newMinistryClient(timeout time.Duration) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment, TLSHandshakeTimeout: 10 * time.Second}
	}
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, CipherSuites: ministryCipherSuites()}
	return &http.Client{Transport: transport, Timeout: timeout}
}
