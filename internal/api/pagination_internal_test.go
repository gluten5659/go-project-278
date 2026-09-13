package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePageRange(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		rawRange         string
		totalRecords     int64
		wantError        bool
		wantPageSize     int64
		wantContentRange string
	}{
		{
			name:             "takes the whole collection without a range",
			totalRecords:     42,
			wantPageSize:     42,
			wantContentRange: "links 0-41/42",
		},
		{
			name:             "reports an empty collection without a range",
			totalRecords:     0,
			wantPageSize:     0,
			wantContentRange: "links 0-0/0",
		},
		{
			name:             "counts both bounds of a range",
			rawRange:         "[0,9]",
			totalRecords:     42,
			wantPageSize:     10,
			wantContentRange: "links 0-9/42",
		},
		{
			name:             "takes a single record range",
			rawRange:         "[3,3]",
			totalRecords:     42,
			wantPageSize:     1,
			wantContentRange: "links 3-3/42",
		},
		{
			name:             "skips the offset of a range",
			rawRange:         "[10,19]",
			totalRecords:     42,
			wantPageSize:     10,
			wantContentRange: "links 10-19/42",
		},
		{
			name:             "clamps a range that reaches past the last record",
			rawRange:         "[0,99]",
			totalRecords:     42,
			wantPageSize:     100,
			wantContentRange: "links 0-41/42",
		},
		{
			name:         "rejects a malformed range",
			rawRange:     "[0",
			totalRecords: 42,
			wantError:    true,
		},
		{
			name:         "rejects a range without two bounds",
			rawRange:     "[1,2,3]",
			totalRecords: 42,
			wantError:    true,
		},
		{
			name:         "rejects a negative range",
			rawRange:     "[-1,10]",
			totalRecords: 42,
			wantError:    true,
		},
		{
			name:         "rejects a reversed range",
			rawRange:     "[10,5]",
			totalRecords: 42,
			wantError:    true,
		},
		{
			name:         "rejects a range above the page limit",
			rawRange:     "[0,1000]",
			totalRecords: 42,
			wantError:    true,
		},
		{
			name:             "accepts a range at the page limit",
			rawRange:         "[0,999]",
			totalRecords:     2000,
			wantPageSize:     1000,
			wantContentRange: "links 0-999/2000",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			bounds, err := parsePageRange(testCase.rawRange, testCase.totalRecords)

			if testCase.wantError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantPageSize, bounds.pageSize())
			assert.Equal(
				t,
				testCase.wantContentRange,
				bounds.contentRange(linksResource, testCase.totalRecords),
			)
		})
	}
}
