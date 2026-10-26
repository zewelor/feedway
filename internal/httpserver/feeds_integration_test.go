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

func TestNamedFeedsLifecycle(t *testing.T) {
	createHTTPTestDatabase(t)
	pool, err := database.Open(t.Context(), dataTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Freeze the single-feed schema and a real canonical ID to exercise upgrades.
	const oldID = "sha256-v1:8506cee5fab15db87988e0f15c82fb23e29d31473dfd503fdd5caccb909de3e5"
	if _, err := pool.Exec(t.Context(), `
		CREATE TABLE entries (
			id text PRIMARY KEY CHECK (id ~ '^sha256-v1:[0-9a-f]{64}$'),
			title text,
			content_html text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			data_json json
		);
		INSERT INTO entries (id, title, content_html, data_json, created_at)
		VALUES ('`+oldID+`', 'Daily' || chr(10) || 'report', '<p>one' || chr(10) || 'two   three</p>',
			'{"legacy":true}', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := database.Prepare(t.Context(), pool); err != nil {
			t.Fatal(err)
		}
	}
	var logs bytes.Buffer
	server := httptest.NewServer(databaseHandler(
		Config{APIToken: testAPIToken, FeedResponseMaxEntries: 2},
		pool,
		&readiness{database: pool},
		slog.New(slog.NewTextHandler(&logs, nil)),
	))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, body, etag string, status int) ([]byte, http.Header) {
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
	publish := func(body string, status int) entryResponse {
		t.Helper()
		data, _ := request(http.MethodPost, "/api/v1/entries", body, "", status)
		var result entryResponse
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	type item struct {
		ID          string `json:"id"`
		URL         string `json:"url"`
		Date        string `json:"date_published"`
		Attachments []struct {
			URL string `json:"url"`
		} `json:"attachments"`
	}
	type feedBody struct {
		Title    string `json:"title"`
		Items    []item `json:"items"`
		Metadata struct {
			Truncated bool `json:"truncated"`
		} `json:"_feedway"`
	}
	readFeed := func(path string, count int, truncated bool) (feedBody, http.Header) {
		t.Helper()
		data, headers := request(http.MethodGet, path, "", "", http.StatusOK)
		var result feedBody
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if result.Items == nil || len(result.Items) != count || result.Metadata.Truncated != truncated {
			t.Fatalf("feed %s = %s; want %d items, truncated=%v", path, data, count, truncated)
		}
		return result, headers
	}
	const dailyPath = "/feeds/daily-briefing/feed.json"
	const madeiraPath = "/feeds/madeira-news/feed.json"
	readFeed(dailyPath, 0, false)
	legacy, _ := readFeed("/feed.json", 1, false)
	if legacy.Title != "Feedway" || legacy.Items[0].ID != oldID {
		t.Fatalf("legacy feed = %+v", legacy)
	}
	const legacyBody = `{"title":"Daily\nreport","content_html":"<p>one\ntwo   three</p>"}`
	if retry := publish(legacyBody, http.StatusOK); retry.ID != oldID || retry.Result != "deduplicated" {
		t.Fatalf("legacy retry = %+v", retry)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE entries SET feed = 'daily-briefing' WHERE id = $1", oldID); err != nil {
		t.Fatal(err)
	}
	migrated, _ := readFeed(dailyPath, 1, false)
	before, _ := json.Marshal(legacy.Items[0])
	after, _ := json.Marshal(migrated.Items[0])
	if migrated.Title != "daily-briefing" || !bytes.Equal(before, after) {
		t.Fatalf("migration changed the entry: before=%s, after=%s", before, after)
	}
	if retry := publish(`{"feed":"daily-briefing",`+legacyBody[1:], http.StatusOK); retry.ID != oldID {
		t.Fatalf("migrated retry = %+v", retry)
	}
	data, _ := request(http.MethodGet, "/entries/"+oldID+"/data.json", "", "", http.StatusOK)
	if string(data) != `{"legacy":true}` {
		t.Fatalf("legacy attachment = %s", data)
	}
	request(http.MethodGet, "/entries/"+oldID, "", "", http.StatusOK)
	const content = `"title":"Shared","content_html":"<p>shared</p>"`
	daily := publish(`{"feed":"daily-briefing",`+content+`,"data_json":{"source":"daily"}}`, http.StatusCreated)
	madeira := publish(`{"feed":"madeira-news",`+content+`,"data_json":{"source":"madeira"}}`, http.StatusCreated)
	if daily.ID == madeira.ID || strings.HasPrefix(daily.ID, "sha256-v1:") {
		t.Fatalf("new publication IDs = %q, %q", daily.ID, madeira.ID)
	}
	if retry := publish(`{"feed":"daily-briefing",`+content+`,"data_json":false}`, http.StatusOK); retry.ID != daily.ID {
		t.Fatalf("daily retry = %+v", retry)
	}
	for _, publication := range []struct{ id, data string }{
		{id: daily.ID, data: `{"source":"daily"}`},
		{id: madeira.ID, data: `{"source":"madeira"}`},
	} {
		data, _ := request(http.MethodGet, "/entries/"+publication.id+"/data.json", "", "", http.StatusOK)
		if string(data) != publication.data {
			t.Fatalf("attachment %s = %s", publication.id, data)
		}
	}
	dailyFeed, dailyHeaders := readFeed(dailyPath, 2, false)
	if dailyFeed.Items[0].ID != daily.ID || dailyFeed.Items[0].URL != "/entries/"+daily.ID ||
		dailyFeed.Items[0].Attachments[0].URL != "/entries/"+daily.ID+"/data.json" {
		t.Fatalf("named feed permalinks = %+v", dailyFeed.Items[0])
	}
	for i := range 3 {
		publish(`{"feed":"madeira-news","content_html":"<p>`+strconv.Itoa(i)+`</p>"}`, http.StatusCreated)
	}
	readFeed(madeiraPath, 2, true)
	readFeed("/feed.json", 2, true)
	unchanged, _ := request(http.MethodGet, dailyPath, "", dailyHeaders.Get("ETag"), http.StatusNotModified)
	if len(unchanged) != 0 {
		t.Fatal("304 contains a body")
	}
	head, headHeaders := request(http.MethodHead, dailyPath, "", "", http.StatusOK)
	if len(head) != 0 || headHeaders.Get("ETag") != dailyHeaders.Get("ETag") ||
		headHeaders.Get("Content-Length") != dailyHeaders.Get("Content-Length") {
		t.Fatalf("HEAD mismatch: %v", headHeaders)
	}
	// Corrections retain identity, then publication into both source and target remains valid.
	if _, err := pool.Exec(t.Context(), "UPDATE entries SET feed = 'corrected' WHERE id = $1", daily.ID); err != nil {
		t.Fatal(err)
	}
	corrected := publish(`{"feed":"corrected",`+content+`}`, http.StatusOK)
	replacement := publish(`{"feed":"daily-briefing",`+content+`}`, http.StatusCreated)
	if corrected.ID != daily.ID || replacement.ID == daily.ID {
		t.Fatalf("correction IDs: original=%s corrected=%s replacement=%s", daily.ID, corrected.ID, replacement.ID)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE entries SET feed = 'madeira-news' WHERE id = $1", daily.ID); err == nil {
		t.Fatal("conflicting reassignment succeeded")
	}
	readFeed("/feeds/corrected/feed.json", 1, false)
	// Omission and null share the same unassigned deduplication scope.
	unassigned := publish(`{`+content+`}`, http.StatusCreated)
	if retry := publish(`{"feed":null,`+content+`}`, http.StatusOK); retry.ID != unassigned.ID {
		t.Fatalf("unassigned retry = %+v", retry)
	}
	for _, value := range []string{
		`""`, `"Daily"`, `" daily"`, `"daily "`, `"a--b"`, `"-a"`, `"a-"`, `"ą"`, `"a_b"`, `"a/b"`,
		`"` + strings.Repeat("a", 65) + `"`,
	} {
		request(http.MethodPost, "/api/v1/entries", `{"feed":`+value+`,`+content+`}`, "", http.StatusUnprocessableEntity)
	}
	for _, value := range []string{`12`, `false`, `{}`, `[]`} {
		request(http.MethodPost, "/api/v1/entries", `{"feed":`+value+`,`+content+`}`, "", http.StatusBadRequest)
	}
	for _, slug := range []string{"Daily", "a--b", "a_b", strings.Repeat("a", 65)} {
		request(http.MethodGet, "/feeds/"+slug+"/feed.json", "", "", http.StatusBadRequest)
	}
	for _, slug := range []string{"0", strings.Repeat("a", 64)} {
		publish(`{"feed":"`+slug+`",`+content+`}`, http.StatusCreated)
		feed, _ := readFeed("/feeds/"+slug+"/feed.json", 1, false)
		if feed.Title != slug {
			t.Fatalf("feed title = %q, want %q", feed.Title, slug)
		}
	}
	if err := database.Prepare(t.Context(), pool); err != nil {
		t.Fatalf("schema preparation after named publications: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `
		UPDATE entries SET created_at = now() - interval '31 days' WHERE feed = 'corrected'
	`); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteExpiredEntries(t.Context(), pool, 30); err != nil {
		t.Fatal(err)
	}
	readFeed("/feeds/corrected/feed.json", 0, false)
	request(http.MethodGet, "/entries/"+daily.ID, "", "", http.StatusNotFound)
	request(http.MethodGet, "/entries/"+daily.ID+"/data.json", "", "", http.StatusNotFound)
	if fresh := publish(`{"feed":"corrected",`+content+`}`, http.StatusCreated); fresh.ID == daily.ID {
		t.Fatal("publication after retention reused the old ID")
	}
}
