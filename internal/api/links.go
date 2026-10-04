package api

import (
	"code/internal/db"
	"code/internal/links"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Exported for the black-box tests, not as an API.
const (
	LinksResource = "links"

	linkIDBitSize = 64

	ShortNameField = "short_name"

	shortNameTakenMessage = "short name already in use"
)

type createLinkRequest struct {
	OriginalURL string `json:"original_url" validate:"required,http_url"`
	ShortName   string `json:"short_name"   validate:"omitempty,min=3,max=32,path_segment"`
}

type updateLinkRequest struct {
	OriginalURL string `json:"original_url" validate:"required,http_url"`
	ShortName   string `json:"short_name"   validate:"required,min=3,max=32,path_segment"`
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
	linkService links.Service
	validate    *validator.Validate
	baseURL     string
}

func (handler linksHandler) newLinkResponses(storedLinks []db.Link) []linkResponse {
	responses := make([]linkResponse, 0, len(storedLinks))

	for _, link := range storedLinks {
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
	ctx := ginContext.Request.Context()

	totalLinks, err := handler.linkService.Count(ctx)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	bounds, err := parsePageRange(ginContext.Query("range"), totalLinks)
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	storedLinks, err := handler.linkService.List(ctx, bounds.offset, bounds.size)
	if err != nil {
		respondWithInternalError(ginContext, err)

		return
	}

	contentRange := bounds.contentRange(LinksResource, int64(len(storedLinks)), totalLinks)

	ginContext.Header("Content-Range", contentRange)
	ginContext.JSON(http.StatusOK, handler.newLinkResponses(storedLinks))
}

func (handler linksHandler) create(ginContext *gin.Context) {
	var request createLinkRequest

	isValid := bindRequest(ginContext, handler.validate, &request)
	if !isValid {
		return
	}

	link, err := handler.linkService.Create(
		ginContext.Request.Context(),
		request.OriginalURL,
		request.ShortName,
	)
	if err != nil {
		respondWithLinkError(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusCreated, newLinkResponse(link, handler.baseURL))
}

func (handler linksHandler) show(ginContext *gin.Context) {
	linkID, err := parseLinkID(ginContext.Param("id"))
	if err != nil {
		respondWithInvalidRequest(ginContext, err)

		return
	}

	link, err := handler.linkService.Find(ginContext.Request.Context(), linkID)
	if err != nil {
		respondWithLinkError(ginContext, err)

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

	link, err := handler.linkService.Update(
		ginContext.Request.Context(),
		linkID,
		request.OriginalURL,
		request.ShortName,
	)
	if err != nil {
		respondWithLinkError(ginContext, err)

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

	err = handler.linkService.Delete(ginContext.Request.Context(), linkID)
	if err != nil {
		respondWithLinkError(ginContext, err)

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
