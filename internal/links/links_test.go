package links_test

import (
	"code/internal/db"
	"code/internal/links"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	originalURL = "https://example.com"
	shortName   = "example"
	linkID      = 7

	totalLinks  = 42
	totalVisits = 84
	pageOffset  = 10
	pageSize    = 5
)

var errQueryFailed = errors.New("query failed")

type stubStore struct {
	createLink         func(ctx context.Context, parameters db.CreateLinkParams) (db.Link, error)
	updateLink         func(ctx context.Context, parameters db.UpdateLinkParams) (db.Link, error)
	deleteLink         func(ctx context.Context, linkID int64) (int64, error)
	countLinks         func(ctx context.Context) (int64, error)
	getLinks           func(ctx context.Context, parameters db.GetLinksParams) ([]db.Link, error)
	getLinkByID        func(ctx context.Context, linkID int64) (db.Link, error)
	getLinkByShortName func(ctx context.Context, shortName string) (db.Link, error)
	createLinkVisit    func(
		ctx context.Context,
		parameters db.CreateLinkVisitParams,
	) (db.LinkVisit, error)
	countLinkVisits func(ctx context.Context) (int64, error)
	getLinkVisits   func(
		ctx context.Context,
		parameters db.GetLinkVisitsParams,
	) ([]db.LinkVisit, error)
}

func (stub stubStore) DeleteLink(ctx context.Context, linkID int64) (int64, error) {
	if stub.deleteLink == nil {
		panic("DeleteLink was not expected to be called")
	}

	return stub.deleteLink(ctx, linkID)
}

func (stub stubStore) CountLinks(ctx context.Context) (int64, error) {
	if stub.countLinks == nil {
		panic("CountLinks was not expected to be called")
	}

	return stub.countLinks(ctx)
}

func (stub stubStore) GetLinks(
	ctx context.Context,
	parameters db.GetLinksParams,
) ([]db.Link, error) {
	if stub.getLinks == nil {
		panic("GetLinks was not expected to be called")
	}

	return stub.getLinks(ctx, parameters)
}

func (stub stubStore) CreateLink(
	ctx context.Context,
	parameters db.CreateLinkParams,
) (db.Link, error) {
	if stub.createLink == nil {
		panic("CreateLink was not expected to be called")
	}

	return stub.createLink(ctx, parameters)
}

func (stub stubStore) UpdateLink(
	ctx context.Context,
	parameters db.UpdateLinkParams,
) (db.Link, error) {
	if stub.updateLink == nil {
		panic("UpdateLink was not expected to be called")
	}

	return stub.updateLink(ctx, parameters)
}

func (stub stubStore) GetLinkById(ctx context.Context, linkID int64) (db.Link, error) {
	if stub.getLinkByID == nil {
		panic("GetLinkById was not expected to be called")
	}

	return stub.getLinkByID(ctx, linkID)
}

func (stub stubStore) GetLinkByShortName(
	ctx context.Context,
	requestedShortName string,
) (db.Link, error) {
	if stub.getLinkByShortName == nil {
		panic("GetLinkByShortName was not expected to be called")
	}

	return stub.getLinkByShortName(ctx, requestedShortName)
}

func (stub stubStore) CreateLinkVisit(
	ctx context.Context,
	parameters db.CreateLinkVisitParams,
) (db.LinkVisit, error) {
	if stub.createLinkVisit == nil {
		panic("CreateLinkVisit was not expected to be called")
	}

	return stub.createLinkVisit(ctx, parameters)
}

func (stub stubStore) CountLinkVisits(ctx context.Context) (int64, error) {
	if stub.countLinkVisits == nil {
		panic("CountLinkVisits was not expected to be called")
	}

	return stub.countLinkVisits(ctx)
}

func (stub stubStore) GetLinkVisits(
	ctx context.Context,
	parameters db.GetLinkVisitsParams,
) ([]db.LinkVisit, error) {
	if stub.getLinkVisits == nil {
		panic("GetLinkVisits was not expected to be called")
	}

	return stub.getLinkVisits(ctx, parameters)
}

func uniqueViolation(constraintName string) error {
	return &pgconn.PgError{Code: links.UniqueViolationCode, ConstraintName: constraintName}
}

func storedLink(storedShortName string) db.Link {
	return db.Link{
		ID:          linkID,
		OriginalURL: originalURL,
		ShortName:   storedShortName,
		CreatedAt:   time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "updates the link",
		},
		{
			name:       "reports a missing link",
			queryError: sql.ErrNoRows,
			wantError:  links.ErrNotFound,
		},
		{
			name:       "reports a taken short name",
			queryError: uniqueViolation(links.ShortNameIndex),
			wantError:  links.ErrShortNameTaken,
		},
		{
			name:       "passes an unrelated failure through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var receivedParameters db.UpdateLinkParams

			service := links.NewService(stubStore{
				updateLink: func(
					_ context.Context,
					parameters db.UpdateLinkParams,
				) (db.Link, error) {
					receivedParameters = parameters

					if testCase.queryError != nil {
						return db.Link{}, testCase.queryError
					}

					return storedLink(parameters.ShortName), nil
				},
			})

			link, err := service.Update(t.Context(), linkID, originalURL, shortName)

			assert.Equal(t, db.UpdateLinkParams{
				ID:          linkID,
				OriginalURL: originalURL,
				ShortName:   shortName,
			}, receivedParameters)

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, storedLink(shortName), link)
		})
	}
}

func TestFindAndResolve(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "returns the stored link",
		},
		{
			name:       "reports a missing link",
			queryError: sql.ErrNoRows,
			wantError:  links.ErrNotFound,
		},
		{
			name:       "passes an unrelated failure through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	lookups := []struct {
		name   string
		lookup func(ctx context.Context, service links.Service) (db.Link, error)
	}{
		{
			name: "find by identifier",
			lookup: func(ctx context.Context, service links.Service) (db.Link, error) {
				return service.Find(ctx, linkID)
			},
		},
		{
			name: "resolve by short name",
			lookup: func(ctx context.Context, service links.Service) (db.Link, error) {
				return service.Resolve(ctx, shortName)
			},
		},
	}

	for _, lookup := range lookups {
		for _, testCase := range testCases {
			t.Run(lookup.name+" "+testCase.name, func(t *testing.T) {
				t.Parallel()

				answer := func() (db.Link, error) {
					if testCase.queryError != nil {
						return db.Link{}, testCase.queryError
					}

					return storedLink(shortName), nil
				}

				service := links.NewService(stubStore{
					getLinkByID: func(_ context.Context, requestedID int64) (db.Link, error) {
						assert.Equal(t, int64(linkID), requestedID)

						return answer()
					},
					getLinkByShortName: func(
						_ context.Context,
						requestedShortName string,
					) (db.Link, error) {
						assert.Equal(t, shortName, requestedShortName)

						return answer()
					},
				})

				link, err := lookup.lookup(t.Context(), service)

				if testCase.wantError != nil {
					require.ErrorIs(t, err, testCase.wantError)

					return
				}

				require.NoError(t, err)
				assert.Equal(t, storedLink(shortName), link)
			})
		}
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		deletedCount int64
		queryError   error
		wantError    error
	}{
		{
			name:         "deletes the link",
			deletedCount: 1,
		},
		{
			name:      "reports a link that was not there",
			wantError: links.ErrNotFound,
		},
		{
			name:       "passes a failed delete through",
			queryError: errQueryFailed,
			wantError:  errQueryFailed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			receivedID := int64(0)

			service := links.NewService(stubStore{
				deleteLink: func(_ context.Context, requestedID int64) (int64, error) {
					receivedID = requestedID

					return testCase.deletedCount, testCase.queryError
				},
			})

			err := service.Delete(t.Context(), linkID)

			assert.Equal(t, int64(linkID), receivedID)

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		queryError error
		wantError  error
	}{
		{
			name: "returns the number of stored links",
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
				countLinks: func(context.Context) (int64, error) {
					return totalLinks, testCase.queryError
				},
			})

			total, err := service.Count(t.Context())

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, int64(totalLinks), total)
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	page := []db.Link{storedLink(shortName)}

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

			var receivedParameters db.GetLinksParams

			service := links.NewService(stubStore{
				getLinks: func(
					_ context.Context,
					parameters db.GetLinksParams,
				) ([]db.Link, error) {
					receivedParameters = parameters

					return page, testCase.queryError
				},
			})

			listed, err := service.List(t.Context(), pageOffset, pageSize)

			assert.Equal(
				t,
				db.GetLinksParams{PageOffset: pageOffset, PageSize: pageSize},
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
