package api_test

import (
	"code/internal/api"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCheckHealth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		pingError  error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "answers pong when the database replies",
			wantStatus: http.StatusOK,
			wantBody:   `{"message": "pong"}`,
		},
		{
			name:       "answers service unavailable when the database is down",
			pingError:  errQueryFailed,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   unavailableBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ping", nil)

			api.NewRouter(api.Config{
				Queries:        stubQuerier{},
				Database:       stubDatabase{pingError: testCase.pingError},
				AllowedOrigins: []string{allowedOrigin},
				ReportError:    discardErrorReports,
			}).ServeHTTP(recorder, request)

			require.Equal(t, testCase.wantStatus, recorder.Code)
			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
		})
	}
}
