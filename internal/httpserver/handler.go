package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"
)

type errorResponse struct {
	Error string `json:"error"`
}

func newHandler(apiToken string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("/healthz", methodNotAllowed)
	mux.Handle("POST /api/v1/entries", authenticate(
		apiToken,
		requireJSON(http.HandlerFunc(entries)),
	))
	mux.HandleFunc("/api/v1/entries", methodNotAllowed)
	mux.HandleFunc("/", notFound)

	return logRequests(logger, mux)
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}

func methodNotAllowed(response http.ResponseWriter, _ *http.Request) {
	writeError(response, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(response http.ResponseWriter, _ *http.Request) {
	writeError(response, http.StatusNotFound, "not found")
}

func entries(response http.ResponseWriter, request *http.Request) {
	_, err := io.Copy(io.Discard, request.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(response, http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}
		writeError(response, http.StatusBadRequest, "request body is invalid")
		return
	}

	writeError(response, http.StatusNotImplemented, "publishing is not implemented")
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
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	body, err := json.Marshal(errorResponse{Error: message})
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

		if request.Pattern == "GET /healthz" && recorder.status == http.StatusOK {
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
