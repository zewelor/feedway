//go:build integration

package database

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zewelor/feedway/internal/entry"
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

func TestInsertEntryDeduplicatesConcurrentWrites(t *testing.T) {
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
		t.Fatalf("Prepare() error = %v", err)
	}
	if _, err := pool.Exec(t.Context(), "TRUNCATE entries"); err != nil {
		t.Fatalf("truncate entries: %v", err)
	}

	values, err := entry.Normalize("title", "<p>content</p>")
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	const publications = 16
	var created atomic.Int32
	var waitGroup sync.WaitGroup
	insertErrors := make(chan error, publications)
	for range publications {
		waitGroup.Go(func() {
			wasCreated, err := InsertEntry(t.Context(), pool, values)
			if err != nil {
				insertErrors <- err
				return
			}
			if wasCreated {
				created.Add(1)
			}
		})
	}
	waitGroup.Wait()
	close(insertErrors)

	for err := range insertErrors {
		t.Errorf("InsertEntry() error = %v", err)
	}
	if created.Load() != 1 {
		t.Errorf("created count = %d, want 1", created.Load())
	}

	var (
		count       int
		title       *string
		contentHTML string
	)
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT count(*), min(title), min(content_html)
			FROM entries
			WHERE id = $1
		`,
		values.ID,
	).Scan(&count, &title, &contentHTML); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if count != 1 {
		t.Errorf("entry count = %d, want 1", count)
	}
	if title == nil || *title != "title" {
		t.Errorf("title = %v, want title", title)
	}
	if contentHTML != "<p>content</p>" {
		t.Errorf("content_html = %q, want %q", contentHTML, "<p>content</p>")
	}
}
