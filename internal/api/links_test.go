package api_test

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	originalURL        = "https://example.com"
	shortName          = "example"
	linkPath           = "/api/links/1"
	unparsableLinkPath = "/api/links/abc"
	collectionPath     = "/api/links"
	invalidRequestBody = `{"error": "invalid request"}`
	notFoundBody       = `{"error": "not found"}`
	internalErrorBody  = `{"error": "internal server error"}`
	unavailableBody    = `{"error": "service unavailable"}`
	allowedOrigin      = "http://localhost:5173"
	validLinkBody      = `{"original_url": "https://example.com", "short_name": "example"}`
	namelessBody       = `{"original_url": "https://example.com"}`

	unparsableIDCase = "rejects a non numeric identifier"

	takenShortNameBody = `{"errors": {"short_name": "short name already in use"}}`

	createRequestStructName = "createLinkRequest"
	updateRequestStructName = "updateLinkRequest"

	originalURLField = "original_url"

	requiredTag  = "required"
	httpURLTag   = "http_url"
	minLengthTag = "min"
	maxLengthTag = "max"

	malformedBody         = `{"original_url":`
	invalidURLBody        = `{"original_url": "example.com", "short_name": "example"}`
	shortNameTooShortBody = `{"original_url": "https://example.com", "short_name": "ab"}`
	shortNameTooLongBody  = `{"original_url": "https://example.com",` +
		` "short_name": "abcdefghijklmnopqrstuvwxyz0123456789"}`
	everyFieldInvalidBody = `{"original_url": "example.com", "short_name": "ab"}`
)

func uniqueViolation(constraintName string) error {
	return &pgconn.PgError{Code: api.UniqueViolationCode, ConstraintName: constraintName}
}

const queryFailedMessage = "query failed"

var errQueryFailed = errors.New(queryFailedMessage)

type stubQuerier struct {
	countLinks         func(ctx context.Context) (int64, error)
	getLinks           func(ctx context.Context, parameters db.GetLinksParams) ([]db.Link, error)
	createLink         func(ctx context.Context, parameters db.CreateLinkParams) (db.Link, error)
	getLinkByID        func(ctx context.Context, linkID int64) (db.Link, error)
	getLinkByShortName func(ctx context.Context, shortName string) (db.Link, error)
	deleteLink         func(ctx context.Context, linkID int64) (int64, error)
	updateLink         func(
		ctx context.Context,
		parameters db.UpdateLinkParams,
	) (db.Link, error)
	countLinkVisits func(ctx context.Context) (int64, error)
	getLinkVisits   func(
		ctx context.Context,
		parameters db.GetLinkVisitsParams,
	) ([]db.LinkVisit, error)
	createLinkVisit func(
		ctx context.Context,
		parameters db.CreateLinkVisitParams,
	) (db.LinkVisit, error)
}

func (stub stubQuerier) CountLinks(ctx context.Context) (int64, error) {
	return stub.countLinks(ctx)
}

func (stub stubQuerier) GetLinks(
	ctx context.Context,
	parameters db.GetLinksParams,
) ([]db.Link, error) {
	return stub.getLinks(ctx, parameters)
}

func (stub stubQuerier) CreateLink(
	ctx context.Context,
	parameters db.CreateLinkParams,
) (db.Link, error) {
	return stub.createLink(ctx, parameters)
}

func (stub stubQuerier) GetLinkById(ctx context.Context, linkID int64) (db.Link, error) {
	return stub.getLinkByID(ctx, linkID)
}

func (stub stubQuerier) DeleteLink(ctx context.Context, linkID int64) (int64, error) {
	return stub.deleteLink(ctx, linkID)
}

func (stub stubQuerier) GetLinkByShortName(
	ctx context.Context,
	shortName string,
) (db.Link, error) {
	return stub.getLinkByShortName(ctx, shortName)
}

func (stub stubQuerier) CountLinkVisits(ctx context.Context) (int64, error) {
	return stub.countLinkVisits(ctx)
}

func (stub stubQuerier) GetLinkVisits(
	ctx context.Context,
	parameters db.GetLinkVisitsParams,
) ([]db.LinkVisit, error) {
	return stub.getLinkVisits(ctx, parameters)
}

func (stub stubQuerier) CreateLinkVisit(
	ctx context.Context,
	parameters db.CreateLinkVisitParams,
) (db.LinkVisit, error) {
	return stub.createLinkVisit(ctx, parameters)
}

func (stub stubQuerier) UpdateLink(
	ctx context.Context,
	parameters db.UpdateLinkParams,
) (db.Link, error) {
	return stub.updateLink(ctx, parameters)
}

func newLink(linkID int64) db.Link {
	return db.Link{
		ID:          linkID,
		OriginalURL: originalURL,
		ShortName:   shortName,
		CreatedAt:   time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC),
	}
}

func linkJSON(link db.Link) string {
	return fmt.Sprintf(
		`{"id": %d, "original_url": %q, "short_name": %q, "created_at": %q}`,
		link.ID,
		link.OriginalURL,
		link.ShortName,
		link.CreatedAt.Format(time.RFC3339),
	)
}

func discardErrorReports(*http.Request, error) {}

func performRequest(
	t *testing.T,
	queries db.Querier,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))

	api.NewRouter(api.Config{
		Queries:        queries,
		AllowedOrigins: []string{allowedOrigin},
		ReportError:    discardErrorReports,
	}).ServeHTTP(recorder, request)

	return recorder
}

func assertResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, body string) {
	t.Helper()

	response := recorder.Result()
	defer func() {
		require.NoError(t, response.Body.Close())
	}()

	require.Equal(t, status, response.StatusCode)

	actualBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	if body == "" {
		assert.Empty(t, string(actualBody))

		return
	}

	assert.JSONEq(t, body, string(actualBody))
}

func fieldValidationMessage(structName string, field string, failedTag string) string {
	return fmt.Sprintf(
		"Key: '%s.%s' Error:Field validation for '%s' failed on the '%s' tag",
		structName,
		field,
		field,
		failedTag,
	)
}

func invalidFieldsBody(
	t *testing.T,
	structName string,
	failedTagsByField map[string]string,
) string {
	t.Helper()

	messagesByField := make(map[string]string, len(failedTagsByField))

	for field, failedTag := range failedTagsByField {
		messagesByField[field] = fieldValidationMessage(structName, field, failedTag)
	}

	body, err := json.Marshal(map[string]map[string]string{"errors": messagesByField})
	require.NoError(t, err)

	return string(body)
}

func TestPing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "responds with pong",
			path:       "/ping",
			wantStatus: http.StatusOK,
			wantBody:   `{"message": "pong"}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder := performRequest(t, stubQuerier{}, http.MethodGet, testCase.path, "")

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
		})
	}
}

func TestUnroutedRequests(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "unknown path",
			method: http.MethodGet,
			path:   "/unknown",
		},
		{
			name:   "method without a route on the collection",
			method: http.MethodPatch,
			path:   collectionPath,
		},
		{
			name:   "method without a route on a single link",
			method: http.MethodPost,
			path:   linkPath,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder := performRequest(t, stubQuerier{}, testCase.method, testCase.path, "")

			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		method            string
		origin            string
		preflightMethod   string
		wantStatus        int
		wantAllowOrigin   string
		wantExposeHeaders string
	}{
		{
			name:              "allows a simple request from the frontend origin",
			method:            http.MethodGet,
			origin:            allowedOrigin,
			wantStatus:        http.StatusOK,
			wantAllowOrigin:   allowedOrigin,
			wantExposeHeaders: "Content-Range",
		},
		{
			name:            "answers a preflight request",
			method:          http.MethodOptions,
			origin:          allowedOrigin,
			preflightMethod: http.MethodPost,
			wantStatus:      http.StatusNoContent,
			wantAllowOrigin: allowedOrigin,
		},
		{
			name:       "rejects an unknown origin",
			method:     http.MethodGet,
			origin:     "http://evil.example",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "leaves a request without an origin alone",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)

			queries := stubQuerier{
				countLinks: func(context.Context) (int64, error) {
					return 0, nil
				},
				getLinks: func(context.Context, db.GetLinksParams) ([]db.Link, error) {
					return nil, nil
				},
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				t.Context(),
				testCase.method,
				collectionPath,
				nil,
			)

			if testCase.origin != "" {
				request.Header.Set("Origin", testCase.origin)
			}

			if testCase.preflightMethod != "" {
				request.Header.Set("Access-Control-Request-Method", testCase.preflightMethod)
			}

			api.NewRouter(api.Config{
				Queries:        queries,
				AllowedOrigins: []string{allowedOrigin},
				ReportError:    discardErrorReports,
			}).ServeHTTP(recorder, request)

			assert.Equal(t, testCase.wantStatus, recorder.Code)
			assert.Equal(
				t,
				testCase.wantAllowOrigin,
				recorder.Header().Get("Access-Control-Allow-Origin"),
			)
			assert.Equal(
				t,
				testCase.wantExposeHeaders,
				recorder.Header().Get("Access-Control-Expose-Headers"),
			)
		})
	}
}

func TestListLinks(t *testing.T) {
	t.Parallel()

	runListContract(t, listFixture[db.GetLinksParams, db.Link]{
		resource:    api.LinksResource,
		path:        collectionPath,
		records:     []db.Link{newLink(1), newLink(2)},
		recordsJSON: "[" + linkJSON(newLink(1)) + "," + linkJSON(newLink(2)) + "]",
		parameters: func(pageOffset int64, pageSize int64) db.GetLinksParams {
			return db.GetLinksParams{PageOffset: pageOffset, PageSize: pageSize}
		},
		newQueries: func(
			countRecords func(ctx context.Context) (int64, error),
			listRecords func(ctx context.Context, parameters db.GetLinksParams) ([]db.Link, error),
		) stubQuerier {
			return stubQuerier{countLinks: countRecords, getLinks: listRecords}
		},
	})
}

func TestCreateLink(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		body          string
		takenNames    int
		queryError    error
		wantCalls     int
		wantShortName string
		wantStatus    int
		wantBody      string
	}{
		{
			name:          "creates a link from the request body",
			body:          validLinkBody,
			wantCalls:     1,
			wantShortName: shortName,
			wantStatus:    http.StatusCreated,
			wantBody:      linkJSON(newLink(1)),
		},
		{
			name:       "rejects a malformed body",
			body:       malformedBody,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects an empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects a body without an original url",
			body:       `{"short_name": "example"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField: requiredTag,
			}),
		},
		{
			name:       "rejects an original url that is not a url",
			body:       invalidURLBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField: httpURLTag,
			}),
		},
		{
			name:       "rejects a short name below the minimum length",
			body:       shortNameTooShortBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:       "rejects a short name above the maximum length",
			body:       shortNameTooLongBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				api.ShortNameField: maxLengthTag,
			}),
		},
		{
			name:       "reports every invalid field at once",
			body:       everyFieldInvalidBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField:   httpURLTag,
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:          "reports the taken short name as a validation failure",
			body:          validLinkBody,
			takenNames:    1,
			wantCalls:     1,
			wantShortName: shortName,
			wantStatus:    http.StatusUnprocessableEntity,
			wantBody:      takenShortNameBody,
		},
		{
			name:          "returns internal server error when creating fails",
			body:          validLinkBody,
			queryError:    errQueryFailed,
			wantCalls:     1,
			wantShortName: shortName,
			wantStatus:    http.StatusInternalServerError,
			wantBody:      internalErrorBody,
		},
		{
			name:       "generates a short name when the body has none",
			body:       namelessBody,
			wantCalls:  1,
			wantStatus: http.StatusCreated,
			wantBody:   linkJSON(newLink(1)),
		},
		{
			name:       "retries when the generated short name is taken",
			body:       namelessBody,
			takenNames: 1,
			wantCalls:  2,
			wantStatus: http.StatusCreated,
			wantBody:   linkJSON(newLink(1)),
		},
		{
			name:       "gives up once the attempts run out",
			body:       namelessBody,
			takenNames: api.ShortNameAttempts,
			wantCalls:  api.ShortNameAttempts,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   unavailableBody,
		},
		{
			name:       "does not retry an unrelated failure",
			body:       namelessBody,
			queryError: errQueryFailed,
			wantCalls:  1,
			wantStatus: http.StatusInternalServerError,
			wantBody:   internalErrorBody,
		},
		{
			name:       "does not retry a violation of another index",
			body:       namelessBody,
			queryError: uniqueViolation("links_pkey"),
			wantCalls:  1,
			wantStatus: http.StatusInternalServerError,
			wantBody:   internalErrorBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var receivedParameters []db.CreateLinkParams

			queries := stubQuerier{
				createLink: func(
					_ context.Context,
					parameters db.CreateLinkParams,
				) (db.Link, error) {
					receivedParameters = append(receivedParameters, parameters)

					if testCase.queryError != nil {
						return db.Link{}, testCase.queryError
					}

					if len(receivedParameters) <= testCase.takenNames {
						return db.Link{}, uniqueViolation(api.ShortNameIndex)
					}

					return newLink(1), nil
				},
			}

			recorder := performRequest(t, queries, http.MethodPost, collectionPath, testCase.body)

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Len(t, receivedParameters, testCase.wantCalls)

			generatedNamePattern := fmt.Sprintf(`^[a-zA-Z]{%d}$`, api.ShortNameLength)
			seenNames := make(map[string]bool, len(receivedParameters))

			for _, parameters := range receivedParameters {
				assert.Equal(t, originalURL, parameters.OriginalURL)

				if testCase.wantShortName == "" {
					assert.Regexp(t, generatedNamePattern, parameters.ShortName)
				} else {
					assert.Equal(t, testCase.wantShortName, parameters.ShortName)
				}

				assert.False(t, seenNames[parameters.ShortName], "reused a short name")

				seenNames[parameters.ShortName] = true
			}
		})
	}
}

func TestShowLink(t *testing.T) {
	t.Parallel()

	storedLink := func(_ context.Context, linkID int64) (db.Link, error) {
		return newLink(linkID), nil
	}

	testCases := []struct {
		name            string
		path            string
		getLinkByID     func(ctx context.Context, linkID int64) (db.Link, error)
		wantQueryCalled bool
		wantLinkID      int64
		wantStatus      int
		wantBody        string
	}{
		{
			name:            "returns the requested link",
			path:            linkPath,
			getLinkByID:     storedLink,
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusOK,
			wantBody:        linkJSON(newLink(1)),
		},
		{
			name:            "accepts an identifier beyond the 32 bit range",
			path:            "/api/links/4294967296",
			getLinkByID:     storedLink,
			wantQueryCalled: true,
			wantLinkID:      4294967296,
			wantStatus:      http.StatusOK,
			wantBody:        linkJSON(newLink(4294967296)),
		},
		{
			name: "returns not found when the link is missing",
			path: linkPath,
			getLinkByID: func(context.Context, int64) (db.Link, error) {
				return db.Link{}, sql.ErrNoRows
			},
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusNotFound,
			wantBody:        notFoundBody,
		},
		{
			name:        unparsableIDCase,
			path:        unparsableLinkPath,
			getLinkByID: storedLink,
			wantStatus:  http.StatusBadRequest,
			wantBody:    invalidRequestBody,
		},
		{
			name: "returns internal server error when loading fails",
			path: linkPath,
			getLinkByID: func(context.Context, int64) (db.Link, error) {
				return db.Link{}, errQueryFailed
			},
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusInternalServerError,
			wantBody:        internalErrorBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false
			receivedLinkID := int64(0)

			queries := stubQuerier{
				getLinkByID: func(ctx context.Context, linkID int64) (db.Link, error) {
					queryCalled = true
					receivedLinkID = linkID

					return testCase.getLinkByID(ctx, linkID)
				},
			}

			recorder := performRequest(t, queries, http.MethodGet, testCase.path, "")

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantLinkID, receivedLinkID)
			}
		})
	}
}

func TestUpdateLink(t *testing.T) {
	t.Parallel()

	storedLink := func(_ context.Context, parameters db.UpdateLinkParams) (db.Link, error) {
		return newLink(parameters.ID), nil
	}

	failedWith := func(err error) func(context.Context, db.UpdateLinkParams) (db.Link, error) {
		return func(context.Context, db.UpdateLinkParams) (db.Link, error) {
			return db.Link{}, err
		}
	}

	testCases := []struct {
		name            string
		path            string
		body            string
		updateLink      func(ctx context.Context, parameters db.UpdateLinkParams) (db.Link, error)
		wantQueryCalled bool
		wantParameters  db.UpdateLinkParams
		wantStatus      int
		wantBody        string
	}{
		{
			name:            "updates the link from the request body",
			path:            linkPath,
			body:            validLinkBody,
			updateLink:      storedLink,
			wantQueryCalled: true,
			wantParameters: db.UpdateLinkParams{
				ID:          1,
				OriginalURL: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusOK,
			wantBody:   linkJSON(newLink(1)),
		},
		{
			name:            "accepts an identifier beyond the 32 bit range",
			path:            "/api/links/4294967296",
			body:            validLinkBody,
			updateLink:      storedLink,
			wantQueryCalled: true,
			wantParameters: db.UpdateLinkParams{
				ID:          4294967296,
				OriginalURL: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusOK,
			wantBody:   linkJSON(newLink(4294967296)),
		},
		{
			name:       unparsableIDCase,
			path:       unparsableLinkPath,
			body:       validLinkBody,
			updateLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects a malformed body",
			path:       linkPath,
			body:       malformedBody,
			updateLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects an empty body",
			path:       linkPath,
			body:       "",
			updateLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects a body without an original url",
			path:       linkPath,
			body:       `{"short_name": "example"}`,
			updateLink: storedLink,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				originalURLField: requiredTag,
			}),
		},
		{
			name:       "rejects an original url that is not a url",
			path:       linkPath,
			body:       invalidURLBody,
			updateLink: storedLink,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				originalURLField: httpURLTag,
			}),
		},
		{
			name:       "rejects a short name below the minimum length",
			path:       linkPath,
			body:       shortNameTooShortBody,
			updateLink: storedLink,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:       "rejects a short name above the maximum length",
			path:       linkPath,
			body:       shortNameTooLongBody,
			updateLink: storedLink,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: maxLengthTag,
			}),
		},
		{
			name:       "rejects a body without a short name",
			path:       linkPath,
			body:       namelessBody,
			updateLink: storedLink,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: requiredTag,
			}),
		},
		{
			name:            "returns not found when the link is missing",
			path:            linkPath,
			body:            validLinkBody,
			updateLink:      failedWith(sql.ErrNoRows),
			wantQueryCalled: true,
			wantParameters: db.UpdateLinkParams{
				ID:          1,
				OriginalURL: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusNotFound,
			wantBody:   notFoundBody,
		},
		{
			name:            "reports the taken short name as a validation failure",
			path:            linkPath,
			body:            validLinkBody,
			updateLink:      failedWith(uniqueViolation(api.ShortNameIndex)),
			wantQueryCalled: true,
			wantParameters: db.UpdateLinkParams{
				ID:          1,
				OriginalURL: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   takenShortNameBody,
		},
		{
			name:            "returns internal server error when updating fails",
			path:            linkPath,
			body:            validLinkBody,
			updateLink:      failedWith(errQueryFailed),
			wantQueryCalled: true,
			wantParameters: db.UpdateLinkParams{
				ID:          1,
				OriginalURL: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   internalErrorBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false
			receivedParameters := db.UpdateLinkParams{}

			queries := stubQuerier{
				updateLink: func(
					ctx context.Context,
					parameters db.UpdateLinkParams,
				) (db.Link, error) {
					queryCalled = true
					receivedParameters = parameters

					return testCase.updateLink(ctx, parameters)
				},
			}

			recorder := performRequest(t, queries, http.MethodPut, testCase.path, testCase.body)

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantParameters, receivedParameters)
			}
		})
	}
}

func TestDestroyLink(t *testing.T) {
	t.Parallel()

	deletedOneRow := func(context.Context, int64) (int64, error) {
		return 1, nil
	}

	testCases := []struct {
		name            string
		path            string
		deleteLink      func(ctx context.Context, linkID int64) (int64, error)
		wantQueryCalled bool
		wantLinkID      int64
		wantStatus      int
		wantBody        string
	}{
		{
			name:            "returns no content when the link is deleted",
			path:            linkPath,
			deleteLink:      deletedOneRow,
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusNoContent,
			wantBody:        "",
		},
		{
			name: "returns not found when nothing was deleted",
			path: linkPath,
			deleteLink: func(context.Context, int64) (int64, error) {
				return 0, nil
			},
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusNotFound,
			wantBody:        notFoundBody,
		},
		{
			name:       unparsableIDCase,
			path:       unparsableLinkPath,
			deleteLink: deletedOneRow,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name: "returns internal server error when deleting fails",
			path: linkPath,
			deleteLink: func(context.Context, int64) (int64, error) {
				return 0, errQueryFailed
			},
			wantQueryCalled: true,
			wantLinkID:      1,
			wantStatus:      http.StatusInternalServerError,
			wantBody:        internalErrorBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false
			receivedLinkID := int64(0)

			queries := stubQuerier{
				deleteLink: func(ctx context.Context, linkID int64) (int64, error) {
					queryCalled = true
					receivedLinkID = linkID

					return testCase.deleteLink(ctx, linkID)
				},
			}

			recorder := performRequest(t, queries, http.MethodDelete, testCase.path, "")

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantLinkID, receivedLinkID)
			}
		})
	}
}
