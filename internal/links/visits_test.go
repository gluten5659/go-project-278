package links_test

import (
	"code/internal/db"
	"code/internal/links"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordVisit(t *testing.T) {
	t.Parallel()

	visit := links.Visit{
		LinkID:    linkID,
		IP:        "192.0.2.1",
		UserAgent: "curl/8.5.0",
		Referer:   "https://news.example/post",
		Status:    302,
	}

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "stores every field of the visit",
		},
		{
			name:       "passes a failed write through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var receivedParameters db.CreateLinkVisitParams

			service := links.NewService(stubStore{
				createLinkVisit: func(
					_ context.Context,
					parameters db.CreateLinkVisitParams,
				) (db.LinkVisit, error) {
					receivedParameters = parameters

					return db.LinkVisit{}, testCase.queryError
				},
			})

			err := service.RecordVisit(t.Context(), visit)

			assert.Equal(t, db.CreateLinkVisitParams{
				LinkID:    visit.LinkID,
				IP:        visit.IP,
				UserAgent: visit.UserAgent,
				Referer:   visit.Referer,
				Status:    visit.Status,
			}, receivedParameters)

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
		})
	}
}
