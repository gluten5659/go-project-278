package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"code/internal/db"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const sentryFlushTimeout = 2 * time.Second

var (
	errMissingSentryDSN   = errors.New("SENTRY_DSN must be set to a non-empty value")
	errMissongDatabaseDSN = errors.New("DATABASE_DSN must be set to a non-empty value ")
)

type PostBody struct {
	OriginalUrl string `json:"original_url"`
	ShortName   string `json:"short_name"`
}

func newRouter(queries *db.Queries) *gin.Engine {
	ginEngine := gin.Default()

	ginEngine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	ginEngine.GET("/api/links", func(c *gin.Context) {
		links, err := queries.GetLinks(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		if len(links) == 0 {
			links = []db.Link{}
		}
		c.JSON(http.StatusOK, links)
	})

	ginEngine.POST("/api/links", func(c *gin.Context) {
		var req PostBody
		c.ShouldBindJSON(&req)
		args := db.CreateLinkParams{
			req.OriginalUrl,
			req.ShortName,
		}
		link, err := queries.CreateLink(c.Request.Context(), args)
		if err != nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusCreated, link)
	})

	return ginEngine
}

func run() error {
	sentryDSN, isSet := os.LookupEnv("SENTRY_DSN")
	if !isSet || sentryDSN == "" {
		return errMissingSentryDSN
	}

	databaseDSN, isSet := os.LookupEnv("DATABASE_DSN")
	if !isSet || databaseDSN == "" {
		return errMissongDatabaseDSN
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn: sentryDSN,

		SendDefaultPII: true,

		EnableTracing:        false,
		TracesSampleRate:     0,
		DisableLogs:          true,
		DisableMetrics:       true,
		DisableClientReports: true,
	})
	if err != nil {
		return fmt.Errorf("sentry.Init: %w", err)
	}

	defer sentry.Flush(sentryFlushTimeout)

	conn, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}

	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = conn.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("conn.PingContext: %w", err)
	}

	queries := db.New(conn)

	err = newRouter(queries).Run()
	if err != nil {
		return fmt.Errorf("failed to run server: %w", err)
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}
