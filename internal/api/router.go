package api

import (
	"code/internal/db"
	"code/internal/links"
	"fmt"
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

func NewRouter(config Config) (*gin.Engine, error) {
	trustedProxies := []string{"127.0.0.1", "::1"}

	ginEngine := gin.Default()
	ginEngine.TrustedPlatform = gin.PlatformCloudflare

	err := ginEngine.SetTrustedProxies(trustedProxies)
	if err != nil {
		return nil, fmt.Errorf("trust proxies %v: %w", trustedProxies, err)
	}

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

	ginEngine.NoRoute(respondWithNotFound)

	health := healthHandler{database: config.Database}

	ginEngine.GET("/ping", health.check)

	linkService := links.NewService(config.Queries)

	linkHandler := linksHandler{
		linkService: linkService,
		validate:    newValidator(),
		baseURL:     config.BaseURL,
	}
	visitHandler := linkVisitsHandler{
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

	return ginEngine, nil
}
