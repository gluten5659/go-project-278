package api

import (
	"code/internal/db"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"go.step.sm/crypto/randutil"
)

const linkIDBitSize = 64

type createLinkRequest struct {
	OriginalURL string `binding:"required" json:"original_url"`
	ShortName   string `json:"short_name"`
}

func (request createLinkRequest) createLinkParameters() db.CreateLinkParams {
	return db.CreateLinkParams{
		OriginalUrl: request.OriginalURL,
		ShortName:   request.ShortName,
	}
}

type linksHandler struct {
	queries db.Querier
}

func (handler linksHandler) index(ginContext *gin.Context) {
	links, err := handler.queries.GetLinks(ginContext.Request.Context())
	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	if links == nil {
		links = []db.Link{}
	}

	ginContext.JSON(http.StatusOK, links)
}

const (
	defaultShortURLsize = 8

	shortNameAttempts = 3

	uniqueViolationCode = "23505"
	shortNameIndexName  = "idx_short_name"
)

var errShortNameAttemptsExhausted = errors.New("ran out of short name attempts")

func (handler linksHandler) create(ginContext *gin.Context) {
	var request createLinkRequest

	err := ginContext.ShouldBindJSON(&request)
	if err != nil {
		respondWithError(ginContext, http.StatusBadRequest, err)

		return
	}

	if request.ShortName == "" {
		handler.createWithGeneratedShortName(ginContext, request)

		return
	}

	handler.createWithRequestedShortName(ginContext, request)
}

func (handler linksHandler) createWithRequestedShortName(
	ginContext *gin.Context,
	request createLinkRequest,
) {
	link, err := handler.queries.CreateLink(
		ginContext.Request.Context(),
		request.createLinkParameters(),
	)

	if isShortNameTaken(err) {
		respondWithStatus(ginContext, http.StatusConflict)

		return
	}

	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	ginContext.JSON(http.StatusCreated, link)
}

func (handler linksHandler) createWithGeneratedShortName(
	ginContext *gin.Context,
	request createLinkRequest,
) {
	for range shortNameAttempts {
		generatedShortName, err := randutil.Alphabet(defaultShortURLsize)
		if err != nil {
			respondWithError(ginContext, http.StatusInternalServerError, err)

			return
		}

		request.ShortName = generatedShortName

		link, err := handler.queries.CreateLink(
			ginContext.Request.Context(),
			request.createLinkParameters(),
		)

		if isShortNameTaken(err) {
			continue
		}

		if err != nil {
			respondWithError(ginContext, http.StatusInternalServerError, err)

			return
		}

		ginContext.JSON(http.StatusCreated, link)

		return
	}

	respondWithError(ginContext, http.StatusInternalServerError, errShortNameAttemptsExhausted)
}

func isShortNameTaken(err error) bool {
	var pgError *pgconn.PgError

	return errors.As(err, &pgError) &&
		pgError.Code == uniqueViolationCode &&
		pgError.ConstraintName == shortNameIndexName
}

func (handler linksHandler) show(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithError(ginContext, http.StatusBadRequest, err)

		return
	}

	link, err := handler.queries.GetLinkById(ginContext.Request.Context(), linkID)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithStatus(ginContext, http.StatusNotFound)

		return
	}

	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	ginContext.JSON(http.StatusOK, link)
}

func (handler linksHandler) destroy(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithError(ginContext, http.StatusBadRequest, err)

		return
	}

	deletedCount, err := handler.queries.DeleteLink(ginContext.Request.Context(), linkID)
	if err != nil {
		respondWithError(ginContext, http.StatusInternalServerError, err)

		return
	}

	if deletedCount == 0 {
		respondWithStatus(ginContext, http.StatusNotFound)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

func parseLinkID(rawLinkID string) (int64, error) {
	linkID, err := strconv.ParseInt(rawLinkID, 10, linkIDBitSize)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseInt: %w", err)
	}

	return linkID, nil
}

func respondWithError(ginContext *gin.Context, status int, err error) {
	_ = ginContext.Error(err)

	respondWithStatus(ginContext, status)
}

func respondWithStatus(ginContext *gin.Context, status int) {
	ginContext.JSON(status, nil)
}
