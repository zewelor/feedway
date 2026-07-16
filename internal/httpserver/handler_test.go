package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testAPIToken = "01234567890123456789012345678901"

func TestHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		headers        map[string]string
		expectedStatus int
		expectedError  string
		skipLog        bool
	}{
		{
			name:           "health is public",
			method:         http.MethodGet,
			path:           "/healthz",
			expectedStatus: http.StatusOK,
			skipLog:        true,
		},
		{
			name:           "publishing requires authorization",
			method:         http.MethodPost,
			path:           "/api/v1/entries",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:   "publishing rejects invalid token",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Bearer invalid",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:   "publishing requires JSON",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIToken,
				"Content-Type":  "text/plain",
			},
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "Content-Type must be application/json",
		},
		{
			name:   "publishing accepts JSON media type parameters",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			body:   "{}",
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIToken,
				"Content-Type":  "application/json; charset=utf-8",
			},
			expectedStatus: http.StatusNotImplemented,
			expectedError:  "publishing is not implemented",
		},
		{
			name:   "publishing limits request body",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			body:   strings.Repeat("a", requestMaxBytes+1),
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIToken,
				"Content-Type":  "application/json",
			},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedError:  "request body is too large",
		},
		{
			name:           "unknown route is not found",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusNotFound,
			expectedError:  "not found",
		},
		{
			name:           "unsupported method is rejected",
			method:         http.MethodPost,
			path:           "/healthz",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method not allowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			response := httptest.NewRecorder()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			readiness := &readiness{
				database: pinger{},
			}

			newHandler(testAPIToken, readiness, logger).ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.expectedStatus)
			}
			if test.expectedError != "" {
				var body errorResponse
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if body.Error != test.expectedError {
					t.Fatalf("error = %q, want %q", body.Error, test.expectedError)
				}
			}

			if test.skipLog && logs.Len() != 0 {
				t.Fatalf("logs = %q, want no access log", logs.String())
			}
			if !test.skipLog && logs.Len() == 0 {
				t.Fatal("access log is empty")
			}
			if strings.Contains(logs.String(), testAPIToken) || strings.Contains(logs.String(), test.body) && test.body != "" {
				t.Fatal("logs contain a secret or request body")
			}
		})
	}
}

func TestHandlerResponseBodyIsBoundedAtLimit(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/entries",
		io.LimitReader(strings.NewReader(strings.Repeat("a", requestMaxBytes)), requestMaxBytes),
	)
	request.Header.Set("Authorization", "Bearer "+testAPIToken)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	readiness := &readiness{
		database: pinger{},
	}
	newHandler(
		testAPIToken,
		readiness,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	).ServeHTTP(response, request)

	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotImplemented)
	}
}

func TestReadiness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		databaseError  error
		isShuttingDown bool
		expectedStatus int
		expectLog      bool
	}{
		{
			name:           "ready",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "database unavailable",
			databaseError:  errors.New("database unavailable"),
			expectedStatus: http.StatusServiceUnavailable,
			expectLog:      true,
		},
		{
			name:           "shutting down",
			isShuttingDown: true,
			expectedStatus: http.StatusServiceUnavailable,
			expectLog:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			readiness := &readiness{
				database: pinger{err: test.databaseError},
			}
			readiness.isShuttingDown.Store(test.isShuttingDown)
			var logs bytes.Buffer
			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			response := httptest.NewRecorder()

			newHandler(
				testAPIToken,
				readiness,
				slog.New(slog.NewJSONHandler(&logs, nil)),
			).ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.expectedStatus)
			}
			if test.expectLog && logs.Len() == 0 {
				t.Fatal("access log is empty")
			}
			if !test.expectLog && logs.Len() != 0 {
				t.Fatalf("logs = %q, want no access log", logs.String())
			}
		})
	}
}

func TestPingDatabaseTimeout(t *testing.T) {
	t.Parallel()

	err := pingDatabase(t.Context(), pinger{waitForContext: true}, time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pingDatabase() error = %v, want context deadline exceeded", err)
	}
}

type pinger struct {
	err            error
	waitForContext bool
}

func (p pinger) Ping(ctx context.Context) error {
	if p.waitForContext {
		<-ctx.Done()
		return ctx.Err()
	}
	return p.err
}
