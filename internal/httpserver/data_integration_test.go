//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/database"
)

var dataTestConfig = database.Config{
	Host: "postgres", Port: 5432, Name: "feedway_http_test",
	User: "feedway", Password: "feedway_test", SSLMode: "disable",
}

func TestEntryDataLifecycle(t *testing.T) {
	createHTTPTestDatabase(t)
	pool, err := database.Open(t.Context(), dataTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Freeze the pre-MCA-34 schema to exercise upgrade of an existing database.
	const legacySchema = `
		CREATE TABLE entries (
			id text PRIMARY KEY CHECK (id ~ '^sha256-v1:[0-9a-f]{64}$'),
			title text CHECK (title IS NULL OR char_length(title) <= 1000),
			content_html text NOT NULL CHECK (
				nullif(btrim(content_html), '') IS NOT NULL AND octet_length(content_html) <= 262144
			),
			created_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE INDEX entries_created_index ON entries(created_at DESC, id DESC);
		INSERT INTO entries (id, title, content_html)
		VALUES ('sha256-v1:8506cee5fab15db87988e0f15c82fb23e29d31473dfd503fdd5caccb909de3e5',
			'Daily' || chr(10) || 'report', '<p>one' || chr(10) || 'two   three</p>');
	`
	if _, err := pool.Exec(t.Context(), legacySchema); err != nil {
		t.Fatal(err)
	}
	startServer := func() *httptest.Server {
		t.Helper()
		if err := database.Prepare(t.Context(), pool); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(databaseHandler(
			Config{APIToken: testAPIToken, BaseURL: "https://feed.example.com", FeedResponseMaxEntries: 100},
			pool,
			&readiness{database: pool},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		))
		t.Cleanup(server.Close)
		return server
	}
	server := startServer()
	baseURL := server.URL
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(method, path, body string, status int) ([]byte, http.Header) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
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
			t.Fatalf("%s %s = %d, want %d; body = %s", method, path, response.StatusCode, status, data)
		}
		return data, response.Header
	}
	publish := func(body string, status int) entryResponse {
		t.Helper()
		data, _ := request(http.MethodPost, "/api/v1/entries", body, status)
		var result entryResponse
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	const oldID = "sha256-v1:8506cee5fab15db87988e0f15c82fb23e29d31473dfd503fdd5caccb909de3e5"
	old := publish(`{"title":"Daily\nreport","content_html":"<p>one\ntwo   three</p>"}`, http.StatusOK)
	if old.ID != oldID || old.Result != "deduplicated" {
		t.Fatalf("legacy retry = %+v", old)
	}
	request(http.MethodGet, "/entries/"+oldID, "", http.StatusOK)
	request(http.MethodGet, "/entries/"+oldID+"/data.json", "", http.StatusNotFound)
	request(http.MethodGet, "/entries/missing/data.json", "", http.StatusNotFound)
	withData := publish(`{"title":"Daily\nreport","content_html":"<p>one\ntwo   three</p>","data_json":{}}`,
		http.StatusOK)
	if withData.ID != oldID || withData.Result != "deduplicated" {
		t.Fatalf("adding JSON to a legacy retry = %+v", withData)
	}
	request(http.MethodGet, "/entries/"+oldID+"/data.json", "", http.StatusNotFound)

	const reportData = `{"z": [{"text":"<script>data</script>","source":"https://example.org"}],` +
		`"n":9007199254740993,"decimal":0.12345678901234567890123456789,"nul":"\u0000","huge":1e1000000}`
	const report = `{"title":"report","content_html":"<p onclick=\"bad\">Report</p>",` +
		`"data_json":` + reportData + `}`
	created := publish(report, http.StatusCreated)
	retry := publish(`{"title":"report","content_html":"<p>Report</p>","data_json": {`+
		`"nul":"\u0000", "huge":1e1000000,"decimal":0.12345678901234567890123456789,`+
		`"n":9007199254740993,"z":[{"source":"https://example.org","text":"<script>data</script>"}]}}`,
		http.StatusOK)
	if retry.ID != created.ID || retry.Result != "deduplicated" {
		t.Fatalf("retry = %+v, created = %+v", retry, created)
	}
	changedData := publish(strings.Replace(report, "9007199254740993", "9007199254740994", 1), http.StatusOK)
	if changedData.ID != created.ID || changedData.Result != "deduplicated" {
		t.Fatalf("JSON-only change = %+v, created = %+v", changedData, created)
	}
	withoutData := publish(`{"title":"report","content_html":"<p>Report</p>","data_json":null}`, http.StatusOK)
	if withoutData.ID != created.ID || withoutData.Result != "deduplicated" {
		t.Fatalf("retry without JSON = %+v, created = %+v", withoutData, created)
	}
	changed := publish(strings.Replace(report, "Report</p>", "Updated report</p>", 1), http.StatusCreated)
	if changed.ID == created.ID {
		t.Fatal("changed HTML kept the same id")
	}
	page, _ := request(http.MethodGet, "/entries/"+created.ID, "", http.StatusOK)
	if !bytes.Contains(page, []byte("<p>Report</p>")) || bytes.Contains(page, []byte("onclick")) ||
		bytes.Contains(page, []byte("<script>")) {
		t.Fatalf("unsafe or missing HTML: %s", page)
	}
	feedBody, _ := request(http.MethodGet, "/feed.json", "", http.StatusOK)
	var feed struct {
		Items []struct {
			ID          string `json:"id"`
			Attachments []struct {
				URL      string `json:"url"`
				MIMEType string `json:"mime_type"`
			} `json:"attachments"`
		} `json:"items"`
	}
	if err := json.Unmarshal(feedBody, &feed); err != nil {
		t.Fatal(err)
	}
	attachmentURL := ""
	for _, item := range feed.Items {
		if item.ID == oldID && len(item.Attachments) != 0 {
			t.Error("legacy entry has an attachment")
		}
		if item.ID == created.ID && len(item.Attachments) == 1 && item.Attachments[0].MIMEType == "application/json" {
			attachmentURL = item.Attachments[0].URL
		}
	}
	const publicOrigin = "https://feed.example.com"
	if attachmentURL != publicOrigin+"/entries/"+created.ID+"/data.json" {
		t.Fatalf("attachment url = %q; feed = %s", attachmentURL, feedBody)
	}
	path := strings.TrimPrefix(attachmentURL, publicOrigin)
	data, headers := request(http.MethodGet, path, "", http.StatusOK)
	if string(data) != reportData || headers.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("data = %s; headers = %v", data, headers)
	}
	head, headHeaders := request(http.MethodHead, path, "", http.StatusOK)
	if len(head) != 0 || headHeaders.Get("Content-Length") != headers.Get("Content-Length") {
		t.Fatalf("HEAD body = %s; headers = %v", head, headHeaders)
	}

	// Recreate the HTTP server and database connections against the same data.
	server.Close()
	pool.Close()
	pool, err = database.Open(t.Context(), dataTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	server = startServer()
	baseURL = server.URL
	data, _ = request(http.MethodGet, path, "", http.StatusOK)
	if string(data) != reportData {
		t.Fatalf("data after reconnecting = %s", data)
	}
	emptyArray := publish(`{"content_html":"<p>empty array</p>","data_json":[]}`, http.StatusCreated)
	data, _ = request(http.MethodGet, "/entries/"+emptyArray.ID+"/data.json", "", http.StatusOK)
	if string(data) != "[]" {
		t.Fatalf("empty array attachment = %s", data)
	}

	if _, err := pool.Exec(t.Context(), "UPDATE entries SET created_at = now() - interval '61 days' WHERE id = $1",
		created.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteExpiredEntries(t.Context(), pool, 60); err != nil {
		t.Fatal(err)
	}
	request(http.MethodGet, path, "", http.StatusNotFound)
	request(http.MethodGet, "/entries/"+created.ID, "", http.StatusNotFound)
	request(http.MethodHead, path, "", http.StatusNotFound)
	request(http.MethodGet, "/entries/"+changed.ID+"/data.json", "", http.StatusOK)
}

func createHTTPTestDatabase(t *testing.T) {
	t.Helper()
	// A separate database prevents interference with the database package's tests.
	adminConfig := dataTestConfig
	adminConfig.Name = "feedway_test"
	admin, err := database.Open(t.Context(), adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE feedway_http_test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE feedway_http_test"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})
}
