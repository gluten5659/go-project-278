package api

import (
	"code/internal/db"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(queries db.Querier) *gin.Engine {
	ginEngine := gin.Default()

	ginEngine.GET("/ping", pong)

	links := linksHandler{queries: queries}

	ginEngine.GET("/api/links", links.index)
	ginEngine.POST("/api/links", links.create)
	ginEngine.GET("/api/links/:id", links.show)
	ginEngine.DELETE("/api/links/:id", links.destroy)

	return ginEngine
}

func pong(ginContext *gin.Context) {
	ginContext.JSON(http.StatusOK, gin.H{"message": "pong"})
}
