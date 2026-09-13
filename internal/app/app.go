package app

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	sentryFlushTimeout  = 2 * time.Second
	databasePingTimeout = 5 * time.Second
	readHeaderTimeout   = 5 * time.Second

	maxOpenConnections    = 10
	maxIdleConnections    = 5
	connectionMaxLifetime = 30 * time.Minute
	connectionMaxIdleTime = 5 * time.Minute

	defaultAllowedOrigin = "http://localhost:5173"

	serverAddress = ":8080"
)

var errMissingEnvironmentVariable = errors.New("must be set to a non-empty value")

func requireEnvironmentVariable(name string) (string, error) {
	value, isSet := os.LookupEnv(name)
	if !isSet || value == "" {
		return "", fmt.Errorf("%s %w", name, errMissingEnvironmentVariable)
	}

	return value, nil
}

func allowedOrigins() []string {
	rawOrigins, isSet := os.LookupEnv("CORS_ALLOWED_ORIGINS")
	if !isSet || rawOrigins == "" {
		return []string{defaultAllowedOrigin}
	}

	origins := strings.Split(rawOrigins, ",")
	for index, origin := range origins {
		origins[index] = strings.TrimSpace(origin)
	}

	return origins
}

func startSentry() error {
	sentryDSN, isSet := os.LookupEnv("SENTRY_DSN")
	if !isSet || sentryDSN == "" {
		log.Print("SENTRY_DSN is not set, error reporting is disabled")

		return nil
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
		return fmt.Errorf("start sentry: %w", err)
	}

	return nil
}

func reportToSentry(request *http.Request, err error) {
	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetRequest(request)
	hub.CaptureException(err)
	hub.Flush(sentryFlushTimeout)
}

func Run() error {
	databaseDSN, err := requireEnvironmentVariable("DATABASE_DSN")
	if err != nil {
		return err
	}

	err = startSentry()
	if err != nil {
		return err
	}

	defer sentry.Flush(sentryFlushTimeout)

	conn, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	conn.SetMaxOpenConns(maxOpenConnections)
	conn.SetMaxIdleConns(maxIdleConnections)
	conn.SetConnMaxLifetime(connectionMaxLifetime)
	conn.SetConnMaxIdleTime(connectionMaxIdleTime)

	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Printf("close database: %v", closeErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), databasePingTimeout)
	defer cancel()

	err = conn.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	router := api.NewRouter(db.New(conn), allowedOrigins(), reportToSentry)

	return serve(router)
}

func serve(router http.Handler) error {
	server := &http.Server{
		Addr:              serverAddress,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	err := server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("run server on %s: %w", serverAddress, err)
	}

	return nil
}
