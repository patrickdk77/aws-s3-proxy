package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func basicReq(user, pass string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.SetBasicAuth(user, pass)
	return req
}

var (
	users = []string{"alice", "bob"}
	pass  = []string{"apw", "bpw"}
)

func TestBasicAuthEachUser(t *testing.T) {
	for i := range users {
		info := &ReqInfo{}
		assert.True(t, auth(basicReq(users[i], pass[i]),
			users, pass, info))
		assert.Equal(t, users[i], info.user)
	}
}

func TestBasicAuthWrongPassword(t *testing.T) {
	assert.False(t, auth(basicReq("alice", "nope"),
		users, pass, &ReqInfo{}))
}

func TestBasicAuthOtherUsersPassword(t *testing.T) {
	assert.False(t, auth(basicReq("alice", "bpw"),
		users, pass, &ReqInfo{}))
}

func TestBasicAuthUnknownUser(t *testing.T) {
	assert.False(t, auth(basicReq("mallory", "apw"),
		users, pass, &ReqInfo{}))
}

func TestBasicAuthPrefixNotEnough(t *testing.T) {
	assert.False(t, auth(basicReq("alic", "apw"),
		users, pass, &ReqInfo{}))
	assert.False(t, auth(basicReq("alice", "apwx"),
		users, pass, &ReqInfo{}))
}

func TestBasicAuthMoreUsersThanPasswords(t *testing.T) {
	short := []string{"apw"}
	assert.NotPanics(t, func() {
		assert.False(t, auth(basicReq("bob", "bpw"),
			users, short, &ReqInfo{}))
	})
	assert.True(t, auth(basicReq("alice", "apw"),
		users, short, &ReqInfo{}))
}

func TestBasicAuthMorePasswordsThanUsers(t *testing.T) {
	assert.False(t, auth(basicReq("", "bpw"),
		[]string{"alice"}, pass, &ReqInfo{}))
}

func jwtReq(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestJwtMalformedWithSecret(t *testing.T) {
	restoreConfig(t)
	config.Config.JwtSecretKey = "secret"
	config.Config.JwtUserField = "email"
	for _, tok := range []string{"garbage", "a.b", "a.b.c.d",
		"..", "x.y.z"} {
		req := jwtReq(tok)
		assert.NotPanics(t, func() {
			ok := isValidJwt(req, &ReqInfo{})
			assert.False(t, ok, tok)
		}, tok)
	}
}

func TestJwtMalformedUserFieldOnly(t *testing.T) {
	restoreConfig(t)
	config.Config.JwtSecretKey = ""
	config.Config.JwtUserField = "email"
	info := &ReqInfo{user: "-"}
	assert.NotPanics(t, func() {
		assert.True(t, isValidJwt(jwtReq("garbage"), info))
	})
	assert.Equal(t, "-", info.user)
}

func TestJwtMalformedCustomHeader(t *testing.T) {
	restoreConfig(t)
	config.Config.JwtSecretKey = "secret"
	config.Config.JwtHeader = "Cf-Access-Jwt-Assertion"
	req := httptest.NewRequest(http.MethodGet, sample, nil)
	req.Header.Set("Cf-Access-Jwt-Assertion", "not.a.jwt")
	assert.NotPanics(t, func() {
		assert.False(t, isValidJwt(req, &ReqInfo{}))
	})
}

func TestJwtValidSetsUser(t *testing.T) {
	restoreConfig(t)
	config.Config.JwtSecretKey = "secret"
	config.Config.JwtUserField = "email"
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{"email": "a@example.com"}).
		SignedString([]byte("secret"))
	info := &ReqInfo{}
	assert.True(t, isValidJwt(jwtReq(tok), info))
	assert.Equal(t, "a@example.com", info.user)
}

func TestJwtMalformedThroughWrapper(t *testing.T) {
	restoreConfig(t)
	config.Config.JwtSecretKey = "secret"
	rr := httptest.NewRecorder()
	called := false
	assert.NotPanics(t, func() {
		WrapHandler(func(http.ResponseWriter, *http.Request) {
			called = true
		}).ServeHTTP(rr, jwtReq("garbage"))
	})
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
