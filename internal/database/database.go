package database

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zewelor/feedway/internal/entry"
)

const operationTimeout = 5 * time.Second

//go:embed schema.sql
var schema string

type Config struct {
	Host     string
	Port     uint16
	Name     string
	User     string
	Password string
}

func Open(ctx context.Context, database Config) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.ConnConfig.Host = database.Host
	config.ConnConfig.Port = database.Port
	config.ConnConfig.Database = database.Name
	config.ConnConfig.User = database.User
	config.ConnConfig.Password = database.Password
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

func InsertEntry(ctx context.Context, pool *pgxpool.Pool, values entry.Values) (bool, error) {
	insertCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	tag, err := pool.Exec(
		insertCtx,
		`
			INSERT INTO entries (id, title, content_html)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO NOTHING
		`,
		values.ID,
		values.Title,
		values.ContentHTML,
	)
	if err != nil {
		return false, fmt.Errorf("insert entry: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}

func GetEntry(ctx context.Context, pool *pgxpool.Pool, id string) (entry.Published, bool, error) {
	getCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	var published entry.Published
	err := pool.QueryRow(
		getCtx,
		`
			SELECT id, title, content_html, created_at
			FROM entries
			WHERE id = $1
		`,
		id,
	).Scan(
		&published.ID,
		&published.Title,
		&published.ContentHTML,
		&published.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry.Published{}, false, nil
	}
	if err != nil {
		return entry.Published{}, false, fmt.Errorf("get entry: %w", err)
	}

	return published, true, nil
}

func ListEntries(ctx context.Context, pool *pgxpool.Pool) ([]entry.Published, error) {
	listCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	rows, err := pool.Query(
		listCtx,
		`
			SELECT id, title, content_html, created_at
			FROM entries
			ORDER BY created_at DESC, id DESC
			LIMIT 100
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	entries := make([]entry.Published, 0)
	for rows.Next() {
		var published entry.Published
		if err := rows.Scan(
			&published.ID,
			&published.Title,
			&published.ContentHTML,
			&published.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, published)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries: %w", err)
	}

	return entries, nil
}

func DeleteExpiredEntries(ctx context.Context, pool *pgxpool.Pool, retentionDays int) error {
	deleteCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	if _, err := pool.Exec(
		deleteCtx,
		"DELETE FROM entries WHERE created_at < now() - $1 * interval '1 day'",
		retentionDays,
	); err != nil {
		return fmt.Errorf("delete expired entries: %w", err)
	}

	return nil
}
