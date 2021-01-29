package http

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malusev998/jwt-go/v4"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

const sample = "http://example.com/foo"
var ri = &HTTPReqInfo{}

func TestWithoutAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	assert.False(t, auth(req, []string{"user"}, []string{"pass"}, ri))
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func TestWithoutBasic(t *testing.T) {
	username := "user"
	password := "pass"

	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Authorization", basicAuth(username, password))

	assert.False(t, auth(req, []string{username}, []string{password}, ri))
}

func TestAuthMatch(t *testing.T) {
	username := "user"
	password := "pass"

	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Authorization", "Basic "+basicAuth(username, password))

	assert.True(t, auth(req, []string{username}, []string{password}, ri))
}

func TestWithValidJWT(t *testing.T) {
	username := "user"
	password := "pass"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"password": password,
	})
	tokenString, _ := token.SignedString([]byte("secret"))
	c := config.Config
	c.JwtSecretKey = "secret"
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenString))

	assert.True(t, isValidJwt(req, ri))
}

func TestWithoutValidJWT(t *testing.T) {
	username := "user"
	password := "pass"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"password": password,
	})
	tokenString, _ := token.SignedString([]byte("secret"))
	c := config.Config
	c.JwtSecretKey = "foo"
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenString))

	assert.False(t, isValidJwt(req, ri))
}

func TestWithValidRemoteIPXForwardedFor(t *testing.T) {
	config.Config.ForwardedFor = "X-FORWARDED-FOR"
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("X-FORWARDED-FOR", "10.2.2.2")

	assert.Equal(t, getIP(req), "10.2.2.2")
}

func TestWithValidRemoteIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.RemoteAddr = "10.0.0.12:64564"

	assert.Equal(t, getIP(req), "10.0.0.12:64564")
	clientIP,clientPort,_ := net.SplitHostPort(getIP(req))
	assert.Equal(t,clientIP, "10.0.0.12")
	assert.Equal(t,clientPort, "64564")
}

func TestHeaderWithValue(t *testing.T) {
	expected := "test"

	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Test", expected)

	actual, found := header(req, "Test")

	assert.True(t, found)
	assert.Equal(t, expected, actual)
}

func TestHeaderWithoutValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	_, found := header(req, "Test")
	assert.False(t, found)
}

func TestSplitCsvLine(t *testing.T) {
	expected := 3

	lines := splitCsvLine("1,2,3")

	assert.Equal(t, expected, len(lines))
}

func TestTrimmedSplitCsvLine(t *testing.T) {
	expected := 3

	lines := splitCsvLine("1 , 2 ,3 ")

	assert.Equal(t, expected, len(lines))
	assert.Equal(t, "1", lines[0])
	assert.Equal(t, "2", lines[1])
	assert.Equal(t, "3", lines[2])
}
