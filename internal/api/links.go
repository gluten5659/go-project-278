package api

import (
	"code/internal/db"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	LinksResource = "links"

	linkIDBitSize = 64

	ShortNameField = "short_name"

	ShortNameTakenMessage = "short name already in use"
)

type createLinkRequest struct {
	OriginalURL string `json:"original_url" validate:"required,http_url"`
	ShortName   string `json:"short_name"   validate:"omitempty,min=3,max=32"`
}

func (request createLinkRequest) createLinkParameters() db.CreateLinkParams {
	return db.CreateLinkParams{
		OriginalURL: request.OriginalURL,
		ShortName:   request.ShortName,
	}
}

type updateLinkRequest struct {
	OriginalURL string `json:"original_url" validate:"required,http_url"`
	ShortName   string `json:"short_name"   validate:"required,min=3,max=32"`
}

func (request updateLinkRequest) updateLinkParameters(linkID int64) db.UpdateLinkParams {
	return db.UpdateLinkParams{
		ID:          linkID,
		OriginalURL: request.OriginalURL,
		ShortName:   request.ShortName,
	}
}

type linkResponse struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortName   string    `json:"short_name"`
	ShortURL    string    `json:"short_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func newLinkResponse(link db.Link, baseURL string) linkResponse {
	return linkResponse{
		ID:          link.ID,
		OriginalURL: link.OriginalURL,
		ShortName:   link.ShortName,
		ShortURL:    baseURL + RedirectPrefix + link.ShortName,
		CreatedAt:   link.CreatedAt,
	}
}

type linksHandler struct {
	queries  db.Querier
	validate *validator.Validate
	baseURL  string
}

func (handler linksHandler) newLinkResponses(links []db.Link) []linkResponse {
	responses := make([]linkResponse, 0, len(links))

	for _, link := range links {
		responses = append(responses, newLinkResponse(link, handler.baseURL))
	}

	return responses
}

func bindRequest(ginContext *gin.Context, validate *validator.Validate, request any) bool {
	err := ginContext.ShouldBindJSON(request)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return false
	}

	err = validate.Struct(request)
	if err != nil {
		respondWithValidationError(ginContext, err)

		return false
	}

	return true
}

func (handler linksHandler) list(ginContext *gin.Context) {
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

	ginContext.Header("Content-Range", bounds.contentRange(LinksResource, totalLinks))
	ginContext.JSON(http.StatusOK, handler.newLinkResponses(links))
}

const (
	ShortNameLength = 8

	shortNameAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	ShortNameAttempts = 3

	UniqueViolationCode = "23505"
	ShortNameIndex      = "idx_short_name"
)

var errShortNameAttemptsExhausted = errors.New("ran out of short name attempts")

func (handler linksHandler) create(ginContext *gin.Context) {
	var request createLinkRequest

	isValid := bindRequest(ginContext, handler.validate, &request)
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
	request createLinkRequest,
) {
	link, err := handler.queries.CreateLink(
		ginContext.Request.Context(),
		request.createLinkParameters(),
	)

	if isShortNameTaken(err) {
		respondWithFieldError(ginContext, err, ShortNameField, ShortNameTakenMessage)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusCreated, newLinkResponse(link, handler.baseURL))
}

func (handler linksHandler) createWithGeneratedShortName(
	ginContext *gin.Context,
	request createLinkRequest,
) {
	for range ShortNameAttempts {
		generatedShortName, err := generateShortName()
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

		ginContext.JSON(http.StatusCreated, newLinkResponse(link, handler.baseURL))

		return
	}

	respondWithUnavailable(ginContext, errShortNameAttemptsExhausted)
}

func generateShortName() (string, error) {
	name := make([]byte, ShortNameLength)
	alphabetSize := big.NewInt(int64(len(shortNameAlphabet)))

	for index := range name {
		position, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("read random source: %w", err)
		}

		name[index] = shortNameAlphabet[position.Int64()]
	}

	return string(name), nil
}

func isShortNameTaken(err error) bool {
	var pgError *pgconn.PgError

	return errors.As(err, &pgError) &&
		pgError.Code == UniqueViolationCode &&
		pgError.ConstraintName == ShortNameIndex
}

func (handler linksHandler) show(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	link, err := handler.queries.GetLinkById(ginContext.Request.Context(), linkID)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithNotFound(ginContext)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusOK, newLinkResponse(link, handler.baseURL))
}

func (handler linksHandler) update(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	var request updateLinkRequest

	isValid := bindRequest(ginContext, handler.validate, &request)
	if !isValid {
		return
	}

	link, err := handler.queries.UpdateLink(
		ginContext.Request.Context(),
		request.updateLinkParameters(linkID),
	)

	if errors.Is(err, sql.ErrNoRows) {
		respondWithNotFound(ginContext)

		return
	}

	if isShortNameTaken(err) {
		respondWithFieldError(ginContext, err, ShortNameField, ShortNameTakenMessage)

		return
	}

	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusOK, newLinkResponse(link, handler.baseURL))
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
		respondWithNotFound(ginContext)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

func parseLinkID(rawLinkID string) (int64, error) {
	linkID, err := strconv.ParseInt(rawLinkID, 10, linkIDBitSize)
	if err != nil {
		return 0, fmt.Errorf("parse link id %q: %w", rawLinkID, err)
	}

	return linkID, nil
}
