//go:build integration

package database

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zewelor/feedway/internal/entry"
)

func TestDeduplicationSurvivesConcurrentRemovalOrReassignment(t *testing.T) {
	tests := []struct {
		name   string
		feed   string
		remove bool
	}{
		{name: "retention of unassigned entry", remove: true},
		{name: "retention of named entry", feed: "daily-briefing", remove: true},
		{name: "reassignment of unassigned entry"},
		{name: "reassignment of named entry", feed: "daily-briefing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDeduplicationFixture(t, test.feed)
			barrier := &publicationQueryBarrier{
				completed: make(chan struct{}),
				resume:    make(chan struct{}),
			}
			config, err := poolConfig(testConfig)
			if err != nil {
				t.Fatal(err)
			}
			config.ConnConfig.Tracer = barrier
			publisher, err := pgxpool.NewWithConfig(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(publisher.Close)
			resume := sync.OnceFunc(func() { close(barrier.resume) })
			defer resume()
			finished := make(chan publicationResult, 1)
			ctx := context.WithValue(t.Context(), publicationTraceKey{}, true)
			go func() {
				id, created, err := InsertEntry(ctx, publisher, fixture.retry)
				finished <- publicationResult{id: id, created: created, err: err}
			}()
			// Pause after the server completes the first statement, before InsertEntry returns.
			// The old two-statement flow would lose the conflict before its subsequent lookup.
			select {
			case <-barrier.completed:
			case got := <-finished:
				t.Fatalf("publication did not reach query barrier: %+v", got)
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			}
			if test.remove {
				if err := DeleteExpiredEntries(t.Context(), fixture.pool, 30); err != nil {
					t.Fatal(err)
				}
			} else {
				mutationCtx, cancel := context.WithTimeout(t.Context(), operationTimeout)
				defer cancel()
				if _, err := fixture.pool.Exec(mutationCtx, `
					UPDATE entries SET feed = 'madeira-news' WHERE id = $1
				`, fixture.published.ID); err != nil {
					t.Fatal(err)
				}
			}
			resume()
			got := <-finished
			if got.err != nil || got.created || got.id != fixture.published.ID {
				t.Fatalf("publication resolved before mutation: %+v, want deduplicated ID %s", got, fixture.published.ID)
			}
			fixture.assertOriginal(t, !test.remove)
			newID, created, err := InsertEntry(t.Context(), fixture.pool, fixture.retry)
			if err != nil || !created || newID == fixture.published.ID {
				t.Fatalf("publication after mutation: id=%s created=%v error=%v", newID, created, err)
			}
			repeatedID, created, err := InsertEntry(t.Context(), fixture.pool, fixture.retry)
			if err != nil || created || repeatedID != newID {
				t.Fatalf("repeat after mutation: id=%s created=%v error=%v", repeatedID, created, err)
			}
		})
	}
}

func TestPublicationWaitsForRemovalOrReassignment(t *testing.T) {
	tests := []struct {
		name   string
		feed   string
		remove bool
		commit bool
	}{
		{name: "retention commit unassigned", remove: true, commit: true},
		{name: "retention rollback unassigned", remove: true},
		{name: "retention commit named", feed: "daily-briefing", remove: true, commit: true},
		{name: "retention rollback named", feed: "daily-briefing", remove: true},
		{name: "reassignment commit unassigned", commit: true},
		{name: "reassignment rollback unassigned"},
		{name: "reassignment commit named", feed: "daily-briefing", commit: true},
		{name: "reassignment rollback named", feed: "daily-briefing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDeduplicationFixture(t, test.feed)
			ctx, cancel := context.WithTimeout(t.Context(), operationTimeout)
			defer cancel()
			config, err := poolConfig(testConfig)
			if err != nil {
				t.Fatal(err)
			}
			config.MaxConns = 1
			publisher, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(publisher.Close)
			connection, err := publisher.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			publisherPID := connection.Conn().PgConn().PID()
			connection.Release()
			tx, err := fixture.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), operationTimeout)
				defer cleanupCancel()
				if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
					t.Errorf("rollback mutation: %v", err)
				}
			}()
			mutation := "UPDATE entries SET feed = 'madeira-news' WHERE id = $1"
			if test.remove {
				mutation = "DELETE FROM entries WHERE id = $1 AND created_at < now() - interval '30 days'"
			}
			tag, err := tx.Exec(ctx, mutation, fixture.published.ID)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("mutate original entry: rows=%d error=%v", tag.RowsAffected(), err)
			}
			finished := make(chan publicationResult, 1)
			go func() {
				id, created, err := InsertEntry(ctx, publisher, fixture.retry)
				finished <- publicationResult{id: id, created: created, err: err}
			}()
			// Observe the actual lock wait before deciding the other transaction's outcome.
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				err := fixture.pool.QueryRow(
					ctx,
					"SELECT $1::int = ANY(pg_blocking_pids($2::int))",
					tx.Conn().PgConn().PID(),
					publisherPID,
				).Scan(&blocked)
				if err != nil {
					t.Fatalf("observe publication lock: %v", err)
				}
				if blocked {
					break
				}
				select {
				case got := <-finished:
					t.Fatalf("publication did not wait for mutation: %+v", got)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-ticker.C:
				}
			}
			if test.commit {
				err = tx.Commit(ctx)
			} else {
				err = tx.Rollback(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
			got := <-finished
			if got.err != nil || got.created != test.commit {
				t.Fatalf("publication after mutation: %+v; want created=%v", got, test.commit)
			}
			if test.commit && got.id == fixture.published.ID {
				t.Fatal("committed mutation reused the original ID in the source feed")
			}
			if !test.commit && got.id != fixture.published.ID {
				t.Fatal("rolled-back mutation changed the publication ID")
			}
			fixture.assertOriginal(t, !test.remove || !test.commit)
			data, found, err := GetEntryData(t.Context(), fixture.pool, got.id)
			wantData := `{"first":true}`
			if test.commit {
				wantData = `{"first":false}`
			}
			if err != nil || !found || string(data) != wantData {
				t.Fatalf("returned publication attachment: %q, found=%v error=%v", data, found, err)
			}
		})
	}
}

type publicationResult struct {
	id      string
	created bool
	err     error
}

type deduplicationFixture struct {
	pool      *pgxpool.Pool
	published entry.Published
	retry     entry.Values
}

func newDeduplicationFixture(t *testing.T, feedName string) *deduplicationFixture {
	t.Helper()
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
	original, err := entry.Normalize("report", "<p>report</p>", json.RawMessage(`{"first":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if feedName != "" {
		feed, err := entry.ParseFeed(feedName)
		if err != nil {
			t.Fatal(err)
		}
		original.Feed = &feed
	}
	id, created, err := InsertEntry(t.Context(), pool, original)
	if err != nil || !created {
		t.Fatalf("initial publication: created=%v error=%v", created, err)
	}
	if _, err := pool.Exec(t.Context(), `
		UPDATE entries SET created_at = now() - interval '31 days' WHERE id = $1
	`, id); err != nil {
		t.Fatal(err)
	}
	published, found, err := GetEntry(t.Context(), pool, id)
	if err != nil || !found || published.Title == nil {
		t.Fatalf("get original entry: found=%v error=%v", found, err)
	}
	retry, err := entry.Normalize("report", "<p>report</p>", json.RawMessage(`{"first":false}`))
	if err != nil {
		t.Fatal(err)
	}
	retry.Feed = original.Feed
	return &deduplicationFixture{pool: pool, published: published, retry: retry}
}

func (f *deduplicationFixture) assertOriginal(t *testing.T, wantFound bool) {
	t.Helper()
	after, found, err := GetEntry(t.Context(), f.pool, f.published.ID)
	if err != nil || found != wantFound {
		t.Fatalf("original entry after mutation: found=%v error=%v", found, err)
	}
	if !found {
		return
	}
	if !after.CreatedAt.Equal(f.published.CreatedAt) || after.ContentHTML.String() != f.published.ContentHTML.String() {
		t.Fatal("deduplication changed the stored content or publication date")
	}
	if after.Title == nil || *after.Title != *f.published.Title {
		t.Fatal("deduplication changed the stored title")
	}
	data, found, err := GetEntryData(t.Context(), f.pool, f.published.ID)
	if err != nil || !found || string(data) != `{"first":true}` {
		t.Fatalf("original attachment: %q, found=%v error=%v", data, found, err)
	}
}

type publicationTraceKey struct{}

type publicationQueryBarrier struct {
	once      sync.Once
	completed chan struct{}
	resume    chan struct{}
}

func (*publicationQueryBarrier) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TraceQueryStartData,
) context.Context {
	return ctx
}

func (b *publicationQueryBarrier) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(publicationTraceKey{}) != true || data.Err != nil {
		return
	}
	b.once.Do(func() {
		close(b.completed)
		select {
		case <-b.resume:
		case <-ctx.Done():
		}
	})
}
