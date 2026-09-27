package links

import (
	"code/internal/db"
	"context"
	"fmt"
)

// Visit is one hit of a short link as the redirect saw it.
type Visit struct {
	LinkID    int64
	IP        string
	UserAgent string
	Referer   string
	Status    int32
}

// RecordVisit stores the visit. The caller decides what a failure means, because
// the statistics are not what the visitor is waiting for.
func (service Service) RecordVisit(ctx context.Context, visit Visit) error {
	_, err := service.store.CreateLinkVisit(ctx, db.CreateLinkVisitParams{
		LinkID:    visit.LinkID,
		IP:        visit.IP,
		UserAgent: visit.UserAgent,
		Referer:   visit.Referer,
		Status:    visit.Status,
	})
	if err != nil {
		return fmt.Errorf("record visit to link %d: %w", visit.LinkID, err)
	}

	return nil
}

// CountVisits reports how many visits are stored, which is what a caller needs
// to describe a page of them.
func (service Service) CountVisits(ctx context.Context) (int64, error) {
	totalVisits, err := service.store.CountLinkVisits(ctx)
	if err != nil {
		return 0, fmt.Errorf("count visits: %w", err)
	}

	return totalVisits, nil
}

// ListVisits returns at most size visits, starting at offset and ordered by
// identifier.
func (service Service) ListVisits(
	ctx context.Context,
	offset, size int64,
) ([]db.LinkVisit, error) {
	visits, err := service.store.GetLinkVisits(ctx, db.GetLinkVisitsParams{
		PageOffset: offset,
		PageSize:   size,
	})
	if err != nil {
		return nil, fmt.Errorf("list %d visits from %d: %w", size, offset, err)
	}

	return visits, nil
}
