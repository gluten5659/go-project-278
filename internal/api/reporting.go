package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorReporter func(request *http.Request, err error)

var errPanic = errors.New("panic")

func reportServerErrors(reportError ErrorReporter) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		defer reportRecoveredPanic(ginContext, reportError)

		ginContext.Next()

		if ginContext.Writer.Status() < http.StatusInternalServerError {
			return
		}

		for _, ginError := range ginContext.Errors {
			reportError(ginContext.Request, ginError.Err)
		}
	}
}

func reportRecoveredPanic(ginContext *gin.Context, reportError ErrorReporter) {
	recovered := recover()
	if recovered == nil {
		return
	}

	reportError(ginContext.Request, panicError(recovered))

	panic(recovered)
}

func panicError(recovered any) error {
	err, isError := recovered.(error)
	if isError {
		return fmt.Errorf("%w: %w", errPanic, err)
	}

	return fmt.Errorf("%w: %v", errPanic, recovered)
}
