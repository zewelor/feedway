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
	"github.com/zewelor/feedway/internal/retention"
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

	pool, err := database.Open(ctx, database.Config{
		Host:     configuration.DBHost,
		Port:     configuration.DBPort,
		Name:     configuration.DBName,
		User:     configuration.DBUser,
		Password: configuration.DBPassword,
		SSLMode:  configuration.DBSSLMode,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Prepare(ctx, pool); err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	retentionCtx, cancelRetention := context.WithCancel(ctx)
	retentionDone := make(chan struct{})
	go func() {
		defer close(retentionDone)
		retention.Run(
			retentionCtx,
			pool,
			configuration.RetentionDays,
			logger,
		)
	}()

	err = httpserver.Run(ctx, httpserver.Config{
		Port:     configuration.HTTPPort,
		APIToken: configuration.APIToken,
		BaseURL:  configuration.BaseURL,
	}, pool, logger)
	cancelRetention()
	<-retentionDone

	return err
}
