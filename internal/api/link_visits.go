package api

import (
	"code/internal/db"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	linkVisitsResource = "link_visits"

	redirectStatus = http.StatusFound
)

type linkVisitsHandler struct {
	queries     db.Querier
	reportError ErrorReporter
}

func (handler linkVisitsHandler) index(ginContext *gin.Context) {
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

	ginContext.Header("Content-Range", bounds.contentRange(linkVisitsResource, totalVisits))
	ginContext.JSON(http.StatusOK, visits)
}

func (handler linkVisitsHandler) redirect(ginContext *gin.Context) {
	link, err := handler.queries.GetLinkByShortName(
		ginContext.Request.Context(),
		ginContext.Param("code"),
	)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithNotFound(ginContext)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	handler.recordVisit(ginContext, link.ID)

	ginContext.Redirect(redirectStatus, link.OriginalURL)
}

func (handler linkVisitsHandler) recordVisit(ginContext *gin.Context, linkID int64) {
	_, err := handler.queries.CreateLinkVisit(
		ginContext.Request.Context(),
		db.CreateLinkVisitParams{
			LinkID:    linkID,
			IP:        ginContext.ClientIP(),
			UserAgent: ginContext.Request.UserAgent(),
			Referer:   ginContext.Request.Referer(),
			Status:    redirectStatus,
		},
	)
	if err != nil {
		handler.reportError(ginContext.Request, err)
	}
}
