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
)

var errQueryFailed = errors.New("query failed")

type stubStore struct {
	createLink func(ctx context.Context, parameters db.CreateLinkParams) (db.Link, error)
	updateLink func(ctx context.Context, parameters db.UpdateLinkParams) (db.Link, error)
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
