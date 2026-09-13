package api

import (
	"code/internal/db"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const corsPreflightMaxAge = 12 * time.Hour

type Config struct {
	Queries        db.Querier
	Database       DatabasePinger
	AllowedOrigins []string
	ReportError    ErrorReporter
}

func NewRouter(config Config) *gin.Engine {
	ginEngine := gin.Default()
	ginEngine.TrustedPlatform = gin.PlatformCloudflare
	_ = ginEngine.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	ginEngine.Use(reportServerErrors(config.ReportError))

	ginEngine.Use(cors.New(cors.Config{
		AllowOrigins: config.AllowedOrigins,
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

	health := healthHandler{database: config.Database}

	ginEngine.GET("/ping", health.check)

	links := linksHandler{queries: config.Queries, validate: newValidator()}
	linkVisits := linkVisitsHandler{queries: config.Queries, reportError: config.ReportError}

	ginEngine.GET("/r/:code", linkVisits.redirect)

	ginEngine.GET("/api/link_visits", linkVisits.list)

	ginEngine.GET("/api/links", links.list)
	ginEngine.POST("/api/links", links.create)
	ginEngine.GET("/api/links/:id", links.show)
	ginEngine.PUT("/api/links/:id", links.update)
	ginEngine.DELETE("/api/links/:id", links.destroy)

	return ginEngine
}
