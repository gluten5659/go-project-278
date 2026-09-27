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
	"net/url"
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

	defaultServerAddress = ":8080"
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

func serverAddress() string {
	rawAddress, isSet := environmentVariable("HTTP_ADDR")
	if !isSet {
		return defaultServerAddress
	}

	return parseServerAddress(rawAddress)
}

func parseServerAddress(rawAddress string) string {
	address := strings.TrimSpace(rawAddress)

	if strings.Contains(address, ":") {
		return address
	}

	return ":" + address
}

var (
	errMissingBaseURL = errors.New("BASE_URL must be set to a non-empty value")
	errInvalidBaseURL = errors.New(
		"BASE_URL must be an http or https address with a host and nothing after it",
	)
)

func requireBaseURL() (string, error) {
	rawBaseURL, isSet := environmentVariable("BASE_URL")
	if !isSet {
		return "", errMissingBaseURL
	}

	return parseBaseURL(rawBaseURL)
}

func parseBaseURL(rawBaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return "", fmt.Errorf("parse BASE_URL %q: %w", rawBaseURL, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errInvalidBaseURL
	}

	if parsed.Hostname() == "" || parsed.User != nil {
		return "", errInvalidBaseURL
	}

	if strings.Trim(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errInvalidBaseURL
	}

	return parsed.Scheme + "://" + parsed.Host, nil
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

		SendDefaultPII: false,

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

func reportToSentry(report api.ErrorReport, err error) {
	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetTags(map[string]string{
		"method": report.Method,
		"path":   report.Path,
	})
	hub.CaptureException(err)
}

func Run(ctx context.Context) error {
	databaseDSN, err := requireDatabaseDSN()
	if err != nil {
		return err
	}

	baseURL, err := requireBaseURL()
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

	router, err := api.NewRouter(api.Config{
		Queries:        db.New(conn),
		Database:       conn,
		AllowedOrigins: allowedOrigins(),
		BaseURL:        baseURL,
		ReportError:    reportToSentry,
	})
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	return serve(ctx, router, serverAddress())
}

func serve(ctx context.Context, router http.Handler, address string) error {
	server := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverFailed := make(chan error, 1)

	go func() {
		serverFailed <- server.ListenAndServe()
	}()

	select {
	case err := <-serverFailed:
		return fmt.Errorf("run server on %s: %w", address, err)
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
