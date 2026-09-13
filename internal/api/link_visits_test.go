package api_test

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	visitsPath   = "/api/link_visits"
	redirectPath = "/r/example"

	visitUserAgent = "curl/8.5.0"
	visitReferer   = "https://news.example/post"
	remoteIP       = "192.0.2.1"
	cloudflareIP   = "203.0.113.7"

	cloudflareHeader = "CF-Connecting-IP"
)

func newVisit(visitID int64) db.LinkVisit {
	return db.LinkVisit{
		ID:        visitID,
		LinkID:    1,
		Ip:        remoteIP,
		UserAgent: visitUserAgent,
		Referer:   visitReferer,
		Status:    http.StatusFound,
		CreatedAt: time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC),
	}
}

func visitJSON(visit db.LinkVisit) string {
	return fmt.Sprintf(
		`{"id": %d, "link_id": %d, "ip": %q, "user_agent": %q,`+
			` "referer": %q, "status": %d, "created_at": %q}`,
		visit.ID,
		visit.LinkID,
		visit.Ip,
		visit.UserAgent,
		visit.Referer,
		visit.Status,
		visit.CreatedAt.Format(time.RFC3339),
	)
}

func recordedVisit(clientIP string) db.CreateLinkVisitParams {
	return db.CreateLinkVisitParams{
		LinkID:    1,
		Ip:        clientIP,
		UserAgent: visitUserAgent,
		Referer:   visitReferer,
		Status:    http.StatusFound,
	}
}

func performRedirect(
	t *testing.T,
	queries db.Querier,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, redirectPath, nil)

	request.Header.Set("User-Agent", visitUserAgent)
	request.Header.Set("Referer", visitReferer)

	for name, value := range headers {
		request.Header.Set(name, value)
	}

	api.NewRouter(queries, []string{allowedOrigin}, discardErrorReports).ServeHTTP(recorder, request)

	return recorder
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	storedLink := func(context.Context, string) (db.Link, error) {
		return newLink(1), nil
	}

	testCases := []struct {
		name               string
		headers            map[string]string
		getLinkByShortName func(ctx context.Context, shortName string) (db.Link, error)
		createError        error
		wantStatus         int
		wantLocation       string
		wantVisitRecorded  bool
		wantVisit          db.CreateLinkVisitParams
	}{
		{
			name:               "redirects to the original url and records the visit",
			getLinkByShortName: storedLink,
			wantStatus:         http.StatusFound,
			wantLocation:       originalURL,
			wantVisitRecorded:  true,
			wantVisit:          recordedVisit(remoteIP),
		},
		{
			name:               "takes the client address from the cloudflare header",
			headers:            map[string]string{cloudflareHeader: cloudflareIP},
			getLinkByShortName: storedLink,
			wantStatus:         http.StatusFound,
			wantLocation:       originalURL,
			wantVisitRecorded:  true,
			wantVisit:          recordedVisit(cloudflareIP),
		},
		{
			name: "returns not found when the short name is unknown",
			getLinkByShortName: func(context.Context, string) (db.Link, error) {
				return db.Link{}, sql.ErrNoRows
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "returns internal server error when the lookup fails",
			getLinkByShortName: func(context.Context, string) (db.Link, error) {
				return db.Link{}, errQueryFailed
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:               "returns internal server error when recording the visit fails",
			getLinkByShortName: storedLink,
			createError:        errQueryFailed,
			wantStatus:         http.StatusInternalServerError,
			wantVisitRecorded:  true,
			wantVisit:          recordedVisit(remoteIP),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			visitRecorded := false
			receivedShortName := ""

			var receivedVisit db.CreateLinkVisitParams

			queries := stubQuerier{
				getLinkByShortName: func(
					ctx context.Context,
					name string,
				) (db.Link, error) {
					receivedShortName = name

					return testCase.getLinkByShortName(ctx, name)
				},
				createLinkVisit: func(
					_ context.Context,
					parameters db.CreateLinkVisitParams,
				) (db.LinkVisit, error) {
					visitRecorded = true
					receivedVisit = parameters

					return newVisit(1), testCase.createError
				},
			}

			recorder := performRedirect(t, queries, testCase.headers)

			require.Equal(t, testCase.wantStatus, recorder.Code)
			assert.Equal(t, testCase.wantLocation, recorder.Header().Get("Location"))
			assert.Equal(t, shortName, receivedShortName)
			require.Equal(t, testCase.wantVisitRecorded, visitRecorded)

			if testCase.wantVisitRecorded {
				assert.Equal(t, testCase.wantVisit, receivedVisit)
			}
		})
	}
}

func TestIndexLinkVisits(t *testing.T) {
	t.Parallel()

	storedVisits := func(context.Context, db.GetLinkVisitsParams) ([]db.LinkVisit, error) {
		return []db.LinkVisit{newVisit(1), newVisit(2)}, nil
	}

	storedVisitsJSON := "[" + visitJSON(newVisit(1)) + "," + visitJSON(newVisit(2)) + "]"

	testCases := []indexCase[db.GetLinkVisitsParams, db.LinkVisit]{
		{
			name:            "returns stored visits",
			countRecords:    countedRecords(2),
			listRecords:     storedVisits,
			wantQueryCalled: true,
			wantParameters: db.GetLinkVisitsParams{
				PageOffset: 0,
				PageSize:   2,
			},
			wantContentRange: "link_visits 0-2/2",
			wantStatus:       http.StatusOK,
			wantBody:         storedVisitsJSON,
		},
		{
			name:         "returns an empty array when nothing is stored",
			countRecords: countedRecords(0),
			listRecords: func(context.Context, db.GetLinkVisitsParams) ([]db.LinkVisit, error) {
				return nil, nil
			},
			wantQueryCalled: true,
			wantParameters: db.GetLinkVisitsParams{
				PageOffset: 0,
				PageSize:   0,
			},
			wantContentRange: "link_visits 0-0/0",
			wantStatus:       http.StatusOK,
			wantBody:         "[]",
		},
		{
			name:            "skips the offset of a range",
			query:           "?range=[10,%2020]",
			countRecords:    countedRecords(357),
			listRecords:     storedVisits,
			wantQueryCalled: true,
			wantParameters: db.GetLinkVisitsParams{
				PageOffset: 10,
				PageSize:   10,
			},
			wantContentRange: "link_visits 10-20/357",
			wantStatus:       http.StatusOK,
			wantBody:         storedVisitsJSON,
		},
		{
			name:         "rejects a malformed range",
			query:        "?range=[10,5]",
			countRecords: countedRecords(357),
			listRecords:  storedVisits,
			wantStatus:   http.StatusBadRequest,
			wantBody:     invalidRequestBody,
		},
		{
			name: "returns internal server error when counting fails",
			countRecords: func(context.Context) (int64, error) {
				return 0, errQueryFailed
			},
			listRecords: storedVisits,
			wantStatus:  http.StatusInternalServerError,
			wantBody:    nullJSONBody,
		},
		{
			name:         "returns internal server error when listing fails",
			countRecords: countedRecords(2),
			listRecords: func(context.Context, db.GetLinkVisitsParams) ([]db.LinkVisit, error) {
				return nil, errQueryFailed
			},
			wantQueryCalled: true,
			wantParameters: db.GetLinkVisitsParams{
				PageOffset: 0,
				PageSize:   2,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   nullJSONBody,
		},
	}

	runIndexCases(t, visitsPath, testCases, func(
		countRecords func(ctx context.Context) (int64, error),
		listRecords func(
			ctx context.Context,
			parameters db.GetLinkVisitsParams,
		) ([]db.LinkVisit, error),
	) stubQuerier {
		return stubQuerier{countLinkVisits: countRecords, getLinkVisits: listRecords}
	})
}
