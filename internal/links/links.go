package links

import (
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrNotFound means no stored link matches what the caller asked for.
	ErrNotFound = errors.New("link not found")

	// ErrShortNameTaken means another link already holds that short name.
	ErrShortNameTaken = errors.New("short name already in use")

	// ErrNoFreeShortName means every generated name was taken and the caller
	// should try again later.
	ErrNoFreeShortName = errors.New("ran out of short name attempts")
)

// Store is the set of queries a service needs. The generated db.Querier
// satisfies it, so a test can hand over a stub instead of a database.
type Store interface {
	CreateLink(ctx context.Context, arg db.CreateLinkParams) (db.Link, error)
	UpdateLink(ctx context.Context, arg db.UpdateLinkParams) (db.Link, error)
	DeleteLink(ctx context.Context, id int64) (int64, error)
	CountLinks(ctx context.Context) (int64, error)
	GetLinks(ctx context.Context, arg db.GetLinksParams) ([]db.Link, error)
	GetLinkByID(ctx context.Context, id int64) (db.Link, error)
	GetLinkByShortName(ctx context.Context, shortName string) (db.Link, error)
	CreateLinkVisit(ctx context.Context, arg db.CreateLinkVisitParams) (db.LinkVisit, error)
	CountLinkVisits(ctx context.Context) (int64, error)
	GetLinkVisits(ctx context.Context, arg db.GetLinkVisitsParams) ([]db.LinkVisit, error)
}

// Service keeps the rules of the links themselves away from HTTP and from the
// driver. Callers read the errors of this package and never a driver error.
type Service struct {
	store Store
}

// NewService returns a service that reads and writes its links in the store.
func NewService(store Store) Service {
	return Service{store: store}
}

// Create stores a link for the original URL. An empty short name asks the
// service to generate one, a name that is already stored comes back as
// ErrShortNameTaken.
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

// Update replaces the original URL and the short name of a stored link. A link
// nobody stored comes back as ErrNotFound, a name another link holds comes back
// as ErrShortNameTaken.
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

// Delete removes a stored link along with the visits recorded for it. A link
// nobody stored comes back as ErrNotFound.
func (service Service) Delete(ctx context.Context, linkID int64) error {
	deletedCount, err := service.store.DeleteLink(ctx, linkID)
	if err != nil {
		return fmt.Errorf("delete link %d: %w", linkID, err)
	}

	if deletedCount == 0 {
		return fmt.Errorf("link %d: %w", linkID, ErrNotFound)
	}

	return nil
}

// Count reports how many links are stored, which is what a caller needs to
// describe a page of them.
func (service Service) Count(ctx context.Context) (int64, error) {
	totalLinks, err := service.store.CountLinks(ctx)
	if err != nil {
		return 0, fmt.Errorf("count links: %w", err)
	}

	return totalLinks, nil
}

// List returns at most size links, starting at offset and ordered by identifier.
func (service Service) List(ctx context.Context, offset, size int64) ([]db.Link, error) {
	storedLinks, err := service.store.GetLinks(ctx, db.GetLinksParams{
		PageOffset: offset,
		PageSize:   size,
	})
	if err != nil {
		return nil, fmt.Errorf("list %d links from %d: %w", size, offset, err)
	}

	return storedLinks, nil
}

// Find returns the link with that identifier. A link nobody stored comes back as
// ErrNotFound.
func (service Service) Find(ctx context.Context, linkID int64) (db.Link, error) {
	link, err := service.store.GetLinkByID(ctx, linkID)

	if errors.Is(err, sql.ErrNoRows) {
		return db.Link{}, fmt.Errorf("link %d: %w", linkID, ErrNotFound)
	}

	if err != nil {
		return db.Link{}, fmt.Errorf("find link %d: %w", linkID, err)
	}

	return link, nil
}

// Resolve returns the link a short name points at. An unknown name comes back as
// ErrNotFound.
func (service Service) Resolve(ctx context.Context, shortName string) (db.Link, error) {
	link, err := service.store.GetLinkByShortName(ctx, shortName)

	if errors.Is(err, sql.ErrNoRows) {
		return db.Link{}, fmt.Errorf("short name %q: %w", shortName, ErrNotFound)
	}

	if err != nil {
		return db.Link{}, fmt.Errorf("resolve short name %q: %w", shortName, err)
	}

	return link, nil
}
