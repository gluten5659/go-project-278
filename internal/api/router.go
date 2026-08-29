package api

import (
	"code/internal/db"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const corsPreflightMaxAge = 12 * time.Hour

func NewRouter(queries db.Querier, allowedOrigins []string) *gin.Engine {
	ginEngine := gin.Default()

	ginEngine.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Range"},
		AllowCredentials: false,
		MaxAge:           corsPreflightMaxAge,
	}))

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
