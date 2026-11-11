package viewer

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func upstreamAddr(t *testing.T, h http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestHandler(t *testing.T) {
	t.Run("proxies static files with the prefix stripped", func(t *testing.T) {
		addr := upstreamAddr(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("path=" + r.URL.Path + " query=" + r.URL.RawQuery))
		}))
		srv := httptest.NewServer(Handler(addr))
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/viewer/vnc.html?autoconnect=true")
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })

		buf := make([]byte, 128)
		n, _ := resp.Body.Read(buf)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "path=/vnc.html query=autoconnect=true", string(buf[:n]))
	})

	t.Run("relays a websocket upgrade", func(t *testing.T) {
		var upstreamPath string
		addr := upstreamAddr(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upstreamPath = r.URL.Path
			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			_ = rw.Flush()
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			_, _ = rw.WriteString("echo:" + line)
			_ = rw.Flush()
		}))
		srv := httptest.NewServer(Handler(addr))
		t.Cleanup(srv.Close)

		conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		_, err = conn.Write([]byte("GET /viewer/websockify HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"))
		require.NoError(t, err)
		reader := bufio.NewReader(conn)
		status, err := reader.ReadString('\n')
		require.NoError(t, err)
		assert.Contains(t, status, "101")
		for {
			header, err := reader.ReadString('\n')
			require.NoError(t, err)
			if header == "\r\n" {
				break
			}
		}

		_, err = conn.Write([]byte("hello\n"))
		require.NoError(t, err)
		echoed, err := reader.ReadString('\n')
		require.NoError(t, err)
		assert.Equal(t, "echo:hello\n", echoed)
		assert.Equal(t, "/websockify", upstreamPath)
	})

	t.Run("returns 502 when the upstream is down", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().String()
		require.NoError(t, ln.Close())

		rec := httptest.NewRecorder()
		Handler(addr).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/viewer/vnc.html", nil))
		assert.Equal(t, http.StatusBadGateway, rec.Code)
	})
}
