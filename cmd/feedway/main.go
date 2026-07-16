package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zewelor/feedway/internal/cli"
	"github.com/zewelor/feedway/internal/config"
	"github.com/zewelor/feedway/internal/httpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	os.Exit(cli.Run(
		os.Args[1:],
		os.LookupEnv,
		os.Stderr,
		func(configuration config.Config) error {
			return httpserver.Run(ctx, configuration, logger)
		},
		func(config.Config) error {
			return errors.New("migrate is not implemented")
		},
	))
}
