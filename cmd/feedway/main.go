package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zewelor/feedway/internal/config"
	"github.com/zewelor/feedway/internal/httpserver"
)

func main() {
	configuration, err := config.Load(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "feedway: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := httpserver.Run(ctx, configuration, logger); err != nil {
		fmt.Fprintf(os.Stderr, "feedway: %v\n", err)
		os.Exit(1)
	}
}
