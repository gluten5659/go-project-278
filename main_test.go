package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func performGet(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	newRouter().ServeHTTP(recorder, request)

	return recorder
}

func TestPingRespondsWithPong(t *testing.T) {
	t.Parallel()

	response := performGet(t, "/ping").Result()
	defer func() {
		require.NoError(t, response.Body.Close())
	}()

	require.Equal(t, http.StatusOK, response.StatusCode)

	var body map[string]string

	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	assert.Equal(t, map[string]string{"message": "pong"}, body)
}

func TestUnknownRouteRespondsWithNotFound(t *testing.T) {
	t.Parallel()

	response := performGet(t, "/unknown").Result()
	defer func() {
		require.NoError(t, response.Body.Close())
	}()

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
}
