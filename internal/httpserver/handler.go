package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/zewelor/feedway/internal/entry"
)

const (
	readinessTimeout = 2 * time.Second
	// maxFeedBytes caps the uncompressed JSON Feed at 16 MiB.
	maxFeedBytes = 16 * 1024 * 1024
)

type errorResponse struct {
	Error string `json:"error"`
}

type entryRequest struct {
	Title       string `json:"title"`
	ContentHTML string `json:"content_html"`
}

type entryResponse struct {
	Result string `json:"result"`
	ID     string `json:"id"`
}

type entryPageData struct {
	Title       *string
	ContentHTML template.HTML
}

var entryPageTemplate = template.Must(template.New("entry").Parse(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Title}}{{.Title}}{{else}}Feedway{{end}}</title>
</head>
<body>
<article>
{{if .Title}}<h1>{{.Title}}</h1>
{{end}}{{.ContentHTML}}
</article>
</body>
</html>
`))

type databasePinger interface {
	Ping(context.Context) error
}

type publishEntry func(context.Context, entry.Values) (bool, error)
type loadEntry func(context.Context, string) (entry.Published, bool, error)
type loadFeed func(context.Context) ([]byte, error)

func newHandler(
	apiToken string,
	readiness *readiness,
	publish publishEntry,
	loadOne loadEntry,
	load loadFeed,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", readiness.handle)
	mux.Handle("POST /api/v1/entries", authenticate(
		apiToken,
		requireJSON(http.HandlerFunc(entries(publish, logger))),
	))
	mux.HandleFunc("GET /entries/{id}", entryPage(loadOne, logger))
	mux.HandleFunc("GET /feed.json", feed(load, logger))

	return logRequests(logger, mux)
}

func entryPage(load loadEntry, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		published, found, err := load(request.Context(), request.PathValue("id"))
		if err != nil {
			logger.ErrorContext(request.Context(), "load entry", "error", err)
			http.Error(response, "internal server error", http.StatusInternalServerError)
			return
		}
		if !found {
			http.NotFound(response, request)
			return
		}

		var body bytes.Buffer
		if err := entryPageTemplate.Execute(&body, entryPageData{
			Title: published.Title,
			// ContentHTML is sanitized before it is stored.
			ContentHTML: template.HTML(published.ContentHTML),
		}); err != nil {
			logger.ErrorContext(request.Context(), "render entry", "error", err)
			http.Error(response, "internal server error", http.StatusInternalServerError)
			return
		}

		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Content-Length", strconv.Itoa(body.Len()))
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.WriteHeader(http.StatusOK)
		if request.Method == http.MethodHead {
			return
		}
		if _, err := response.Write(body.Bytes()); err != nil {
			return
		}
	}
}

func feed(load loadFeed, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		body, err := load(request.Context())
		if err != nil {
			logger.ErrorContext(request.Context(), "load feed", "error", err)
			writeError(response, http.StatusInternalServerError, "internal server error")
			return
		}
		if len(body) > maxFeedBytes {
			writeError(response, http.StatusUnprocessableEntity, "feed is too large")
			return
		}

		hash := sha256.Sum256(body)
		etag := fmt.Sprintf(`"%x"`, hash)
		response.Header().Set("Content-Type", "application/feed+json; charset=utf-8")
		response.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("ETag", etag)
		response.Header().Set("Content-Length", strconv.Itoa(len(body)))

		if request.Header.Get("If-None-Match") == etag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
		if request.Method == http.MethodHead {
			response.WriteHeader(http.StatusOK)
			return
		}

		response.WriteHeader(http.StatusOK)
		if _, err := response.Write(body); err != nil {
			return
		}
	}
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}

func (r *readiness) handle(response http.ResponseWriter, request *http.Request) {
	if r.isShuttingDown.Load() {
		writeError(response, http.StatusServiceUnavailable, "not ready")
		return
	}

	if err := pingDatabase(request.Context(), r.database, readinessTimeout); err != nil {
		writeError(response, http.StatusServiceUnavailable, "not ready")
		return
	}

	response.WriteHeader(http.StatusOK)
}

func pingDatabase(ctx context.Context, database databasePinger, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return database.Ping(ctx)
}

func entries(publish publishEntry, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input entryRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeDecodeError(response, err)
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeDecodeError(response, err)
			return
		}

		values, err := entry.Normalize(input.Title, input.ContentHTML)
		if err != nil {
			writeError(response, http.StatusUnprocessableEntity, err.Error())
			return
		}

		created, err := publish(request.Context(), values)
		if err != nil {
			logger.ErrorContext(request.Context(), "publish entry", "error", err)
			writeError(response, http.StatusInternalServerError, "internal server error")
			return
		}

		result := "deduplicated"
		status := http.StatusOK
		if created {
			result = "created"
			status = http.StatusCreated
		}

		logger.InfoContext(
			request.Context(),
			"entry published",
			"id", values.ID,
			"result", result,
		)
		writeJSON(response, status, entryResponse{
			Result: result,
			ID:     values.ID,
		})
	}
}

func writeDecodeError(response http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(response, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	writeError(response, http.StatusBadRequest, "request body is invalid")
}

func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(response, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		request.Body = http.MaxBytesReader(response, request.Body, requestMaxBytes)
		next.ServeHTTP(response, request)
	})
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, errorResponse{Error: message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	body, err := json.Marshal(value)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(status)
	if _, err := response.Write(append(body, '\n')); err != nil {
		return
	}
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: response,
			status:         http.StatusOK,
		}

		next.ServeHTTP(recorder, request)

		isSuccessfulProbe := recorder.status == http.StatusOK &&
			(request.Pattern == "GET /healthz" || request.Pattern == "GET /readyz")
		if isSuccessfulProbe {
			return
		}

		logger.InfoContext(
			request.Context(),
			"HTTP request",
			"method", request.Method,
			"route", request.Pattern,
			"status", recorder.status,
			"duration", time.Since(startedAt),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
