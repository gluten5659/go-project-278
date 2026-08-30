package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type indexCase[Parameters any, Record any] struct {
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

func runIndexCases[Parameters any, Record any](
	t *testing.T,
	path string,
	testCases []indexCase[Parameters, Record],
	newQueries func(
		countRecords func(ctx context.Context) (int64, error),
		listRecords func(ctx context.Context, parameters Parameters) ([]Record, error),
	) stubQuerier,
) {
	t.Helper()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queryCalled := false

			var receivedParameters Parameters

			queries := newQueries(
				testCase.countRecords,
				func(ctx context.Context, parameters Parameters) ([]Record, error) {
					queryCalled = true
					receivedParameters = parameters

					return testCase.listRecords(ctx, parameters)
				},
			)

			recorder := performRequest(t, queries, http.MethodGet, path+testCase.query, "")

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
			require.Equal(t, testCase.wantQueryCalled, queryCalled)
			assert.Equal(t, testCase.wantContentRange, recorder.Header().Get("Content-Range"))

			if testCase.wantQueryCalled {
				assert.Equal(t, testCase.wantParameters, receivedParameters)
			}
		})
	}
}
