package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	invalidRequestMessage = "invalid request"
	notFoundMessage       = "not found"
	internalErrorMessage  = "internal server error"
	unavailableMessage    = "service unavailable"
)

func respondWithValidationError(ginContext *gin.Context, err error) {
	var validationErrors validator.ValidationErrors

	if !errors.As(err, &validationErrors) {
		respondWithInternalError(ginContext, err)

		return
	}

	_ = ginContext.Error(err)

	messagesByField := make(gin.H, len(validationErrors))

	for _, fieldError := range validationErrors {
		messagesByField[fieldError.Field()] = fieldError.Error()
	}

	ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"errors": messagesByField})
}

func respondWithFieldError(ginContext *gin.Context, cause error, field string, message string) {
	_ = ginContext.Error(cause)

	ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"errors": gin.H{field: message}})
}

func respondWithInvalidRequest(ginContext *gin.Context, err error) {
	_ = ginContext.Error(err)

	respondWithMessage(ginContext, http.StatusBadRequest, invalidRequestMessage)
}

func respondWithInternalError(ginContext *gin.Context, err error) {
	_ = ginContext.Error(err)

	respondWithMessage(ginContext, http.StatusInternalServerError, internalErrorMessage)
}

func respondWithUnavailable(ginContext *gin.Context, err error) {
	_ = ginContext.Error(err)

	respondWithMessage(ginContext, http.StatusServiceUnavailable, unavailableMessage)
}

func respondWithNotFound(ginContext *gin.Context) {
	respondWithMessage(ginContext, http.StatusNotFound, notFoundMessage)
}

func respondWithMessage(ginContext *gin.Context, status int, message string) {
	ginContext.JSON(status, gin.H{"error": message})
}
