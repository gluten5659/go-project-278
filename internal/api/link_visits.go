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
	queries     db.Querier
	linkService links.Service
	reportError ErrorReporter
}

func (handler linkVisitsHandler) list(ginContext *gin.Context) {
	totalVisits, err := handler.queries.CountLinkVisits(ginContext.Request.Context())
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	bounds, err := parsePageRange(ginContext.Query("range"), totalVisits)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	visits, err := handler.queries.GetLinkVisits(
		ginContext.Request.Context(),
		db.GetLinkVisitsParams{
			PageOffset: bounds.firstIndex,
			PageSize:   bounds.pageSize(),
		},
	)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	if visits == nil {
		visits = []db.LinkVisit{}
	}

	ginContext.Header("Content-Range", bounds.contentRange(LinkVisitsResource, totalVisits))
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
