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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/entry"
)

const testAPIToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

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
		expectedAllow  string
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
			name:   "publishing rejects wrong authorization scheme",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Basic " + testAPIToken,
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:   "publishing rejects authorization without separator",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Bearer" + testAPIToken,
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:   "publishing rejects extra authorization whitespace",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Bearer  " + testAPIToken,
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:   "publishing rejects token with valid prefix",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIToken + "0",
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
			body:   `{"content_html":"<p>content</p>"}`,
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIToken,
				"Content-Type":  "application/json; charset=utf-8",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:   "publishing limits request body",
			method: http.MethodPost,
			path:   "/api/v1/entries",
			body:   `{"content_html":"` + strings.Repeat("a", requestMaxBytes) + `"}`,
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
		},
		{
			name:           "unsupported method is rejected",
			method:         http.MethodPost,
			path:           "/healthz",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedAllow:  "GET, HEAD",
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

			newHandler(
				testAPIToken,
				readiness,
				func(context.Context, entry.Values) (bool, error) {
					return true, nil
				},
				testEntry,
				testFeed,
				logger,
			).ServeHTTP(response, request)

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
			if response.Header().Get("Allow") != test.expectedAllow {
				t.Fatalf(
					"Allow = %q, want %q",
					response.Header().Get("Allow"),
					test.expectedAllow,
				)
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

func TestRequestLogging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		pattern        string
		method         string
		target         string
		expectedPath   string
		expectedStatus int
	}{
		{
			name:           "dynamic path",
			pattern:        "GET /entries/{id}",
			method:         http.MethodGet,
			target:         "/entries/sha256-v1:test?token=secret",
			expectedPath:   "/entries/sha256-v1:test",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unknown path",
			pattern:        "GET /healthz",
			method:         http.MethodGet,
			target:         "/favicon.ico?token=secret",
			expectedPath:   "/favicon.ico",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unsupported method",
			pattern:        "GET /healthz",
			method:         http.MethodPost,
			target:         "/healthz?token=secret",
			expectedPath:   "/healthz",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			mux.HandleFunc(test.pattern, func(http.ResponseWriter, *http.Request) {})

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			request := httptest.NewRequest(test.method, test.target, nil)
			response := httptest.NewRecorder()

			logRequests(logger, mux).ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.expectedStatus)
			}
			var logged struct {
				Method string `json:"method"`
				Path   string `json:"path"`
				Status int    `json:"status"`
			}
			if err := json.NewDecoder(&logs).Decode(&logged); err != nil {
				t.Fatalf("decode log: %v", err)
			}
			if logged.Method != test.method {
				t.Errorf("method = %q, want %q", logged.Method, test.method)
			}
			if logged.Path != test.expectedPath {
				t.Errorf("path = %q, want %q", logged.Path, test.expectedPath)
			}
			if logged.Status != test.expectedStatus {
				t.Errorf("logged status = %d, want %d", logged.Status, test.expectedStatus)
			}
			if strings.Contains(logs.String(), "secret") {
				t.Fatal("log contains query string")
			}
			if strings.Contains(logs.String(), `"route":`) {
				t.Fatal("log contains redundant route")
			}
		})
	}
}

func TestAuthenticateRejectsMultipleAuthorizationHeaders(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/entries", nil)
	request.Header.Add("Authorization", "Bearer "+testAPIToken)
	request.Header.Add("Authorization", "Bearer invalid")
	response := httptest.NewRecorder()

	authenticate(testAPIToken, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
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
				func(context.Context, entry.Values) (bool, error) {
					return true, nil
				},
				testEntry,
				testFeed,
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

func TestPublishEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		created        bool
		publishError   error
		expectedStatus int
		expectedResult string
		expectedError  string
	}{
		{
			name:           "created",
			body:           `{"title":" title ","content_html":"<p onclick=\"bad\">content</p>"}`,
			created:        true,
			expectedStatus: http.StatusCreated,
			expectedResult: "created",
		},
		{
			name:           "deduplicated",
			body:           `{"content_html":"<p>content</p>"}`,
			expectedStatus: http.StatusOK,
			expectedResult: "deduplicated",
		},
		{
			name:           "missing content",
			body:           `{}`,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "content_html is required",
		},
		{
			name:           "unknown field",
			body:           `{"content_html":"content","id":"client-id"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "request body is invalid",
		},
		{
			name:           "trailing JSON",
			body:           `{"content_html":"content"} {}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "request body is invalid",
		},
		{
			name:           "database error",
			body:           `{"content_html":"content"}`,
			publishError:   errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/entries",
				strings.NewReader(test.body),
			)
			request.Header.Set("Authorization", "Bearer "+testAPIToken)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			var published entry.Values
			publish := func(_ context.Context, values entry.Values) (bool, error) {
				published = values
				return test.created, test.publishError
			}

			newHandler(
				testAPIToken,
				&readiness{database: pinger{}},
				publish,
				testEntry,
				testFeed,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			).ServeHTTP(response, request)

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
				return
			}

			var body entryResponse
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Result != test.expectedResult {
				t.Errorf("result = %q, want %q", body.Result, test.expectedResult)
			}
			if body.ID != published.ID {
				t.Errorf("ID = %q, want %q", body.ID, published.ID)
			}
		})
	}
}

func TestEntryPage(t *testing.T) {
	t.Parallel()

	title := "<Daily & report>"
	const pageWithTitle = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>&lt;Daily &amp; report&gt;</title>
</head>
<body>
<article>
<h1>&lt;Daily &amp; report&gt;</h1>
<p>content</p>
</article>
</body>
</html>
`
	const pageWithoutTitle = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Feedway</title>
</head>
<body>
<article>
<p>content</p>
</article>
</body>
</html>
`

	tests := []struct {
		name           string
		method         string
		published      entry.Published
		found          bool
		loadError      error
		expectedStatus int
		expectedBody   string
		expectedLength int
		expectedAllow  string
	}{
		{
			name:   "entry with title",
			method: http.MethodGet,
			published: entry.Published{
				Title:       &title,
				ContentHTML: "<p>content</p>",
			},
			found:          true,
			expectedStatus: http.StatusOK,
			expectedBody:   pageWithTitle,
			expectedLength: len(pageWithTitle),
		},
		{
			name:   "entry without title",
			method: http.MethodGet,
			published: entry.Published{
				ContentHTML: "<p>content</p>",
			},
			found:          true,
			expectedStatus: http.StatusOK,
			expectedBody:   pageWithoutTitle,
			expectedLength: len(pageWithoutTitle),
		},
		{
			name:   "head",
			method: http.MethodHead,
			published: entry.Published{
				Title:       &title,
				ContentHTML: "<p>content</p>",
			},
			found:          true,
			expectedStatus: http.StatusOK,
			expectedLength: len(pageWithTitle),
		},
		{
			name:           "not found",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "database error",
			method:         http.MethodGet,
			loadError:      errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error\n",
		},
		{
			name:           "unsupported method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
			expectedAllow:  "GET, HEAD",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			load := func(_ context.Context, id string) (entry.Published, bool, error) {
				if id != "sha256-v1:test" {
					t.Errorf("ID = %q, want sha256-v1:test", id)
				}
				return test.published, test.found, test.loadError
			}
			request := httptest.NewRequest(test.method, "/entries/sha256-v1:test", nil)
			response := httptest.NewRecorder()

			newHandler(
				testAPIToken,
				&readiness{database: pinger{}},
				func(context.Context, entry.Values) (bool, error) {
					return true, nil
				},
				load,
				testFeed,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			).ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.expectedStatus)
			}
			if response.Body.String() != test.expectedBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.expectedBody)
			}
			if response.Header().Get("Allow") != test.expectedAllow {
				t.Errorf("Allow = %q, want %q", response.Header().Get("Allow"), test.expectedAllow)
			}
			if test.expectedStatus == http.StatusOK {
				if response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
					t.Errorf("Content-Type = %q", response.Header().Get("Content-Type"))
				}
				if response.Header().Get("Content-Length") != strconv.Itoa(test.expectedLength) {
					t.Errorf("Content-Length = %q", response.Header().Get("Content-Length"))
				}
				if response.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Errorf("X-Content-Type-Options = %q", response.Header().Get("X-Content-Type-Options"))
				}
			}
		})
	}
}

func TestFeed(t *testing.T) {
	t.Parallel()

	const body = `{"version":"https://jsonfeed.org/version/1.1","title":"Feedway","items":[]}`
	const etag = `"810768039550b475b5f15ef292e34ec3fa2440c38f0f9ffee7e3acf7a987f44f"`

	tests := []struct {
		name           string
		method         string
		path           string
		ifNoneMatch    string
		load           loadFeed
		expectedStatus int
		expectedBody   string
		expectedLength int
		expectedETag   string
		expectedError  string
		expectedAllow  string
	}{
		{
			name:           "get",
			method:         http.MethodGet,
			path:           "/feed.json",
			load:           testFeed,
			expectedStatus: http.StatusOK,
			expectedBody:   body,
			expectedLength: len(body),
			expectedETag:   etag,
		},
		{
			name:           "head",
			method:         http.MethodHead,
			path:           "/feed.json",
			load:           testFeed,
			expectedStatus: http.StatusOK,
			expectedLength: len(body),
			expectedETag:   etag,
		},
		{
			name:           "not modified",
			method:         http.MethodGet,
			path:           "/feed.json",
			ifNoneMatch:    etag,
			load:           testFeed,
			expectedStatus: http.StatusNotModified,
			expectedETag:   etag,
		},
		{
			name:           "unsupported method",
			method:         http.MethodPost,
			path:           "/feed.json",
			load:           testFeed,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
			expectedAllow:  "GET, HEAD",
		},
		{
			name:   "maximum size",
			method: http.MethodGet,
			path:   "/feed.json",
			load: func(context.Context) ([]byte, error) {
				return []byte(strings.Repeat("x", maxFeedBytes)), nil
			},
			expectedStatus: http.StatusOK,
			expectedBody:   strings.Repeat("x", maxFeedBytes),
			expectedLength: maxFeedBytes,
		},
		{
			name:   "too large",
			method: http.MethodGet,
			path:   "/feed.json",
			load: func(context.Context) ([]byte, error) {
				return []byte(strings.Repeat("x", maxFeedBytes+1)), nil
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "feed is too large",
		},
		{
			name:   "load error",
			method: http.MethodGet,
			path:   "/feed.json",
			load: func(context.Context) ([]byte, error) {
				return nil, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("If-None-Match", test.ifNoneMatch)
			response := httptest.NewRecorder()

			newHandler(
				testAPIToken,
				&readiness{database: pinger{}},
				func(context.Context, entry.Values) (bool, error) {
					return true, nil
				},
				testEntry,
				test.load,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			).ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.expectedStatus)
			}
			if response.Header().Get("Allow") != test.expectedAllow {
				t.Fatalf(
					"Allow = %q, want %q",
					response.Header().Get("Allow"),
					test.expectedAllow,
				)
			}
			if test.expectedError != "" {
				var errorBody errorResponse
				if err := json.NewDecoder(response.Body).Decode(&errorBody); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if errorBody.Error != test.expectedError {
					t.Fatalf("error = %q, want %q", errorBody.Error, test.expectedError)
				}
				return
			}
			if response.Body.String() != test.expectedBody {
				t.Errorf("body size = %d, want %d", response.Body.Len(), len(test.expectedBody))
			}
			isFeedResponse := test.expectedStatus == http.StatusOK ||
				test.expectedStatus == http.StatusNotModified
			if isFeedResponse {
				if response.Header().Get("Content-Type") != "application/feed+json; charset=utf-8" {
					t.Errorf("Content-Type = %q", response.Header().Get("Content-Type"))
				}
				if response.Header().Get("Cache-Control") != "public, max-age=60, must-revalidate" {
					t.Errorf("Cache-Control = %q", response.Header().Get("Cache-Control"))
				}
				if response.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Errorf("X-Content-Type-Options = %q", response.Header().Get("X-Content-Type-Options"))
				}
			}
			if test.expectedStatus == http.StatusOK {
				if response.Header().Get("Content-Length") != strconv.Itoa(test.expectedLength) {
					t.Errorf("Content-Length = %q", response.Header().Get("Content-Length"))
				}
			}
			if test.expectedETag != "" &&
				response.Header().Get("ETag") != test.expectedETag {
				t.Errorf("ETag = %q, want %q", response.Header().Get("ETag"), test.expectedETag)
			}
		})
	}
}

func testFeed(context.Context) ([]byte, error) {
	return []byte(`{"version":"https://jsonfeed.org/version/1.1","title":"Feedway","items":[]}`), nil
}

func testEntry(context.Context, string) (entry.Published, bool, error) {
	return entry.Published{}, false, nil
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
