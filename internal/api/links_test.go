package api_test

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	originalURL    = "https://example.com"
	shortName      = "example"
	linkPath       = "/api/links/1"
	collectionPath = "/api/links"
	nullJSONBody   = "null"
	validLinkBody  = `{"original_url": "https://example.com", "short_name": "example"}`
)

var errQueryFailed = errors.New("query failed")

type stubQuerier struct {
	getLinks    func(ctx context.Context) ([]db.Link, error)
	createLink  func(ctx context.Context, parameters db.CreateLinkParams) (db.Link, error)
	getLinkByID func(ctx context.Context, linkID int64) (db.Link, error)
	deleteLink  func(ctx context.Context, linkID int64) (int64, error)
}

func (stub stubQuerier) GetLinks(ctx context.Context) ([]db.Link, error) {
	return stub.getLinks(ctx)
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

func (stub stubQuerier) GetLinkByshortName(context.Context, string) (db.Link, error) {
	panic("GetLinkByshortName is not routed and must never be called")
}

func (stub stubQuerier) UpdateLink(context.Context, db.UpdateLinkParams) (db.Link, error) {
	panic("UpdateLink is not routed and must never be called")
}

func newLink(linkID int64) db.Link {
	return db.Link{
		ID:          linkID,
		OriginalUrl: originalURL,
		ShortName:   shortName,
		CreatedAt:   time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC),
	}
}

func linkJSON(link db.Link) string {
	return fmt.Sprintf(
		`{"id": %d, "original_url": %q, "short_name": %q, "created_at": %q}`,
		link.ID,
		link.OriginalUrl,
		link.ShortName,
		link.CreatedAt.Format(time.RFC3339),
	)
}

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

	api.NewRouter(queries).ServeHTTP(recorder, request)

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

func TestIndexLinks(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		getLinks   func(ctx context.Context) ([]db.Link, error)
		wantStatus int
		wantBody   string
	}{
		{
			name: "returns stored links",
			getLinks: func(context.Context) ([]db.Link, error) {
				return []db.Link{newLink(1), newLink(2)}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   "[" + linkJSON(newLink(1)) + "," + linkJSON(newLink(2)) + "]",
		},
		{
			name: "returns an empty array when nothing is stored",
			getLinks: func(context.Context) ([]db.Link, error) {
				return nil, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
		{
			name: "returns internal server error when listing fails",
			getLinks: func(context.Context) ([]db.Link, error) {
				return nil, errQueryFailed
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   nullJSONBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queries := stubQuerier{getLinks: testCase.getLinks}
			recorder := performRequest(t, queries, http.MethodGet, collectionPath, "")

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
		})
	}
}

func TestCreateLink(t *testing.T) {
	t.Parallel()

	storedLink := func(context.Context, db.CreateLinkParams) (db.Link, error) {
		return newLink(1), nil
	}

	testCases := []struct {
		name            string
		body            string
		createLink      func(ctx context.Context, parameters db.CreateLinkParams) (db.Link, error)
		wantQueryCalled bool
		wantParameters  db.CreateLinkParams
		wantStatus      int
		wantBody        string
	}{
		{
			name:            "creates a link from the request body",
			body:            validLinkBody,
			createLink:      storedLink,
			wantQueryCalled: true,
			wantParameters: db.CreateLinkParams{
				OriginalUrl: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusCreated,
			wantBody:   linkJSON(newLink(1)),
		},
		{
			name:       "rejects a malformed body",
			body:       `{"original_url":`,
			createLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   nullJSONBody,
		},
		{
			name:       "rejects an empty body",
			body:       "",
			createLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   nullJSONBody,
		},
		{
			name:       "rejects a body without an original url",
			body:       `{"short_name": "example"}`,
			createLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   nullJSONBody,
		},
		{
			name:       "rejects a body without a short name",
			body:       `{"original_url": "https://example.com"}`,
			createLink: storedLink,
			wantStatus: http.StatusBadRequest,
			wantBody:   nullJSONBody,
		},
		{
			name: "returns internal server error when creating fails",
			body: validLinkBody,
			createLink: func(context.Context, db.CreateLinkParams) (db.Link, error) {
				return db.Link{}, errQueryFailed
			},
			wantQueryCalled: true,
			wantParameters: db.CreateLinkParams{
				OriginalUrl: originalURL,
				ShortName:   shortName,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   nullJSONBody,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false

			var receivedParameters db.CreateLinkParams

			queries := stubQuerier{
				createLink: func(
					ctx context.Context,
					parameters db.CreateLinkParams,
				) (db.Link, error) {
					queryCalled = true
					receivedParameters = parameters

					return testCase.createLink(ctx, parameters)
				},
			}

			recorder := performRequest(t, queries, http.MethodPost, collectionPath, testCase.body)

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantParameters, receivedParameters)
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
			wantBody:        nullJSONBody,
		},
		{
			name:        "rejects a non numeric identifier",
			path:        "/api/links/abc",
			getLinkByID: storedLink,
			wantStatus:  http.StatusBadRequest,
			wantBody:    nullJSONBody,
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
			wantBody:        nullJSONBody,
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
			wantBody:        nullJSONBody,
		},
		{
			name:       "rejects a non numeric identifier",
			path:       "/api/links/abc",
			deleteLink: deletedOneRow,
			wantStatus: http.StatusBadRequest,
			wantBody:   nullJSONBody,
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
			wantBody:        nullJSONBody,
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
