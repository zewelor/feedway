package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/zewelor/feedway/internal/config"
)

const (
	address         = ":8080"
	requestMaxBytes = 1 << 20
	shutdownTimeout = 15 * time.Second
)

func Run(ctx context.Context, configuration config.Config, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	server := &http.Server{
		Handler:           newHandler(configuration.APIToken, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return serve(ctx, server, listener)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
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
