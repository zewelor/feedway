//go:build integration

package database

import (
	"fmt"
	"strings"
	"testing"
)

var testConfig = Config{
	Host:     "postgres",
	Port:     5432,
	Name:     "feedway_test",
	User:     "feedway",
	Password: "feedway_test",
	SSLMode:  "disable",
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
	const constraintQuery = `
		SELECT oid FROM pg_constraint
		WHERE conrelid = 'entries'::regclass AND conname = 'entries_public_id_valid'
	`
	var before, after uint32
	if err := pool.QueryRow(t.Context(), constraintQuery).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(t.Context(), pool); err != nil {
		t.Fatalf("second Prepare() error = %v", err)
	}
	if err := pool.QueryRow(t.Context(), constraintQuery).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("repeated schema preparation recreated the public ID constraint")
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
			SSLMode:  "disable",
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
			INSERT INTO entries (id, content_hash, title, content_html)
			VALUES ($1, $1, 'title', '<p>content</p>')
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
	if published.ContentHTML.String() != "<p>content</p>" {
		t.Errorf("ContentHTML = %q, want <p>content</p>", published.ContentHTML.String())
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

func TestGetEntryRejectsUnsanitizedHTML(t *testing.T) {
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
			INSERT INTO entries (id, content_hash, content_html)
			VALUES ($1, $1, '<script>alert(1)</script>')
		`,
		id,
	); err != nil {
		t.Fatalf("insert unsafe entry: %v", err)
	}

	_, found, err := GetEntry(t.Context(), pool, id)
	if err == nil || !strings.Contains(err.Error(), "validate entry content") {
		t.Fatalf("GetEntry() error = %v, want content validation error", err)
	}
	if found {
		t.Fatal("GetEntry() found = true, want false")
	}
}

func TestListEntriesUsesRequestedLimit(t *testing.T) {
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
			INSERT INTO entries (id, content_hash, title, content_html, created_at)
			SELECT
				'sha256-v1:' || lpad(to_hex(number), 64, '0'),
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

	entries, err := ListEntries(t.Context(), pool, nil, 100, 4*1024*1024)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 100 {
		t.Fatalf("entry count = %d, want 100", len(entries))
	}
	if entries[0].Title == nil || *entries[0].Title != "newest by id" {
		t.Errorf("first title = %v, want newest by id", entries[0].Title)
	}
	if entries[0].ContentHTML.String() != "<p>101</p>" {
		t.Errorf("first content_html = %q, want %q", entries[0].ContentHTML.String(), "<p>101</p>")
	}
	if entries[1].Title == nil || *entries[1].Title != "second by id" {
		t.Errorf("second title = %v, want second by id", entries[1].Title)
	}
	const wantLastID = "sha256-v1:0000000000000000000000000000000000000000000000000000000000000002"
	if entries[99].ID != wantLastID {
		t.Errorf("last ID = %q, want %q", entries[99].ID, wantLastID)
	}
	withExtra, err := ListEntries(t.Context(), pool, nil, 101, 4*1024*1024)
	if err != nil {
		t.Fatalf("ListEntries() lookahead error = %v", err)
	}
	if len(withExtra) != 101 || withExtra[100].ContentHTML.String() != "<p>1</p>" {
		t.Fatalf("lookahead did not return the older entry: count = %d", len(withExtra))
	}
}

func TestListEntriesBoundsHTMLRead(t *testing.T) {
	pool, err := Open(t.Context(), testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Prepare(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), "TRUNCATE entries"); err != nil {
		t.Fatal(err)
	}
	// Each entry contains exactly 32 bytes of HTML; equal timestamps exercise ID ordering.
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO entries (id, content_hash, content_html, created_at)
		SELECT 'sha256-v1:' || lpad(to_hex(number), 64, '0'),
		       'sha256-v1:' || lpad(to_hex(number), 64, '0'),
		       '<p>' || repeat('x', 25) || '</p>', '2026-01-01T00:00:00Z'::timestamptz
		FROM generate_series(1, 6) AS number
	`); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		limit     int
		budget    int
		wantCount int
	}{
		{name: "all fit", limit: 7, budget: 192, wantCount: 6},
		{name: "newest entry exceeds budget", limit: 7, budget: 1, wantCount: 1},
		{name: "first entry exceeding budget is retained", limit: 7, budget: 33, wantCount: 2},
		{name: "exact budget keeps one more entry", limit: 7, budget: 64, wantCount: 3},
		{name: "count limit applies first", limit: 2, budget: 64, wantCount: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries, err := ListEntries(t.Context(), pool, nil, test.limit, test.budget)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != test.wantCount {
				t.Fatalf("entry count = %d, want %d", len(entries), test.wantCount)
			}
			for i, published := range entries {
				if published.ID != fmt.Sprintf("sha256-v1:%064x", 6-i) {
					t.Errorf("entry %d = %q, want newest-first prefix", i, published.ID)
				}
				if len(published.ContentHTML.String()) != 32 {
					t.Errorf("entry %d HTML was changed or split", i)
				}
			}
		})
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
			INSERT INTO entries (id, content_hash, content_html, created_at)
			VALUES
				('sha256-v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
					'sha256-v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
					'<p>expired</p>', now() - interval '61 days'),
				('sha256-v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
					'sha256-v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
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
