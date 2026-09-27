package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorReport struct {
	Method string
	Path   string
}

type ErrorReporter func(report ErrorReport, err error)

func newErrorReport(ginContext *gin.Context) ErrorReport {
	return ErrorReport{
		Method: ginContext.Request.Method,
		Path:   ginContext.Request.URL.Path,
	}
}

var errPanic = errors.New("panic")

func reportServerErrors(reportError ErrorReporter) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		defer reportRecoveredPanic(ginContext, reportError)

		ginContext.Next()

		if ginContext.Writer.Status() < http.StatusInternalServerError {
			return
		}

		for _, ginError := range ginContext.Errors {
			reportError(newErrorReport(ginContext), ginError.Err)
		}
	}
}

func reportRecoveredPanic(ginContext *gin.Context, reportError ErrorReporter) {
	recovered := recover()
	if recovered == nil {
		return
	}

	reportError(newErrorReport(ginContext), panicError(recovered))

	panic(recovered)
}

func panicError(recovered any) error {
	err, isError := recovered.(error)
	if isError {
		return fmt.Errorf("%w: %w", errPanic, err)
	}

	return fmt.Errorf("%w: %v", errPanic, recovered)
}
