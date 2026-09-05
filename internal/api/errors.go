package api

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const invalidRequestMessage = "invalid request"

func newValidator() *validator.Validate {
	validate := validator.New()

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")

		if jsonName == "-" {
			return ""
		}

		return jsonName
	})

	return validate
}

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

	ginContext.JSON(http.StatusBadRequest, gin.H{"error": invalidRequestMessage})
}

func respondWithInternalError(ginContext *gin.Context, err error) {
	_ = ginContext.Error(err)

	respondWithStatus(ginContext, http.StatusInternalServerError)
}

func respondWithStatus(ginContext *gin.Context, status int) {
	ginContext.JSON(status, nil)
}
