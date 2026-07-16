package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zewelor/feedway/internal/database"
)

const cleanupInterval = 24 * time.Hour

func Run(
	ctx context.Context,
	pool *pgxpool.Pool,
	retentionDays int,
	logger *slog.Logger,
) {
	run(ctx, cleanupInterval, func(ctx context.Context) error {
		return database.DeleteExpiredEntries(ctx, pool, retentionDays)
	}, logger)
}

func run(
	ctx context.Context,
	interval time.Duration,
	cleanup func(context.Context) error,
	logger *slog.Logger,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := cleanup(ctx); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "retention cleanup failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
