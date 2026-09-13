package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type healthHandler struct {
	database DatabasePinger
}

func (handler healthHandler) check(ginContext *gin.Context) {
	err := handler.database.PingContext(ginContext.Request.Context())
	if err != nil {
		respondWithUnavailable(ginContext, err)

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "pong"})
}
