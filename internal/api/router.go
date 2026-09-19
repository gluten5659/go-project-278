package api

import (
	"code/internal/db"
	"code/internal/links"
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
	BaseURL        string
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

	linkService := links.NewService(config.Queries)

	linkHandler := linksHandler{
		queries:     config.Queries,
		linkService: linkService,
		validate:    newValidator(),
		baseURL:     config.BaseURL,
	}
	visitHandler := linkVisitsHandler{
		queries:     config.Queries,
		linkService: linkService,
		reportError: config.ReportError,
	}

	ginEngine.GET(RedirectPrefix+":code", visitHandler.redirect)

	ginEngine.GET("/api/link_visits", visitHandler.list)

	ginEngine.GET("/api/links", linkHandler.list)
	ginEngine.POST("/api/links", linkHandler.create)
	ginEngine.GET("/api/links/:id", linkHandler.show)
	ginEngine.PUT("/api/links/:id", linkHandler.update)
	ginEngine.DELETE("/api/links/:id", linkHandler.destroy)

	return ginEngine
}
