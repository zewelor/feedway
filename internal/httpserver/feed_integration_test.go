//go:build integration

package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/database"
)

func TestFeedTruncationLifecycle(t *testing.T) {
	const feedLimit = 150
	createHTTPTestDatabase(t)
	pool, err := database.Open(t.Context(), dataTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Prepare(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	server := httptest.NewServer(databaseHandler(
		Config{APIToken: testAPIToken, FeedResponseMaxEntries: feedLimit},
		pool,
		&readiness{database: pool},
		slog.New(slog.NewTextHandler(&logs, nil)),
	))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, etag, body string, status int) ([]byte, http.Header) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("If-None-Match", etag)
		if method == http.MethodPost {
			req.Header.Set("Authorization", "Bearer "+testAPIToken)
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		closeError := response.Body.Close()
		if err != nil || closeError != nil {
			t.Fatalf("read response: %v; close response: %v", err, closeError)
		}
		if response.StatusCode != status {
			t.Fatalf("%s %s = %d, want %d; body = %.200s; logs = %s",
				method, path, response.StatusCode, status, data, &logs)
		}
		return data, response.Header
	}
	type feedResponse struct {
		Items    []json.RawMessage `json:"items"`
		Metadata *struct {
			Truncated bool `json:"truncated"`
		} `json:"_feedway"`
	}
	decode := func(body []byte) feedResponse {
		t.Helper()
		var feed feedResponse
		if err := json.Unmarshal(body, &feed); err != nil {
			t.Fatal(err)
		}
		if feed.Metadata == nil {
			t.Fatal("feed has no truncation signal")
		}
		return feed
	}
	body, _ := request(http.MethodGet, "/feed.json", "", "", http.StatusOK)
	if empty := decode(body); len(empty.Items) != 0 || empty.Metadata.Truncated {
		t.Fatal("empty database is marked truncated or has entries")
	}
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO entries (id, content_html, created_at)
		SELECT 'sha256-v1:' || lpad(to_hex(number), 64, '0'), '<p>report</p>',
			now() - ($1+1-number) * interval '1 second'
		FROM generate_series(1, $1) AS number
	`, feedLimit); err != nil {
		t.Fatal(err)
	}
	completeBody, completeHeaders := request(http.MethodGet, "/feed.json", "", "", http.StatusOK)
	complete := decode(completeBody)
	if len(complete.Items) != feedLimit || complete.Metadata.Truncated {
		t.Fatal("entries at the count limit are incorrectly truncated")
	}
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO entries (id, content_html, created_at)
		VALUES ('sha256-v1:' || repeat('0', 64), '<p>older</p>', now() - interval '31 days')
	`); err != nil {
		t.Fatal(err)
	}
	truncatedBody, truncatedHeaders := request(http.MethodGet, "/feed.json", completeHeaders.Get("ETag"), "",
		http.StatusOK)
	truncated := decode(truncatedBody)
	if len(truncated.Items) != feedLimit || !truncated.Metadata.Truncated {
		t.Fatal("the extra retained entry did not mark the feed as truncated")
	}
	for i := range complete.Items {
		if !bytes.Equal(complete.Items[i], truncated.Items[i]) {
			t.Fatalf("visible item %d changed when an older entry was added", i)
		}
	}
	if completeHeaders.Get("ETag") == truncatedHeaders.Get("ETag") {
		t.Fatal("truncation metadata did not change the ETag")
	}
	head, headers := request(http.MethodHead, "/feed.json", "", "", http.StatusOK)
	if len(head) != 0 || headers.Get("Content-Length") != strconv.Itoa(len(truncatedBody)) ||
		headers.Get("ETag") != truncatedHeaders.Get("ETag") {
		t.Fatalf("HEAD differs from GET: headers = %v, body bytes = %d", headers, len(head))
	}
	unchanged, _ := request(http.MethodGet, "/feed.json", truncatedHeaders.Get("ETag"), "",
		http.StatusNotModified)
	if len(unchanged) != 0 {
		t.Fatal("304 has a response body")
	}
	if err := database.DeleteExpiredEntries(t.Context(), pool, 30); err != nil {
		t.Fatal(err)
	}
	retainedBody, _ := request(http.MethodGet, "/feed.json", "", "", http.StatusOK)
	if !bytes.Equal(retainedBody, completeBody) {
		t.Fatal("retention did not restore the complete feed")
	}
	checkByteLimit := func(contentHTML string) {
		t.Helper()
		if _, err := pool.Exec(t.Context(), "UPDATE entries SET content_html = $1", contentHTML); err != nil {
			t.Fatal(err)
		}
		largeBody, _ := request(http.MethodGet, "/feed.json", "", "", http.StatusOK)
		large := decode(largeBody)
		if !large.Metadata.Truncated || len(large.Items) == 0 || len(large.Items) >= feedLimit || len(largeBody) > maxFeedBytes {
			t.Fatalf("byte-limited feed: truncated = %v, items = %d, bytes = %d",
				large.Metadata.Truncated, len(large.Items), len(largeBody))
		}
	}
	// The stored HTML exceeds the SQL budget, independently of the item count limit.
	checkByteLimit("<p>" + strings.Repeat("x", 32*1024) + "</p>")
	// Raw HTML fits the SQL budget, but JSON escaping of U+2028 exceeds the response limit.
	checkByteLimit("<p>" + strings.Repeat("\u2028", 8*1024) + "</p>")
	request(http.MethodPost, "/api/v1/entries", "", `{"content_html":"<p>new report</p>"}`, http.StatusCreated)
}
