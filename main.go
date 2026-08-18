package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

const sentryFlushTimeout = 2 * time.Second

var errMissingSentryDSN = errors.New("SENTRY_DSN must be set to a non-empty value")

func newRouter() *gin.Engine {
	ginEngine := gin.Default()

	ginEngine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	return ginEngine
}

func run() error {
	sentryDSN, isSet := os.LookupEnv("SENTRY_DSN")
	if !isSet || sentryDSN == "" {
		return errMissingSentryDSN
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

	err = newRouter().Run()
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
