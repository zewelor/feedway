package retention

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunCleansImmediatelyAndAtInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		done := make(chan struct{})
		go func() {
			defer close(done)
			run(
				ctx,
				cleanupInterval,
				func(context.Context) error {
					calls.Add(1)
					return nil
				},
				slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
			)
		}()

		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("cleanup calls after start = %d, want 1", calls.Load())
		}

		time.Sleep(cleanupInterval)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatalf("cleanup calls after interval = %d, want 2", calls.Load())
		}

		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("retention did not stop")
		}
	})
}

func TestRunLogsCleanupErrorAndContinues(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var logs bytes.Buffer
		var calls atomic.Int32
		go run(
			ctx,
			cleanupInterval,
			func(context.Context) error {
				if calls.Add(1) == 1 {
					return errors.New("database unavailable")
				}
				return nil
			},
			slog.New(slog.NewTextHandler(&logs, nil)),
		)

		synctest.Wait()
		if !strings.Contains(logs.String(), "database unavailable") {
			t.Fatalf("logs = %q, want cleanup error", logs.String())
		}

		time.Sleep(cleanupInterval)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatalf("cleanup calls = %d, want 2", calls.Load())
		}
	})
}

func TestRunCancelsActiveCleanupWithoutLogging(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var logs bytes.Buffer
		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			run(
				ctx,
				cleanupInterval,
				func(ctx context.Context) error {
					close(started)
					<-ctx.Done()
					return ctx.Err()
				},
				slog.New(slog.NewTextHandler(&logs, nil)),
			)
		}()

		<-started
		cancel()
		synctest.Wait()
		<-done

		if logs.Len() != 0 {
			t.Fatalf("logs = %q, want no shutdown error", logs.String())
		}
	})
}
