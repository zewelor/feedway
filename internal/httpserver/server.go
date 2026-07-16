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
)

const (
	address         = ":8080"
	requestMaxBytes = 1 << 20
	shutdownTimeout = 15 * time.Second
)

func Run(ctx context.Context, apiToken string, pool *pgxpool.Pool, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	readiness := &readiness{
		database: pool,
	}
	server := &http.Server{
		Handler: newHandler(
			apiToken,
			readiness,
			func(ctx context.Context, values entry.Values) (bool, error) {
				return database.InsertEntry(ctx, pool, values)
			},
			logger,
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
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
