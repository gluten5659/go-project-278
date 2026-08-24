package main

import (
	"code/internal/api"
	"code/internal/db"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	sentryFlushTimeout  = 2 * time.Second
	databasePingTimeout = 5 * time.Second
)

var errMissingEnvironmentVariable = errors.New("must be set to a non-empty value")

func requireEnvironmentVariable(name string) (string, error) {
	value, isSet := os.LookupEnv(name)
	if !isSet || value == "" {
		return "", fmt.Errorf("%s %w", name, errMissingEnvironmentVariable)
	}

	return value, nil
}

func run() error {
	sentryDSN, err := requireEnvironmentVariable("SENTRY_DSN")
	if err != nil {
		return err
	}

	databaseDSN, err := requireEnvironmentVariable("DATABASE_DSN")
	if err != nil {
		return err
	}

	err = sentry.Init(sentry.ClientOptions{
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

	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Printf("conn.Close: %v", closeErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), databasePingTimeout)
	defer cancel()

	err = conn.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("conn.PingContext: %w", err)
	}

	err = api.NewRouter(db.New(conn)).Run()
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
