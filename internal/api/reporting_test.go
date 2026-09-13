package api_test

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reportedError struct {
	path    string
	message string
}

func performReportedRequest(
	t *testing.T,
	queries db.Querier,
	method string,
	path string,
	body string,
) (*httptest.ResponseRecorder, []reportedError) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var reports []reportedError

	recordReport := func(request *http.Request, err error) {
		reports = append(reports, reportedError{path: request.URL.Path, message: err.Error()})
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))

	api.NewRouter(api.Config{
		Queries:        queries,
		Database:       stubDatabase{},
		AllowedOrigins: []string{allowedOrigin},
		BaseURL:        baseURL,
		ReportError:    recordReport,
	}).ServeHTTP(recorder, request)

	return recorder, reports
}

func TestReportsServerErrors(t *testing.T) {
	t.Parallel()

	failingCount := func(context.Context) (int64, error) {
		return 0, errQueryFailed
	}

	panickingCount := func(context.Context) (int64, error) {
		panic("boom")
	}

	emptyPage := func(context.Context, db.GetLinksParams) ([]db.Link, error) {
		return nil, nil
	}

	testCases := []struct {
		name        string
		queries     stubQuerier
		method      string
		path        string
		body        string
		wantStatus  int
		wantReports []reportedError
	}{
		{
			name:       "reports a query that failed",
			queries:    stubQuerier{countLinks: failingCount},
			method:     http.MethodGet,
			path:       collectionPath,
			wantStatus: http.StatusInternalServerError,
			wantReports: []reportedError{
				{path: collectionPath, message: queryFailedMessage},
			},
		},
		{
			name:       "reports a panic and still answers with a server error",
			queries:    stubQuerier{countLinks: panickingCount},
			method:     http.MethodGet,
			path:       collectionPath,
			wantStatus: http.StatusInternalServerError,
			wantReports: []reportedError{
				{path: collectionPath, message: "panic: boom"},
			},
		},
		{
			name:       "keeps a validation failure out of the reports",
			queries:    stubQuerier{},
			method:     http.MethodPost,
			path:       collectionPath,
			body:       invalidURLBody,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "keeps a malformed body out of the reports",
			queries:    stubQuerier{},
			method:     http.MethodPost,
			path:       collectionPath,
			body:       malformedBody,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "keeps a missing link out of the reports",
			queries: stubQuerier{
				getLinkByID: func(context.Context, int64) (db.Link, error) {
					return db.Link{}, sql.ErrNoRows
				},
			},
			method:     http.MethodGet,
			path:       linkPath,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "keeps a successful request out of the reports",
			queries: stubQuerier{
				countLinks: countedRecords(0),
				getLinks:   emptyPage,
			},
			method:     http.MethodGet,
			path:       collectionPath,
			wantStatus: http.StatusOK,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder, reports := performReportedRequest(
				t,
				testCase.queries,
				testCase.method,
				testCase.path,
				testCase.body,
			)

			require.Equal(t, testCase.wantStatus, recorder.Code)
			assert.Equal(t, testCase.wantReports, reports)
		})
	}
}
