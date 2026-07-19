package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zewelor/feedway/internal/database"
	"github.com/zewelor/feedway/internal/entry"
	"github.com/zewelor/feedway/internal/jsonfeed"
)

const (
	maxHeaderBytes  = 8 << 10
	requestMaxBytes = 1 << 20
	shutdownTimeout = 15 * time.Second
)

type Config struct {
	Port     uint16
	APIToken string
	BaseURL  string
}

func Run(ctx context.Context, config Config, pool *pgxpool.Pool, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	readiness := &readiness{
		database: pool,
	}
	server := &http.Server{
		Handler: newHandler(
			config.APIToken,
			readiness,
			func(ctx context.Context, values entry.Values) (bool, error) {
				return database.InsertEntry(ctx, pool, values)
			},
			func(ctx context.Context, id string) (entry.Published, bool, error) {
				return database.GetEntry(ctx, pool, id)
			},
			func(ctx context.Context) ([]byte, error) {
				entries, err := database.ListEntries(ctx, pool)
				if err != nil {
					return nil, err
				}
				return jsonfeed.Marshal(entries, config.BaseURL, maxFeedBytes)
			},
			logger,
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	return serve(ctx, server, listener, readiness)
}

func serve(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
	readiness *readiness,
) error {
	serverError := make(chan error, 1)
	go func() {
		serverError <- server.Serve(listener)
	}()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	readiness.isShuttingDown.Store(true)

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP: %w", err)
	}

	err := <-serverError
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

type readiness struct {
	database       databasePinger
	isShuttingDown atomic.Bool
}
