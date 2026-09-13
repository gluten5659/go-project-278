package db_test

import (
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	migrationsDirectory = "../../db/migrations"
	databaseVariable    = "TEST_DATABASE_DSN"

	originalURL = "https://example.com"
	renamedURL  = "https://hexlet.io"

	visitIP        = "192.0.2.1"
	visitUserAgent = "curl/8.5.0"
	visitReferer   = "https://news.example/post"
	visitStatus    = 302

	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"
	shortNameIndex          = "idx_short_name"
)

func pagedNames() []string {
	return []string{"first", "second", "third", "fourth"}
}

func TestMain(m *testing.M) {
	databaseDSN, isSet := os.LookupEnv(databaseVariable)
	if !isSet || databaseDSN == "" {
		os.Exit(m.Run())
	}

	conn, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		log.Fatalf("connect to test database: %v", err)
	}

	goose.SetLogger(goose.NopLogger())

	err = goose.SetDialect("postgres")
	if err != nil {
		log.Fatalf("select goose dialect: %v", err)
	}

	err = goose.Up(conn, migrationsDirectory)
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	err = conn.Close()
	if err != nil {
		log.Fatalf("close test database: %v", err)
	}

	os.Exit(m.Run())
}

func newQueries(t *testing.T) *db.Queries {
	t.Helper()

	databaseDSN, isSet := os.LookupEnv(databaseVariable)
	if !isSet || databaseDSN == "" {
		t.Skip(databaseVariable + " is not set")
	}

	conn, err := sql.Open("pgx", databaseDSN)
	require.NoError(t, err)

	transaction, err := conn.BeginTx(context.WithoutCancel(t.Context()), nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		rollbackErr := transaction.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			t.Errorf("roll back transaction: %v", rollbackErr)
		}

		require.NoError(t, conn.Close())
	})

	return db.New(transaction)
}

func createLink(t *testing.T, queries *db.Queries, shortName string) db.Link {
	t.Helper()

	link, err := queries.CreateLink(t.Context(), db.CreateLinkParams{
		OriginalURL: originalURL,
		ShortName:   shortName,
	})
	require.NoError(t, err)

	return link
}

func recordVisit(t *testing.T, queries *db.Queries, linkID int64) db.LinkVisit {
	t.Helper()

	visit, err := queries.CreateLinkVisit(t.Context(), db.CreateLinkVisitParams{
		LinkID:    linkID,
		IP:        visitIP,
		UserAgent: visitUserAgent,
		Referer:   visitReferer,
		Status:    visitStatus,
	})
	require.NoError(t, err)

	return visit
}

func TestCreateLink(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)

	link := createLink(t, queries, t.Name())

	assert.Positive(t, link.ID)
	assert.Equal(t, originalURL, link.OriginalURL)
	assert.Equal(t, t.Name(), link.ShortName)
	assert.False(t, link.CreatedAt.IsZero(), "the database fills created_at")
}

func TestCreateLinkRejectsATakenShortName(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)

	createLink(t, queries, t.Name())

	_, err := queries.CreateLink(t.Context(), db.CreateLinkParams{
		OriginalURL: originalURL,
		ShortName:   t.Name(),
	})

	var pgError *pgconn.PgError

	require.ErrorAs(t, err, &pgError)
	assert.Equal(t, uniqueViolationCode, pgError.Code)
	assert.Equal(t, shortNameIndex, pgError.ConstraintName)
}

func TestReadStoredLink(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		read func(queries *db.Queries, stored db.Link) (db.Link, error)
	}{
		{
			name: "by identifier",
			read: func(queries *db.Queries, stored db.Link) (db.Link, error) {
				return queries.GetLinkById(t.Context(), stored.ID)
			},
		},
		{
			name: "by short name",
			read: func(queries *db.Queries, stored db.Link) (db.Link, error) {
				return queries.GetLinkByShortName(t.Context(), stored.ShortName)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queries := newQueries(t)
			stored := createLink(t, queries, t.Name())

			link, err := testCase.read(queries, stored)

			require.NoError(t, err)
			assert.Equal(t, stored, link)
		})
	}
}

func TestReadMissingLink(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		read func(queries *db.Queries) (db.Link, error)
	}{
		{
			name: "by identifier",
			read: func(queries *db.Queries) (db.Link, error) {
				return queries.GetLinkById(t.Context(), -1)
			},
		},
		{
			name: "by short name",
			read: func(queries *db.Queries) (db.Link, error) {
				return queries.GetLinkByShortName(t.Context(), "no such name")
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queries := newQueries(t)

			_, err := testCase.read(queries)

			require.ErrorIs(t, err, sql.ErrNoRows)
		})
	}
}

func TestGetLinksPaging(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		pageOffset int64
		pageSize   int64
		wantCount  int
		wantFirst  string
	}{
		{
			name:       "takes the whole collection",
			pageOffset: 0,
			pageSize:   4,
			wantCount:  4,
			wantFirst:  pagedNames()[0],
		},
		{
			name:       "takes the first page",
			pageOffset: 0,
			pageSize:   2,
			wantCount:  2,
			wantFirst:  pagedNames()[0],
		},
		{
			name:       "skips the offset",
			pageOffset: 2,
			pageSize:   2,
			wantCount:  2,
			wantFirst:  pagedNames()[2],
		},
		{
			name:       "stops at the last record",
			pageOffset: 3,
			pageSize:   10,
			wantCount:  1,
			wantFirst:  pagedNames()[3],
		},
		{
			name:       "returns nothing past the end",
			pageOffset: 10,
			pageSize:   10,
			wantCount:  0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queries := newQueries(t)

			for _, position := range pagedNames() {
				createLink(t, queries, t.Name()+position)
			}

			total, err := queries.CountLinks(t.Context())
			require.NoError(t, err)
			require.Equal(t, int64(len(pagedNames())), total)

			links, err := queries.GetLinks(t.Context(), db.GetLinksParams{
				PageOffset: testCase.pageOffset,
				PageSize:   testCase.pageSize,
			})
			require.NoError(t, err)
			require.Len(t, links, testCase.wantCount)

			if testCase.wantCount > 0 {
				assert.Equal(t, t.Name()+testCase.wantFirst, links[0].ShortName)
			}
		})
	}
}

func TestUpdateLink(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)
	stored := createLink(t, queries, t.Name())

	updated, err := queries.UpdateLink(t.Context(), db.UpdateLinkParams{
		ID:          stored.ID,
		OriginalURL: renamedURL,
		ShortName:   t.Name() + "renamed",
	})
	require.NoError(t, err)

	assert.Equal(t, stored.ID, updated.ID)
	assert.Equal(t, renamedURL, updated.OriginalURL)
	assert.Equal(t, t.Name()+"renamed", updated.ShortName)
	assert.Equal(t, stored.CreatedAt, updated.CreatedAt)
}

func TestUpdateMissingLink(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)

	_, err := queries.UpdateLink(t.Context(), db.UpdateLinkParams{
		ID:          -1,
		OriginalURL: originalURL,
		ShortName:   t.Name(),
	})

	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteLink(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		linkExist bool
		wantCount int64
	}{
		{
			name:      "reports the deleted row",
			linkExist: true,
			wantCount: 1,
		},
		{
			name:      "reports nothing for a missing link",
			linkExist: false,
			wantCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queries := newQueries(t)
			linkID := int64(-1)

			if testCase.linkExist {
				linkID = createLink(t, queries, t.Name()).ID
			}

			deleted, err := queries.DeleteLink(t.Context(), linkID)

			require.NoError(t, err)
			assert.Equal(t, testCase.wantCount, deleted)
		})
	}
}

func TestDeleteLinkRemovesItsVisits(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)
	stored := createLink(t, queries, t.Name())

	visit := recordVisit(t, queries, stored.ID)
	assert.Equal(t, stored.ID, visit.LinkID)
	assert.False(t, visit.CreatedAt.IsZero(), "the database fills created_at")

	_, err := queries.DeleteLink(t.Context(), stored.ID)
	require.NoError(t, err)

	remaining, err := queries.CountLinkVisits(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(0), remaining)
}

func TestGetLinkVisitsPaging(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)
	stored := createLink(t, queries, t.Name())

	for range 3 {
		recordVisit(t, queries, stored.ID)
	}

	total, err := queries.CountLinkVisits(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(3), total)

	visits, err := queries.GetLinkVisits(t.Context(), db.GetLinkVisitsParams{
		PageOffset: 1,
		PageSize:   2,
	})
	require.NoError(t, err)
	require.Len(t, visits, 2)

	for _, visit := range visits {
		assert.Equal(t, stored.ID, visit.LinkID)
		assert.Equal(t, visitIP, visit.IP)
		assert.Equal(t, int32(visitStatus), visit.Status)
	}
}

func TestVisitNeedsAnExistingLink(t *testing.T) {
	t.Parallel()

	queries := newQueries(t)

	_, err := queries.CreateLinkVisit(t.Context(), db.CreateLinkVisitParams{
		LinkID:    -1,
		IP:        visitIP,
		UserAgent: visitUserAgent,
		Referer:   visitReferer,
		Status:    visitStatus,
	})

	var pgError *pgconn.PgError

	require.ErrorAs(t, err, &pgError)
	assert.Equal(t, foreignKeyViolationCode, pgError.Code)
}
