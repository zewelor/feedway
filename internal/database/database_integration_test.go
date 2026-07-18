//go:build integration

package database

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zewelor/feedway/internal/entry"
)

var testConfig = Config{
	Host:     "postgres",
	Port:     5432,
	Name:     "feedway_test",
	User:     "feedway",
	Password: "feedway_test",
}

func TestPrepare(t *testing.T) {
	pool, err := Open(t.Context(), testConfig)
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
		Config{
			Host:     "127.0.0.1",
			Port:     1,
			Name:     "feedway_test",
			User:     "feedway",
			Password: "feedway_test",
		},
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
	pool, err := Open(t.Context(), testConfig)
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

func TestGetEntry(t *testing.T) {
	pool, err := Open(t.Context(), testConfig)
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

	const id = "sha256-v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := pool.Exec(
		t.Context(),
		`
			INSERT INTO entries (id, title, content_html)
			VALUES ($1, 'title', '<p>content</p>')
		`,
		id,
	); err != nil {
		t.Fatalf("insert entry: %v", err)
	}

	published, found, err := GetEntry(t.Context(), pool, id)
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}
	if !found {
		t.Fatal("GetEntry() found = false, want true")
	}
	if published.ID != id {
		t.Errorf("ID = %q, want %q", published.ID, id)
	}
	if published.Title == nil || *published.Title != "title" {
		t.Errorf("Title = %v, want title", published.Title)
	}
	if published.ContentHTML != "<p>content</p>" {
		t.Errorf("ContentHTML = %q, want <p>content</p>", published.ContentHTML)
	}
	if published.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	_, found, err = GetEntry(t.Context(), pool, "sha256-v1:missing")
	if err != nil {
		t.Fatalf("GetEntry() missing error = %v", err)
	}
	if found {
		t.Fatal("GetEntry() missing found = true, want false")
	}
}

func TestListEntriesReturnsNewestHundred(t *testing.T) {
	pool, err := Open(t.Context(), testConfig)
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
	if _, err := pool.Exec(
		t.Context(),
		`
			INSERT INTO entries (id, title, content_html, created_at)
			SELECT
				'sha256-v1:' || lpad(to_hex(number), 64, '0'),
				CASE
					WHEN number = 101 THEN 'newest by id'
					WHEN number = 100 THEN 'second by id'
				END,
				'<p>' || number || '</p>',
				'2026-01-01T00:00:00Z'::timestamptz
					+ least(number, 100) * interval '1 second'
			FROM generate_series(1, 101) AS number
		`,
	); err != nil {
		t.Fatalf("insert entries: %v", err)
	}

	entries, err := ListEntries(t.Context(), pool)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 100 {
		t.Fatalf("entry count = %d, want 100", len(entries))
	}
	if entries[0].Title == nil || *entries[0].Title != "newest by id" {
		t.Errorf("first title = %v, want newest by id", entries[0].Title)
	}
	if entries[0].ContentHTML != "<p>101</p>" {
		t.Errorf("first content_html = %q, want %q", entries[0].ContentHTML, "<p>101</p>")
	}
	if entries[1].Title == nil || *entries[1].Title != "second by id" {
		t.Errorf("second title = %v, want second by id", entries[1].Title)
	}
	const wantLastID = "sha256-v1:0000000000000000000000000000000000000000000000000000000000000002"
	if entries[99].ID != wantLastID {
		t.Errorf("last ID = %q, want %q", entries[99].ID, wantLastID)
	}
}

func TestDeleteExpiredEntries(t *testing.T) {
	pool, err := Open(t.Context(), testConfig)
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
	if _, err := pool.Exec(
		t.Context(),
		`
			INSERT INTO entries (id, content_html, created_at)
			VALUES
				('sha256-v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
					'<p>expired</p>', now() - interval '61 days'),
				('sha256-v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
					'<p>retained</p>', now() - interval '59 days')
		`,
	); err != nil {
		t.Fatalf("insert entries: %v", err)
	}

	if err := DeleteExpiredEntries(t.Context(), pool, 60); err != nil {
		t.Fatalf("first DeleteExpiredEntries() error = %v", err)
	}
	if err := DeleteExpiredEntries(t.Context(), pool, 60); err != nil {
		t.Fatalf("second DeleteExpiredEntries() error = %v", err)
	}

	var (
		count       int
		contentHTML string
	)
	if err := pool.QueryRow(
		t.Context(),
		"SELECT count(*), min(content_html) FROM entries",
	).Scan(&count, &contentHTML); err != nil {
		t.Fatalf("query retained entries: %v", err)
	}
	if count != 1 {
		t.Errorf("entry count = %d, want 1", count)
	}
	if contentHTML != "<p>retained</p>" {
		t.Errorf("content_html = %q, want retained entry", contentHTML)
	}
}
