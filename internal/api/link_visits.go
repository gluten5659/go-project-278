package api

import (
	"code/internal/db"
	"code/internal/links"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	LinkVisitsResource = "link_visits"

	redirectStatus = http.StatusFound

	RedirectPrefix = "/r/"
)

type linkVisitsHandler struct {
	linkService links.Service
	reportError ErrorReporter
}

func (handler linkVisitsHandler) list(ginContext *gin.Context) {
	ctx := ginContext.Request.Context()

	totalVisits, err := handler.linkService.CountVisits(ctx)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	bounds, err := parsePageRange(ginContext.Query("range"), totalVisits)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	visits, err := handler.linkService.ListVisits(ctx, bounds.offset, bounds.size)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	contentRange := bounds.contentRange(LinkVisitsResource, int64(len(visits)), totalVisits)

	if visits == nil {
		visits = []db.LinkVisit{}
	}

	ginContext.Header("Content-Range", contentRange)
	ginContext.JSON(http.StatusOK, visits)
}

func (handler linkVisitsHandler) redirect(ginContext *gin.Context) {
	ctx := ginContext.Request.Context()

	link, err := handler.linkService.Resolve(ctx, ginContext.Param("code"))
	if err != nil {
		respondWithLinkError(ginContext, err)

		return
	}

	err = handler.linkService.RecordVisit(ctx, newVisit(ginContext, link.ID))
	if err != nil {
		handler.reportError(ginContext.Request, err)
	}

	ginContext.Redirect(redirectStatus, link.OriginalURL)
}

func newVisit(ginContext *gin.Context, linkID int64) links.Visit {
	return links.Visit{
		LinkID:    linkID,
		IP:        ginContext.ClientIP(),
		UserAgent: ginContext.Request.UserAgent(),
		Referer:   ginContext.Request.Referer(),
		Status:    redirectStatus,
	}
}
