package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/zewelor/feedway/internal/entry"
)

const readinessTimeout = 2 * time.Second

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

type databasePinger interface {
	Ping(context.Context) error
}

type publishEntry func(context.Context, entry.Values) (bool, error)

func newHandler(
	apiToken string,
	readiness *readiness,
	publish publishEntry,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("/healthz", methodNotAllowed)
	mux.HandleFunc("GET /readyz", readiness.handle)
	mux.HandleFunc("/readyz", methodNotAllowed)
	mux.Handle("POST /api/v1/entries", authenticate(
		apiToken,
		requireJSON(http.HandlerFunc(entries(publish, logger))),
	))
	mux.HandleFunc("/api/v1/entries", methodNotAllowed)
	mux.HandleFunc("/", notFound)

	return logRequests(logger, mux)
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

func methodNotAllowed(response http.ResponseWriter, _ *http.Request) {
	writeError(response, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(response http.ResponseWriter, _ *http.Request) {
	writeError(response, http.StatusNotFound, "not found")
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
