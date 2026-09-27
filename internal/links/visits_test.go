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

func TestCountVisits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "returns the number of stored visits",
		},
		{
			name:       "passes a failed count through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := links.NewService(stubStore{
				countLinkVisits: func(context.Context) (int64, error) {
					return totalVisits, testCase.queryError
				},
			})

			total, err := service.CountVisits(t.Context())

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, int64(totalVisits), total)
		})
	}
}

func TestListVisits(t *testing.T) {
	t.Parallel()

	page := []db.LinkVisit{{ID: 1, LinkID: linkID}}

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "returns the requested page",
		},
		{
			name:       "passes a failed page query through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var receivedParameters db.GetLinkVisitsParams

			service := links.NewService(stubStore{
				getLinkVisits: func(
					_ context.Context,
					parameters db.GetLinkVisitsParams,
				) ([]db.LinkVisit, error) {
					receivedParameters = parameters

					return page, testCase.queryError
				},
			})

			listed, err := service.ListVisits(t.Context(), pageOffset, pageSize)

			assert.Equal(
				t,
				db.GetLinkVisitsParams{PageOffset: pageOffset, PageSize: pageSize},
				receivedParameters,
			)

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, page, listed)
		})
	}
}
