package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func corsSetup(t *testing.T) {
	t.Helper()
	restoreConfig(t)
	config.Config.CorsAllowOrigin = "https://app.example.com"
	config.Config.CorsAllowMethods = "GET, HEAD"
	config.Config.CorsAllowHeaders = "Authorization, Range"
	config.Config.CorsMaxAge = 600
}

func corsDo(method string,
	preflight bool) (*httptest.ResponseRecorder, bool) {
	req := httptest.NewRequest(method, sample, nil)
	req.RemoteAddr = "10.1.1.1:5555"
	req.Header.Set("Origin", "https://app.example.com")
	if preflight {
		req.Header.Set(preflightHeader, "GET")
	}
	called := false
	rr := httptest.NewRecorder()
	WrapHandler(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}).ServeHTTP(rr, req)
	return rr, called
}

func TestCorsPreflight(t *testing.T) {
	corsSetup(t)
	rr, called := corsDo(http.MethodOptions, true)
	assert.False(t, called)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	h := rr.Header()
	assert.Equal(t, "https://app.example.com",
		h.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, HEAD",
		h.Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Authorization, Range",
		h.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "600", h.Get("Access-Control-Max-Age"))
}

func TestCorsPreflightSkipsBasicAuth(t *testing.T) {
	corsSetup(t)
	config.Config.BasicAuthUser = []string{"u"}
	config.Config.BasicAuthPass = []string{"p"}
	rr, _ := corsDo(http.MethodOptions, true)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestCorsPreflightSkipsJwt(t *testing.T) {
	corsSetup(t)
	config.Config.JwtSecretKey = "secret"
	rr, _ := corsDo(http.MethodOptions, true)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestCorsPreflightStillWhitelisted(t *testing.T) {
	corsSetup(t)
	_, other, _ := net.ParseCIDR("192.0.2.0/24")
	config.Config.WhiteListIPRanges = []*net.IPNet{other}
	rr, called := corsDo(http.MethodOptions, true)
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestCorsPlainOptionsReachesHandler(t *testing.T) {
	corsSetup(t)
	_, called := corsDo(http.MethodOptions, false)
	assert.True(t, called)
}

func TestCorsDisabledPreflightReachesHandler(t *testing.T) {
	restoreConfig(t)
	config.Config.CorsAllowOrigin = ""
	rr, called := corsDo(http.MethodOptions, true)
	assert.True(t, called)
	assert.Empty(t,
		rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsGetStillAuthenticated(t *testing.T) {
	corsSetup(t)
	config.Config.BasicAuthUser = []string{"u"}
	config.Config.BasicAuthPass = []string{"p"}
	rr, called := corsDo(http.MethodGet, true)
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "https://app.example.com",
		rr.Header().Get("Access-Control-Allow-Origin"))
}
