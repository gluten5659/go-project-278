package links

import (
	"code/internal/db"
	"context"
	"fmt"
)

type Visit struct {
	LinkID    int64
	IP        string
	UserAgent string
	Referer   string
	Status    int32
}

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
