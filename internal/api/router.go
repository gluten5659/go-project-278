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
	ginEngine.TrustedPlatform = gin.PlatformCloudflare
	_ = ginEngine.SetTrustedProxies([]string{"127.0.0.1", "::1"})

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

	links := linksHandler{queries: queries, validate: newValidator()}
	linkVisits := linkVisitsHandler{queries: queries}

	ginEngine.GET("/r/:code", linkVisits.redirect)

	ginEngine.GET("/api/link_visits", linkVisits.index)

	ginEngine.GET("/api/links", links.index)
	ginEngine.POST("/api/links", links.create)
	ginEngine.GET("/api/links/:id", links.show)
	ginEngine.PUT("/api/links/:id", links.update)
	ginEngine.DELETE("/api/links/:id", links.destroy)

	return ginEngine
}

func pong(ginContext *gin.Context) {
	ginContext.JSON(http.StatusOK, gin.H{"message": "pong"})
}
