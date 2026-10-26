package database

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
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
	SSLMode  string
}

func Open(ctx context.Context, database Config) (*pgxpool.Pool, error) {
	config, err := poolConfig(database)
	if err != nil {
		return nil, err
	}

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

func poolConfig(database Config) (*pgxpool.Config, error) {
	connectionURL := &url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(database.User, database.Password),
		Host:    net.JoinHostPort(database.Host, strconv.Itoa(int(database.Port))),
		Path:    "/" + database.Name,
		RawPath: "/" + url.PathEscape(database.Name),
	}
	query := connectionURL.Query()
	query.Set("sslmode", database.SSLMode)
	connectionURL.RawQuery = query.Encode()

	config, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.MaxConns = 4

	return config, nil
}

func Prepare(ctx context.Context, pool *pgxpool.Pool) error {
	prepareCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	err := pgx.BeginFunc(prepareCtx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(prepareCtx, schema)
		return err
	})
	if err != nil {
		return fmt.Errorf("prepare database schema: %w", err)
	}

	return nil
}

func InsertEntry(ctx context.Context, pool *pgxpool.Pool, values entry.Values) (string, bool, error) {
	insertCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var dataJSON *string
	if data := values.DataJSON.String(); data != "" {
		dataJSON = &data
	}

	var feed *string
	if values.Feed != nil {
		name := values.Feed.String()
		feed = &name
	}
	var (
		id      string
		created bool
	)
	err := pool.QueryRow(
		insertCtx,
		`
			INSERT INTO entries (content_hash, title, content_html, data_json, feed)
			VALUES ($1, $2, $3, $4, $5)
			-- Preserve stored values and resolve concurrent deletion or reassignment atomically.
			ON CONFLICT (feed, content_hash)
			DO UPDATE SET content_hash = entries.content_hash
			RETURNING id, OLD.id IS NULL AS created
		`,
		values.ContentHash,
		values.Title,
		values.ContentHTML.String(),
		dataJSON,
		feed,
	).Scan(&id, &created)
	if err != nil {
		return "", false, fmt.Errorf("insert entry: %w", err)
	}

	return id, created, nil
}

func GetEntry(ctx context.Context, pool *pgxpool.Pool, id string) (entry.Published, bool, error) {
	getCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	var (
		published   entry.Published
		contentHTML string
	)
	err := pool.QueryRow(
		getCtx,
		`
			SELECT id, title, content_html, created_at, data_json IS NOT NULL
			FROM entries
			WHERE id = $1
		`,
		id,
	).Scan(
		&published.ID,
		&published.Title,
		&contentHTML,
		&published.CreatedAt,
		&published.HasDataJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry.Published{}, false, nil
	}
	if err != nil {
		return entry.Published{}, false, fmt.Errorf("get entry: %w", err)
	}
	published.ContentHTML, err = entry.ParseHTML(contentHTML)
	if err != nil {
		return entry.Published{}, false, fmt.Errorf("validate entry content: %w", err)
	}

	return published, true, nil
}

// ListEntries keeps the first entry beyond the HTML budget so the feed can detect truncation.
func ListEntries(
	ctx context.Context,
	pool *pgxpool.Pool,
	selected *entry.Feed,
	limit, maxHTMLBytes int,
) ([]entry.Published, error) {
	listCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var feed *string
	if selected != nil {
		name := selected.String()
		feed = &name
	}

	rows, err := pool.Query(
		listCtx,
		`
			WITH recent AS (
				SELECT id, title, content_html, created_at,
				       data_json IS NOT NULL AS has_data_json
				FROM entries
				WHERE $3::text IS NULL OR feed = $3
				ORDER BY created_at DESC, id DESC
				LIMIT $1
			), sized AS (
				SELECT recent.*,
				       COALESCE(sum(octet_length(content_html)) OVER (
					       ORDER BY created_at DESC, id DESC
					       ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
				       ), 0) AS previous_html_bytes
				FROM recent
			)
			SELECT id, title, content_html, created_at, has_data_json
			FROM sized
			WHERE previous_html_bytes <= $2
			ORDER BY created_at DESC, id DESC
		`,
		limit,
		maxHTMLBytes,
		feed,
	)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	entries := make([]entry.Published, 0)
	for rows.Next() {
		var (
			published   entry.Published
			contentHTML string
		)
		if err := rows.Scan(
			&published.ID,
			&published.Title,
			&contentHTML,
			&published.CreatedAt,
			&published.HasDataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		published.ContentHTML, err = entry.ParseHTML(contentHTML)
		if err != nil {
			return nil, fmt.Errorf("validate entry content: %w", err)
		}
		entries = append(entries, published)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries: %w", err)
	}

	return entries, nil
}

func GetEntryData(ctx context.Context, pool *pgxpool.Pool, id string) ([]byte, bool, error) {
	getCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	var data []byte
	err := pool.QueryRow(
		getCtx,
		"SELECT data_json::text FROM entries WHERE id = $1 AND data_json IS NOT NULL",
		id,
	).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get entry data: %w", err)
	}
	return data, true, nil
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
