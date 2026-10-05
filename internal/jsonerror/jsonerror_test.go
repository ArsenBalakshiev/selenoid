package jsonerror

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestErrorString(t *testing.T) {
	se := InvalidArgument(errors.New("boom"))
	assert.Equal(t, "invalid argument: boom", se.Error())
}

func TestConstructors(t *testing.T) {
	err := errors.New("boom")
	assert.Equal(t, http.StatusBadRequest, InvalidArgument(err).Status)
	assert.Equal(t, "invalid argument", InvalidArgument(err).Name)
	assert.Equal(t, http.StatusNotFound, InvalidSessionID(err).Status)
	assert.Equal(t, "invalid session id", InvalidSessionID(err).Name)
	assert.Equal(t, http.StatusInternalServerError, SessionNotCreated(err).Status)
	assert.Equal(t, "session not created", SessionNotCreated(err).Name)
	assert.Equal(t, http.StatusInternalServerError, UnknownError(err).Status)
	assert.Equal(t, "unknown error", UnknownError(err).Name)
}

func TestEncode(t *testing.T) {
	w := httptest.NewRecorder()
	InvalidArgument(errors.New("boom")).Encode(w)

	resp := w.Result() //nolint:bodyclose // recorder body is not a real network body
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestEncodeBody(t *testing.T) {
	w := httptest.NewRecorder()
	UnknownError(errors.New("boom")).Encode(w)
	body, _ := io.ReadAll(w.Body)
	var data map[string]map[string]string
	assert.NoError(t, json.Unmarshal(body, &data))
	assert.Equal(t, "unknown error", data["value"]["error"])
	assert.Equal(t, "boom", data["value"]["message"])
}
