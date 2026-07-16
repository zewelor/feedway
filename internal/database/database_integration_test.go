//go:build integration

package database

import (
	"os"
	"strings"
	"testing"
)

func TestPrepare(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	pool, err := Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := Prepare(t.Context(), pool); err != nil {
		t.Fatalf("first Prepare() error = %v", err)
	}
	if err := Prepare(t.Context(), pool); err != nil {
		t.Fatalf("second Prepare() error = %v", err)
	}

	var tableName string
	if err := pool.QueryRow(
		t.Context(),
		"SELECT to_regclass('public.entries')::text",
	).Scan(&tableName); err != nil {
		t.Fatalf("query entries table: %v", err)
	}
	if tableName != "entries" {
		t.Fatalf("table = %q, want entries", tableName)
	}

	var indexName string
	if err := pool.QueryRow(
		t.Context(),
		"SELECT to_regclass('public.entries_created_index')::text",
	).Scan(&indexName); err != nil {
		t.Fatalf("query entries index: %v", err)
	}
	if indexName != "entries_created_index" {
		t.Fatalf("index = %q, want entries_created_index", indexName)
	}
}

func TestOpenUnavailableDatabase(t *testing.T) {
	pool, err := Open(
		t.Context(),
		"postgres://feedway:feedway_test@127.0.0.1:1/feedway_test?connect_timeout=1",
	)
	if pool != nil {
		pool.Close()
		t.Fatal("Open() returned a pool")
	}
	if err == nil || !strings.Contains(err.Error(), "connect to database") {
		t.Fatalf("Open() error = %v, want connection error", err)
	}
}
