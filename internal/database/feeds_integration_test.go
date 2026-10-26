//go:build integration

package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zewelor/feedway/internal/entry"
)

func TestConcurrentPublicationsAcrossFeeds(t *testing.T) {
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
	const perFeed = 16
	type result struct {
		scope   string
		attempt int
		id      string
		created bool
		err     error
	}
	results := make(chan result, 3*perFeed)
	start := make(chan struct{})
	for _, scope := range []string{"", "daily-briefing", "madeira-news"} {
		var feed *entry.Feed
		if scope != "" {
			parsed, err := entry.ParseFeed(scope)
			if err != nil {
				t.Fatal(err)
			}
			feed = &parsed
		}
		for attempt := range perFeed {
			values, err := entry.Normalize("shared", "<p>shared</p>", json.RawMessage(
				fmt.Sprintf(`{"scope":%q,"attempt":%d}`, scope, attempt),
			))
			if err != nil {
				t.Fatal(err)
			}
			values.Feed = feed
			go func() {
				<-start
				id, created, err := InsertEntry(t.Context(), pool, values)
				results <- result{scope: scope, attempt: attempt, id: id, created: created, err: err}
			}()
		}
	}
	close(start)
	ids := make(map[string]string)
	createdCounts := make(map[string]int)
	winners := make(map[string]int)
	for range 3 * perFeed {
		got := <-results
		if got.err != nil {
			t.Errorf("publish to %q: %v", got.scope, got.err)
			continue
		}
		if previous, ok := ids[got.scope]; ok && previous != got.id {
			t.Errorf("scope %q returned different IDs: %s, %s", got.scope, previous, got.id)
		}
		ids[got.scope] = got.id
		if got.created {
			createdCounts[got.scope]++
			winners[got.scope] = got.attempt
		}
	}
	for _, scope := range []string{"", "daily-briefing", "madeira-news"} {
		if createdCounts[scope] != 1 {
			t.Fatalf("scope %q created %d entries, want 1", scope, createdCounts[scope])
		}
		var version, winningAttempt int
		var storedScope, title, contentHTML string
		err := pool.QueryRow(t.Context(), `
			SELECT uuid_extract_version(id::uuid), data_json->>'scope', (data_json->>'attempt')::int,
			       title, content_html
			FROM entries WHERE id = $1
		`, ids[scope]).Scan(&version, &storedScope, &winningAttempt, &title, &contentHTML)
		if err != nil {
			t.Fatal(err)
		}
		if version != 7 || storedScope != scope || winningAttempt != winners[scope] {
			t.Fatalf("scope %q: UUID version=%d, stored scope=%q, attempt=%d; winner=%d",
				scope, version, storedScope, winningAttempt, winners[scope])
		}
		if title != "shared" || contentHTML != "<p>shared</p>" {
			t.Fatalf("scope %q stored title=%q HTML=%q", scope, title, contentHTML)
		}
	}
	if ids[""] == ids["daily-briefing"] || ids[""] == ids["madeira-news"] ||
		ids["daily-briefing"] == ids["madeira-news"] {
		t.Fatalf("different scopes share IDs: %v", ids)
	}
	var count int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM entries").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("stored entries = %d, want 3", count)
	}
}

func TestFeedSchemaConstraints(t *testing.T) {
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
	const insert = "INSERT INTO entries(feed, content_hash, content_html) VALUES ($1, $2, '<p>safe</p>')"
	hash := "sha256-v1:" + strings.Repeat("a", 64)
	for _, feed := range []any{nil, "a", "b"} {
		if _, err := pool.Exec(t.Context(), insert, feed, hash); err != nil {
			t.Fatal(err)
		}
	}
	expectError := func(code, query string, args ...any) {
		t.Helper()
		_, err := pool.Exec(t.Context(), query, args...)
		var pgError *pgconn.PgError
		if !errors.As(err, &pgError) || pgError.Code != code {
			t.Fatalf("query error = %v, want SQLSTATE %s", err, code)
		}
	}
	expectError("23505", insert, nil, hash)
	expectError("23505", insert, "a", hash)
	expectError("23505", "UPDATE entries SET feed = NULL WHERE feed = 'a'")
	for _, feed := range []string{"", "A", " a", "a ", "a\n", "a--b", "-a", "a-", "ą", strings.Repeat("a", 65)} {
		expectError("23514", insert, feed, hash)
	}
	expectError("23502", insert, "valid", nil)
	expectError("23514", insert, "valid", "invalid-hash")
}

func TestPrepareRollsBackFailedMigration(t *testing.T) {
	admin, err := Open(t.Context(), testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE feedway_migration_test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE feedway_migration_test"); err != nil {
			t.Errorf("drop migration test database: %v", err)
		}
	})
	config := testConfig
	config.Name = "feedway_migration_test"
	pool, err := Open(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(t.Context(), `
		CREATE TABLE entries (
			id text PRIMARY KEY, title text, content_html text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		);
		INSERT INTO entries (id, content_html) VALUES ('invalid-legacy-id', '<p>safe</p>');
	`); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(t.Context(), pool); err == nil {
		t.Fatal("migration accepted an invalid legacy content hash")
	}
	var columns int
	err = pool.QueryRow(t.Context(), `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'entries'
		AND column_name IN ('data_json', 'feed', 'content_hash')
	`).Scan(&columns)
	if err != nil || columns != 0 {
		t.Fatalf("failed migration left %d new columns, error=%v", columns, err)
	}
	var id, html string
	err = pool.QueryRow(t.Context(), "SELECT id, content_html FROM entries").Scan(&id, &html)
	if err != nil || id != "invalid-legacy-id" || html != "<p>safe</p>" {
		t.Fatalf("failed migration changed old data: id=%q html=%q error=%v", id, html, err)
	}
	values, err := entry.Normalize("", html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE entries SET id = $1", values.ContentHash); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Prepare(t.Context(), pool); err != nil {
			t.Fatal(err)
		}
	}
	storedID, created, err := InsertEntry(t.Context(), pool, values)
	if err != nil || created || storedID != values.ContentHash {
		t.Fatalf("legacy retry after repair: id=%q created=%v error=%v", storedID, created, err)
	}
}
