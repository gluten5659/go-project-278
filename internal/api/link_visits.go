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
	queries db.Querier
}

func (handler linkVisitsHandler) index(ginContext *gin.Context) {
	totalVisits, err := handler.queries.CountLinkVisits(ginContext.Request.Context())
	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	bounds, err := parsePageRange(ginContext.Query("range"), totalVisits)
	if err != nil {
		respondWithError(ginContext, http.StatusBadRequest, err)

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
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	if visits == nil {
		visits = []db.LinkVisit{}
	}

	ginContext.Header("Content-Range", bounds.contentRange(linkVisitsResource, totalVisits))
	ginContext.JSON(http.StatusOK, visits)
}

func (handler linkVisitsHandler) redirect(ginContext *gin.Context) {
	link, err := handler.queries.GetLinkByshortName(
		ginContext.Request.Context(),
		ginContext.Param("code"),
	)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithStatus(ginContext, http.StatusNotFound)

		return
	}

	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	_, err = handler.queries.CreateLinkVisit(
		ginContext.Request.Context(),
		db.CreateLinkVisitParams{
			LinkID:    link.ID,
			Ip:        ginContext.ClientIP(),
			UserAgent: ginContext.Request.UserAgent(),
			Referer:   ginContext.Request.Referer(),
			Status:    redirectStatus,
		},
	)
	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	ginContext.Redirect(redirectStatus, link.OriginalUrl)
}
