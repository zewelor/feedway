package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zewelor/feedway/internal/config"
	"github.com/zewelor/feedway/internal/database"
	"github.com/zewelor/feedway/internal/httpserver"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "feedway: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configuration, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, configuration.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Prepare(ctx, pool); err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return httpserver.Run(ctx, configuration.APIToken, pool, logger)
}
