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
	shutdownTimeout     = 3 * time.Second

	maxOpenConnections    = 10
	maxIdleConnections    = 5
	connectionMaxLifetime = 30 * time.Minute
	connectionMaxIdleTime = 5 * time.Minute

	defaultAllowedOrigin = "http://localhost:5173"
	defaultBaseURL       = "http://localhost:8080"

	serverAddress = ":8080"
)

func environmentVariable(name string) (string, bool) {
	value, isSet := os.LookupEnv(name)

	return value, isSet && value != ""
}

func allowedOrigins() []string {
	rawOrigins, isSet := environmentVariable("CORS_ALLOWED_ORIGINS")
	if !isSet {
		return []string{defaultAllowedOrigin}
	}

	origins := strings.Split(rawOrigins, ",")
	for index, origin := range origins {
		origins[index] = strings.TrimSpace(origin)
	}

	return origins
}

func baseURL() string {
	rawBaseURL, isSet := environmentVariable("BASE_URL")
	if !isSet {
		return defaultBaseURL
	}

	return strings.TrimSuffix(strings.TrimSpace(rawBaseURL), "/")
}

var errMissingDatabaseDSN = errors.New(
	"DATABASE_DSN or DATABASE_URL must be set to a non-empty value",
)

func requireDatabaseDSN() (string, error) {
	databaseDSN, isSet := environmentVariable("DATABASE_DSN")
	if isSet {
		return databaseDSN, nil
	}

	databaseDSN, isSet = environmentVariable("DATABASE_URL")
	if isSet {
		return databaseDSN, nil
	}

	return "", errMissingDatabaseDSN
}

func startSentry() error {
	sentryDSN, isSet := environmentVariable("SENTRY_DSN")
	if !isSet {
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

func Run(ctx context.Context) error {
	databaseDSN, err := requireDatabaseDSN()
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

	pingCtx, cancelPing := context.WithTimeout(ctx, databasePingTimeout)
	defer cancelPing()

	err = conn.PingContext(pingCtx)
	if err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	router := api.NewRouter(api.Config{
		Queries:        db.New(conn),
		Database:       conn,
		AllowedOrigins: allowedOrigins(),
		BaseURL:        baseURL(),
		ReportError:    reportToSentry,
	})

	return serve(ctx, router)
}

func serve(ctx context.Context, router http.Handler) error {
	server := &http.Server{
		Addr:              serverAddress,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverFailed := make(chan error, 1)

	go func() {
		serverFailed <- server.ListenAndServe()
	}()

	select {
	case err := <-serverFailed:
		return fmt.Errorf("run server on %s: %w", serverAddress, err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}

	return nil
}
