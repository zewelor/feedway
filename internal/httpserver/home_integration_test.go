//go:build integration

package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/database"
)

func TestHomepageLifecycle(t *testing.T) {
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
		Config{APIToken: testAPIToken, BaseURL: "https://feed.example.com", FeedResponseMaxEntries: 1},
		pool,
		&readiness{database: pool},
		slog.New(slog.NewTextHandler(&logs, nil)),
	))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(method, path, body string, status int) ([]byte, http.Header) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if method == http.MethodPost && path == "/api/v1/entries" {
			req.Header.Set("Authorization", "Bearer "+testAPIToken)
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read response: %v; close response: %v", readErr, closeErr)
		}
		if response.StatusCode != status {
			t.Fatalf("%s %s = %d, want %d; body = %.300s; logs = %s",
				method, path, response.StatusCode, status, data, &logs)
		}
		return data, response.Header
	}
	readHome := func(want []homeLink) []byte {
		t.Helper()
		body, headers := request(http.MethodGet, "/", "", http.StatusOK)
		if headers.Get("Content-Type") != "text/html; charset=utf-8" ||
			headers.Get("Content-Length") != strconv.Itoa(len(body)) ||
			headers.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("homepage headers = %v", headers)
		}
		page := string(body)
		if got := strings.Count(page, `href="`); got != len(want) {
			t.Fatalf("homepage has %d links, want %d", got, len(want))
		}
		for _, link := range want {
			_, after, found := strings.Cut(page, `href="`+link.URL+`"`)
			if !found {
				t.Fatalf("missing or out-of-order feed link %q", link.URL)
			}
			content, rest, closed := strings.Cut(after, "</a>")
			if !closed || !strings.Contains(content, link.Name) {
				t.Fatalf("feed link %q is missing label %q", link.URL, link.Name)
			}
			page = rest
		}
		head, headHeaders := request(http.MethodHead, "/", "", http.StatusOK)
		if len(head) != 0 || headHeaders.Get("Content-Length") != headers.Get("Content-Length") ||
			headHeaders.Get("Content-Type") != headers.Get("Content-Type") {
			t.Fatalf("HEAD mismatch: body=%q, headers=%v", head, headHeaders)
		}
		return body
	}
	readFeed := func(path string) {
		t.Helper()
		body, _ := request(http.MethodGet, path, "", http.StatusOK)
		var result struct {
			HomePageURL string `json:"home_page_url"`
			FeedURL     string `json:"feed_url"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		if result.HomePageURL != "https://feed.example.com/" || result.FeedURL != "https://feed.example.com"+path {
			t.Fatalf("feed URL metadata for %s = %+v", path, result)
		}
	}
	if body := readHome([]homeLink{}); !strings.Contains(string(body), "No feeds yet.") {
		t.Fatalf("empty homepage = %s", body)
	}
	request(http.MethodPost, "/api/v1/entries", `{"content_html":"<p>Unassigned</p>"}`, http.StatusCreated)
	readFeed("/feed.json")
	readFeed("/feeds/daily-briefing/feed.json")
	readHome([]homeLink{})
	// Publish out of order, with multiple entries belonging to one feed.
	for _, publication := range []string{
		`{"feed":"madeira-news","content_html":"<p>Island news</p>"}`,
		`{"feed":"daily-briefing","content_html":"<p>Morning</p>"}`,
		`{"feed":"daily-briefing","content_html":"<p>Evening</p>"}`,
	} {
		request(http.MethodPost, "/api/v1/entries", publication, http.StatusCreated)
	}
	want := []homeLink{
		{URL: "/feeds/daily-briefing/feed.json", Name: "Daily Briefing"},
		{URL: "/feeds/madeira-news/feed.json", Name: "Madeira News"},
	}
	body := readHome(want)
	artifact := filepath.Join(t.ArtifactDir(), "homepage.html")
	if err := os.WriteFile(artifact, body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("homepage preview: %s", artifact)
	for _, link := range want {
		readFeed(link.URL)
	}
	readFeed("/feed.json")
	request(http.MethodGet, "/missing", "", http.StatusNotFound)
	request(http.MethodGet, "/missing/nested", "", http.StatusNotFound)
	_, headers := request(http.MethodPost, "/", "", http.StatusMethodNotAllowed)
	if headers.Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", headers.Get("Allow"))
	}
	if _, err := pool.Exec(t.Context(), `
		UPDATE entries SET created_at = now() - interval '31 days' WHERE feed = 'madeira-news'
	`); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteExpiredEntries(t.Context(), pool, 30); err != nil {
		t.Fatal(err)
	}
	readHome(want[:1])
	// A database failure must not masquerade as an empty directory.
	pool.Close()
	failed, _ := request(http.MethodGet, "/", "", http.StatusInternalServerError)
	if string(failed) != "internal server error\n" || !strings.Contains(logs.String(), "load feeds") {
		t.Fatalf("database failure: body=%q, logs=%s", failed, &logs)
	}
}

type homeLink struct {
	URL  string
	Name string
}
