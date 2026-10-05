package info

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestRequestInfoBasicAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	user, remote := RequestInfo(req)
	assert.Equal(t, "unknown", user)
	assert.Equal(t, "10.0.0.1", remote)
}

func TestRequestInfoRemoteAddrOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	_, remote := RequestInfo(req)
	assert.Equal(t, "10.0.0.1", remote)
}

func TestRequestInfoRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	user, remote := RequestInfo(req)
	assert.Equal(t, "unknown", user)
	assert.Equal(t, "10.0.0.1", remote)
}

func TestRequestInfoXForwardedForWins(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	user, remote := RequestInfo(req)
	assert.Equal(t, "unknown", user)
	assert.Equal(t, "1.2.3.4", remote)
}

func TestRequestInfoAuthenticatedUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:password")))
	user, remote := RequestInfo(req)
	assert.Equal(t, "user", user)
	assert.Equal(t, "10.0.0.1", remote)
}

func TestRequestInfoMalformedAuthHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic !!!invalid-base64!!!")
	req.RemoteAddr = "10.0.0.1:1234"
	user, remote := RequestInfo(req)
	assert.Equal(t, "unknown", user)
	assert.Equal(t, "10.0.0.1", remote)
}

func TestSecondsSince(t *testing.T) {
	past := time.Now().Add(-5 * time.Second)
	seconds := SecondsSince(past)
	assert.GreaterOrEqual(t, seconds, 4.0)
	assert.LessOrEqual(t, seconds, 6.0)
}
