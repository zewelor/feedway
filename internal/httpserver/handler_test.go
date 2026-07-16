package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

			newHandler(testAPIToken, logger).ServeHTTP(response, request)

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

	newHandler(testAPIToken, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(response, request)

	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotImplemented)
	}
}
