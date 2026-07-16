package database

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const operationTimeout = 5 * time.Second

//go:embed schema.sql
var schema string

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return pool, nil
}

func Prepare(ctx context.Context, pool *pgxpool.Pool) error {
	prepareCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	if _, err := pool.Exec(prepareCtx, schema); err != nil {
		return fmt.Errorf("prepare database schema: %w", err)
	}

	return nil
}
