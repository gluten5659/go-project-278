package links

import (
	"code/internal/db"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ShortNameLength   = 8
	ShortNameAttempts = 3

	UniqueViolationCode = "23505"
	ShortNameIndex      = "idx_short_name"

	shortNameAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func (service Service) createWithShortName(
	ctx context.Context,
	originalURL string,
	shortName string,
) (db.Link, error) {
	link, err := service.store.CreateLink(ctx, db.CreateLinkParams{
		OriginalURL: originalURL,
		ShortName:   shortName,
	})

	if isShortNameTaken(err) {
		return db.Link{}, fmt.Errorf("%w: %w", ErrShortNameTaken, err)
	}

	if err != nil {
		return db.Link{}, fmt.Errorf("create link %q: %w", shortName, err)
	}

	return link, nil
}

func (service Service) createWithGeneratedShortName(
	ctx context.Context,
	originalURL string,
) (db.Link, error) {
	for range ShortNameAttempts {
		shortName, err := generateShortName()
		if err != nil {
			return db.Link{}, err
		}

		link, err := service.createWithShortName(ctx, originalURL, shortName)
		if errors.Is(err, ErrShortNameTaken) {
			continue
		}

		if err != nil {
			return db.Link{}, err
		}

		return link, nil
	}

	return db.Link{}, ErrNoFreeShortName
}

func generateShortName() (string, error) {
	name := make([]byte, ShortNameLength)
	alphabetSize := big.NewInt(int64(len(shortNameAlphabet)))

	for index := range name {
		position, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("read random source: %w", err)
		}

		name[index] = shortNameAlphabet[position.Int64()]
	}

	return string(name), nil
}

func isShortNameTaken(err error) bool {
	var pgError *pgconn.PgError

	return errors.As(err, &pgError) &&
		pgError.Code == UniqueViolationCode &&
		pgError.ConstraintName == ShortNameIndex
}
