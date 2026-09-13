package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listFixture[Parameters any, Record any] struct {
	resource    string
	path        string
	records     []Record
	recordsJSON string
	parameters  func(pageOffset int64, pageSize int64) Parameters
	newQueries  func(
		countRecords func(ctx context.Context) (int64, error),
		listRecords func(ctx context.Context, parameters Parameters) ([]Record, error),
	) stubQuerier
}

type listCase[Parameters any, Record any] struct {
	name             string
	query            string
	countRecords     func(ctx context.Context) (int64, error)
	listRecords      func(ctx context.Context, parameters Parameters) ([]Record, error)
	wantQueryCalled  bool
	wantParameters   Parameters
	wantContentRange string
	wantStatus       int
	wantBody         string
}

func countedRecords(total int64) func(context.Context) (int64, error) {
	return func(context.Context) (int64, error) {
		return total, nil
	}
}

func listContractCases[Parameters any, Record any](
	fixture listFixture[Parameters, Record],
) []listCase[Parameters, Record] {
	storedRecords := func(context.Context, Parameters) ([]Record, error) {
		return fixture.records, nil
	}

	failingList := func(context.Context, Parameters) ([]Record, error) {
		return nil, errQueryFailed
	}

	return []listCase[Parameters, Record]{
		{
			name:             "returns stored records",
			countRecords:     countedRecords(2),
			listRecords:      storedRecords,
			wantQueryCalled:  true,
			wantParameters:   fixture.parameters(0, 2),
			wantContentRange: fixture.resource + " 0-1/2",
			wantStatus:       http.StatusOK,
			wantBody:         fixture.recordsJSON,
		},
		{
			name:         "returns an empty array when nothing is stored",
			countRecords: countedRecords(0),
			listRecords: func(context.Context, Parameters) ([]Record, error) {
				return nil, nil
			},
			wantQueryCalled:  true,
			wantParameters:   fixture.parameters(0, 0),
			wantContentRange: fixture.resource + " 0-0/0",
			wantStatus:       http.StatusOK,
			wantBody:         "[]",
		},
		{
			name:             "passes the requested range to the query",
			query:            "?range=[5,%2014]",
			countRecords:     countedRecords(42),
			listRecords:      storedRecords,
			wantQueryCalled:  true,
			wantParameters:   fixture.parameters(5, 10),
			wantContentRange: fixture.resource + " 5-14/42",
			wantStatus:       http.StatusOK,
			wantBody:         fixture.recordsJSON,
		},
		{
			name:         "rejects a range the parser refuses",
			query:        "?range=[10,5]",
			countRecords: countedRecords(42),
			listRecords:  storedRecords,
			wantStatus:   http.StatusBadRequest,
			wantBody:     invalidRequestBody,
		},
		{
			name: "returns internal server error when counting fails",
			countRecords: func(context.Context) (int64, error) {
				return 0, errQueryFailed
			},
			listRecords: storedRecords,
			wantStatus:  http.StatusInternalServerError,
			wantBody:    internalErrorBody,
		},
		{
			name:            "returns internal server error when listing fails",
			countRecords:    countedRecords(2),
			listRecords:     failingList,
			wantQueryCalled: true,
			wantParameters:  fixture.parameters(0, 2),
			wantStatus:      http.StatusInternalServerError,
			wantBody:        internalErrorBody,
		},
	}
}

func runListContract[Parameters any, Record any](
	t *testing.T,
	fixture listFixture[Parameters, Record],
) {
	t.Helper()

	for _, testCase := range listContractCases(fixture) {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false

			var receivedParameters Parameters

			queries := fixture.newQueries(
				testCase.countRecords,
				func(ctx context.Context, parameters Parameters) ([]Record, error) {
					queryCalled = true
					receivedParameters = parameters

					return testCase.listRecords(ctx, parameters)
				},
			)

			recorder := performRequest(
				t,
				queries,
				http.MethodGet,
				fixture.path+testCase.query,
				"",
			)

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)
			assert.Equal(t, testCase.wantContentRange, recorder.Header().Get("Content-Range"))

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantParameters, receivedParameters)
			}
		})
	}
}
