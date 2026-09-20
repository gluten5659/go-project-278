package main

import (
	"code/internal/app"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const (
	exitSuccess = 0
	exitFailure = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := app.Run(ctx)
	if err != nil {
		log.Print(err)

		return exitFailure
	}

	return exitSuccess
}
