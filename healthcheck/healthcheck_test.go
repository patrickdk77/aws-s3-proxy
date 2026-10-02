package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func env(t *testing.T, host, port, ssl, path string) {
	t.Helper()
	t.Setenv("APP_HOST", host)
	t.Setenv("APP_PORT", port)
	t.Setenv("SSL_KEY_PATH", ssl)
	t.Setenv("HEALTHCHECKER_PATH", "")
	t.Setenv("HEALTHCHECK_PATH", path)
}

func serverEnv(t *testing.T, srv *httptest.Server, ssl string) {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	env(t, host, port, ssl, "/healthz")
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			code = http.StatusTeapot
		}
		w.WriteHeader(code)
	}
}

func TestHealthURLDefaults(t *testing.T) {
	env(t, "", "", "", "/healthz")
	assert.Equal(t, "http://localhost:80/healthz", healthURL())
}

func TestHealthURLPortAndTLS(t *testing.T) {
	env(t, "10.0.0.5", "8443", "/k.pem", "/hc")
	assert.Equal(t, "https://10.0.0.5:8443/hc", healthURL())
}

func TestHealthURLIPv6(t *testing.T) {
	env(t, "::1", "8080", "", "/healthz")
	assert.Equal(t, "http://[::1]:8080/healthz", healthURL())
}

func TestHealthURLCheckerPathWins(t *testing.T) {
	env(t, "", "8080", "", "/a")
	t.Setenv("HEALTHCHECKER_PATH", "/b")
	assert.Equal(t, "http://localhost:8080/b", healthURL())
}

func TestHealthURLNone(t *testing.T) {
	env(t, "", "", "", "")
	assert.Empty(t, healthURL())
	assert.Equal(t, 0, probe())
}

func TestProbeHealthy(t *testing.T) {
	srv := httptest.NewServer(status(http.StatusOK))
	defer srv.Close()
	serverEnv(t, srv, "")
	assert.Equal(t, 0, probe())
}

func TestProbeUnhealthy(t *testing.T) {
	srv := httptest.NewServer(status(http.StatusBadGateway))
	defer srv.Close()
	serverEnv(t, srv, "")
	assert.Equal(t, 2, probe())
}

func TestProbeSelfSignedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(status(http.StatusOK))
	defer srv.Close()
	serverEnv(t, srv, "/etc/key.pem")
	assert.Equal(t, 0, probe())
}

func TestProbeDown(t *testing.T) {
	srv := httptest.NewServer(status(http.StatusOK))
	serverEnv(t, srv, "")
	srv.Close()
	assert.Equal(t, 1, probe())
}

func TestProbePlainHTTPToTLSPort(t *testing.T) {
	srv := httptest.NewTLSServer(status(http.StatusOK))
	defer srv.Close()
	serverEnv(t, srv, "")
	assert.NotEqual(t, 0, probe())
}
