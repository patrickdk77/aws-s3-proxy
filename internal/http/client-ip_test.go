package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func restoreConfig(t *testing.T) {
	t.Helper()
	old := *config.Config
	t.Cleanup(func() { *config.Config = old })
}

func whitelisted(t *testing.T, header string,
	values ...string) bool {
	t.Helper()
	restoreConfig(t)
	_, lan, _ := net.ParseCIDR("10.0.0.0/8")
	config.Config.WhiteListIPRanges = []*net.IPNet{lan}
	config.Config.ForwardedFor = header
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.RemoteAddr = "192.0.2.1:4444"
	for _, v := range values {
		req.Header.Add(header, v)
	}
	ok := false
	WrapHandler(func(http.ResponseWriter, *http.Request) {
		ok = true
	}).ServeHTTP(httptest.NewRecorder(), req)
	return ok
}

func TestForwardedForRightmostAllowed(t *testing.T) {
	assert.True(t, whitelisted(t, "X-Forwarded-For",
		"203.0.113.5, 10.1.2.3"))
}

func TestForwardedForSpoofedLeftmostDenied(t *testing.T) {
	assert.False(t, whitelisted(t, "X-Forwarded-For",
		"10.1.2.3, 203.0.113.5"))
}

func TestForwardedForSecondHeaderLine(t *testing.T) {
	assert.False(t, whitelisted(t, "X-Forwarded-For",
		"10.1.2.3", "203.0.113.5"))
	assert.True(t, whitelisted(t, "X-Forwarded-For",
		"203.0.113.5", "10.1.2.3"))
}

func TestForwardedForSingleValueHeader(t *testing.T) {
	assert.True(t, whitelisted(t, "Cf-Connecting-Ip", "10.9.8.7"))
	assert.False(t, whitelisted(t, "Cf-Connecting-Ip",
		"203.0.113.9"))
}

func TestForwardedForTrailingEmpty(t *testing.T) {
	assert.True(t, whitelisted(t, "X-Forwarded-For",
		"10.1.2.3, "))
}

func TestForwardedForMissingUsesRemoteAddr(t *testing.T) {
	assert.False(t, whitelisted(t, "X-Forwarded-For"))
}

func TestForwardedForGarbageDenied(t *testing.T) {
	assert.False(t, whitelisted(t, "X-Forwarded-For",
		"not-an-ip"))
	assert.False(t, whitelisted(t, "X-Forwarded-For", ",,"))
}

func TestForwardedForIPv6(t *testing.T) {
	restoreConfig(t)
	config.Config.ForwardedFor = "X-Forwarded-For"
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("X-Forwarded-For", "10.1.2.3, 2001:db8::7")
	assert.Equal(t, "2001:db8::7", getIP(req))
}
