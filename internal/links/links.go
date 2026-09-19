package links

import (
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrNotFound        = errors.New("link not found")
	ErrShortNameTaken  = errors.New("short name already in use")
	ErrNoFreeShortName = errors.New("ran out of short name attempts")
)

type Store interface {
	CreateLink(ctx context.Context, arg db.CreateLinkParams) (db.Link, error)
	UpdateLink(ctx context.Context, arg db.UpdateLinkParams) (db.Link, error)
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (service Service) Create(
	ctx context.Context,
	originalURL string,
	shortName string,
) (db.Link, error) {
	if shortName == "" {
		return service.createWithGeneratedShortName(ctx, originalURL)
	}

	return service.createWithShortName(ctx, originalURL, shortName)
}

func (service Service) Update(
	ctx context.Context,
	linkID int64,
	originalURL string,
	shortName string,
) (db.Link, error) {
	link, err := service.store.UpdateLink(ctx, db.UpdateLinkParams{
		ID:          linkID,
		OriginalURL: originalURL,
		ShortName:   shortName,
	})

	if errors.Is(err, sql.ErrNoRows) {
		return db.Link{}, fmt.Errorf("link %d: %w", linkID, ErrNotFound)
	}

	if isShortNameTaken(err) {
		return db.Link{}, fmt.Errorf("%w: %w", ErrShortNameTaken, err)
	}

	if err != nil {
		return db.Link{}, fmt.Errorf("update link %d: %w", linkID, err)
	}

	return link, nil
}
