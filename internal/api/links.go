package api

import (
	"code/internal/db"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"go.step.sm/crypto/randutil"
)

const (
	linksResource = "links"

	linkIDBitSize = 64

	shortNameField = "short_name"

	shortNameTakenMessage    = "short name already in use"
	shortNameRequiredMessage = "short name is required"
)

type linkRequest struct {
	OriginalURL string `json:"original_url" validate:"required,url"`
	ShortName   string `json:"short_name"   validate:"omitempty,min=3,max=32"`
}

func (request linkRequest) createLinkParameters() db.CreateLinkParams {
	return db.CreateLinkParams{
		OriginalUrl: request.OriginalURL,
		ShortName:   request.ShortName,
	}
}

func (request linkRequest) updateLinkParameters(linkID int64) db.UpdateLinkParams {
	return db.UpdateLinkParams{
		ID:          linkID,
		OriginalUrl: request.OriginalURL,
		ShortName:   request.ShortName,
	}
}

type linksHandler struct {
	queries  db.Querier
	validate *validator.Validate
}

func (handler linksHandler) bindLinkRequest(ginContext *gin.Context) (linkRequest, bool) {
	var request linkRequest

	err := ginContext.ShouldBindJSON(&request)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return request, false
	}

	err = handler.validate.Struct(request)
	if err != nil {
		respondWithValidationError(ginContext, err)

		return request, false
	}

	return request, true
}

func (handler linksHandler) index(ginContext *gin.Context) {
	totalLinks, err := handler.queries.CountLinks(ginContext.Request.Context())
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	bounds, err := parsePageRange(ginContext.Query("range"), totalLinks)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	links, err := handler.queries.GetLinks(
		ginContext.Request.Context(),
		db.GetLinksParams{
			PageOffset: bounds.firstIndex,
			PageSize:   bounds.pageSize(),
		},
	)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	if links == nil {
		links = []db.Link{}
	}

	ginContext.Header("Content-Range", bounds.contentRange(linksResource, totalLinks))
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
	request, isValid := handler.bindLinkRequest(ginContext)
	if !isValid {
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
	request linkRequest,
) {
	link, err := handler.queries.CreateLink(
		ginContext.Request.Context(),
		request.createLinkParameters(),
	)

	if isShortNameTaken(err) {
		respondWithFieldError(ginContext, err, shortNameField, shortNameTakenMessage)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusCreated, link)
}

func (handler linksHandler) createWithGeneratedShortName(
	ginContext *gin.Context,
	request linkRequest,
) {
	for range shortNameAttempts {
		generatedShortName, err := randutil.Alphabet(defaultShortURLsize)
		if err != nil {
			respondWithInternalError(ginContext, err)

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
			respondWithInternalError(ginContext, err)

			return
		}

		ginContext.JSON(http.StatusCreated, link)

		return
	}

	respondWithInternalError(ginContext, errShortNameAttemptsExhausted)
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
		respondWithInvalidRequest(ginContext, err)

		return
	}

	link, err := handler.queries.GetLinkById(ginContext.Request.Context(), linkID)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithStatus(ginContext, http.StatusNotFound)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusOK, link)
}

var errShortNameRequired = errors.New(shortNameRequiredMessage)

func (handler linksHandler) update(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	request, isValid := handler.bindLinkRequest(ginContext)
	if !isValid {
		return
	}

	if request.ShortName == "" {
		respondWithFieldError(
			ginContext,
			errShortNameRequired,
			shortNameField,
			shortNameRequiredMessage,
		)

		return
	}

	link, err := handler.queries.UpdateLink(
		ginContext.Request.Context(),
		request.updateLinkParameters(linkID),
	)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithStatus(ginContext, http.StatusNotFound)

		return
	}

	if isShortNameTaken(err) {
		respondWithFieldError(ginContext, err, shortNameField, shortNameTakenMessage)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusOK, link)
}

func (handler linksHandler) destroy(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	deletedCount, err := handler.queries.DeleteLink(ginContext.Request.Context(), linkID)
	if err != nil {
		respondWithInternalError(ginContext, err)

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
