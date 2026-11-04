package jsonfeed_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/entry"
	"github.com/zewelor/feedway/internal/jsonfeed"
)

func TestMarshalEmptyFeed(t *testing.T) {
	t.Parallel()

	got, err := jsonfeed.Marshal(nil, "", nil, 200, 1024)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"version":"https://jsonfeed.org/version/1.1","title":"Feedway","items":[],"_feedway":{"truncated":false}}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshalReportsTruncation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		count         int
		wantCount     int
		wantTruncated bool
		limit         int
	}{
		{name: "empty", limit: 100},
		{name: "below limit", count: 99, wantCount: 99, limit: 100},
		{name: "exact limit", count: 100, wantCount: 100, limit: 100},
		{name: "older entries omitted", count: 101, wantCount: 100, wantTruncated: true, limit: 100},
		{name: "custom count", count: 4, wantCount: 3, wantTruncated: true, limit: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			entries := make([]entry.Published, test.count)
			for i := range entries {
				entries[i] = entry.Published{
					ID:          "entry-" + strconv.Itoa(i),
					ContentHTML: mustParseHTML(t, "<p>report</p>"),
				}
			}
			body, err := jsonfeed.Marshal(entries, "", nil, test.limit, 1024*1024)
			if err != nil {
				t.Fatal(err)
			}
			var feed struct {
				Items    []json.RawMessage `json:"items"`
				Metadata *struct {
					Truncated bool `json:"truncated"`
				} `json:"_feedway"`
			}
			if err := json.Unmarshal(body, &feed); err != nil {
				t.Fatal(err)
			}
			if feed.Metadata == nil {
				t.Fatal("feed has no truncation signal")
			}
			if len(feed.Items) != test.wantCount || feed.Metadata.Truncated != test.wantTruncated {
				t.Fatalf("items = %d, truncated = %v; want %d, %v",
					len(feed.Items), feed.Metadata.Truncated, test.wantCount, test.wantTruncated)
			}
		})
	}
}

func TestMarshalFeedURLs(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{"", "https://feed.example.com"} {
		for _, name := range []string{"", "daily-briefing", "madeira-news"} {
			t.Run(baseURL+"/"+name, func(t *testing.T) {
				t.Parallel()

				var selected *entry.Feed
				path := "/feed.json"
				if name != "" {
					parsed, err := entry.ParseFeed(name)
					if err != nil {
						t.Fatal(err)
					}
					selected = &parsed
					path = "/feeds/" + name + "/feed.json"
				}
				body, err := jsonfeed.Marshal(nil, baseURL, selected, 100, 4096)
				if err != nil {
					t.Fatal(err)
				}
				var result map[string]json.RawMessage
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatal(err)
				}
				for field, want := range map[string]string{
					"home_page_url": baseURL + "/",
					"feed_url":      baseURL + path,
				} {
					value, found := result[field]
					if baseURL == "" {
						if found {
							t.Errorf("%s must be omitted without BASE_URL: %s", field, value)
						}
						continue
					}
					if !found {
						t.Errorf("missing %s", field)
						continue
					}
					var got string
					if err := json.Unmarshal(value, &got); err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Errorf("%s = %s, want %q", field, value, want)
					}
				}
			})
		}
	}
}

func TestMarshalEntries(t *testing.T) {
	t.Parallel()

	title := "Daily report"
	firstHTML := mustParseHTML(t, "<p>first</p>")
	secondHTML := mustParseHTML(t, "<p>second</p>")
	got, err := jsonfeed.Marshal([]entry.Published{
		{
			ID:          "sha256-v1:first",
			Title:       &title,
			ContentHTML: firstHTML,
			CreatedAt:   time.Date(2026, 7, 16, 12, 30, 45, 123000000, time.FixedZone("WEST", 3600)),
		},
		{
			ID:          "sha256-v1:second",
			ContentHTML: secondHTML,
			CreatedAt:   time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		},
	}, "https://feed.example.com", nil, 200, 1024)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"version":"https://jsonfeed.org/version/1.1","title":"Feedway",` +
		`"home_page_url":"https://feed.example.com/","feed_url":"https://feed.example.com/feed.json","items":[` +
		`{"id":"sha256-v1:first","url":"https://feed.example.com/entries/sha256-v1:first",` +
		`"title":"Daily report",` +
		`"content_html":"\u003cp\u003efirst\u003c/p\u003e",` +
		`"date_published":"2026-07-16T11:30:45.123Z"},` +
		`{"id":"sha256-v1:second","url":"https://feed.example.com/entries/sha256-v1:second",` +
		`"content_html":"\u003cp\u003esecond\u003c/p\u003e",` +
		`"date_published":"2026-07-16T10:00:00Z"}],"_feedway":{"truncated":false}}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshalUsesRelativeEntryURLWithoutBaseURL(t *testing.T) {
	t.Parallel()

	got, err := jsonfeed.Marshal([]entry.Published{
		{
			ID:          "sha256-v1:first",
			ContentHTML: mustParseHTML(t, "<p>first</p>"),
			CreatedAt:   time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		},
	}, "", nil, 200, 1024)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `"url":"/entries/sha256-v1:first"`
	if !strings.Contains(string(got), want) {
		t.Errorf("Marshal() = %s, want containing %s", got, want)
	}
}

func TestMarshalByteBoundaries(t *testing.T) {
	t.Parallel()
	for _, baseURL := range []string{"", "https://feed.example.com"} {
		for _, name := range []string{"", "daily-briefing"} {
			t.Run(baseURL+"/"+name, func(t *testing.T) {
				t.Parallel()
				var selected *entry.Feed
				if name != "" {
					parsed, err := entry.ParseFeed(name)
					if err != nil {
						t.Fatal(err)
					}
					selected = &parsed
				}
				testMarshalByteBoundaries(t, baseURL, selected)
			})
		}
	}
}

func testMarshalByteBoundaries(t *testing.T, baseURL string, selected *entry.Feed) {
	t.Helper()
	entries := []entry.Published{
		{ID: "first", ContentHTML: mustParseHTML(t, "<p>first</p>")},
		{ID: "second", ContentHTML: mustParseHTML(t, "<p>second</p>")},
	}
	first, err := jsonfeed.Marshal(entries[:1], baseURL, selected, 200, 4096)
	if err != nil {
		t.Fatal(err)
	}
	complete, err := jsonfeed.Marshal(entries, baseURL, selected, 200, 4096)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := jsonfeed.Marshal(entries, baseURL, selected, 200, len(complete))
	if err != nil || string(exact) != string(complete) {
		t.Fatalf("complete feed at exact byte limit: error = %v, body = %s", err, exact)
	}
	for _, test := range []struct {
		name      string
		limit     int
		wantCount int
	}{
		{name: "newest entries that fit", limit: len(first), wantCount: 1},
		{name: "complete feed one byte too large", limit: len(complete) - 1, wantCount: 1},
		{name: "truncated feed exact byte limit", limit: len(first) - 1, wantCount: 1},
		{name: "newest item cannot fit", limit: len(first) - 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body, err := jsonfeed.Marshal(entries, baseURL, selected, 200, test.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(body) > test.limit {
				t.Fatalf("feed size = %d, limit = %d", len(body), test.limit)
			}
			assertTruncatedItems(t, body, entries[:test.wantCount])
		})
	}
}

func mustParseHTML(t *testing.T, value string) entry.HTML {
	t.Helper()

	html, err := entry.ParseHTML(value)
	if err != nil {
		t.Fatalf("ParseHTML() error = %v", err)
	}

	return html
}

func TestMarshalRejectsLimitSmallerThanEmptyFeed(t *testing.T) {
	t.Parallel()

	if _, err := jsonfeed.Marshal(nil, "", nil, 200, 1); err == nil {
		t.Fatal("Marshal() error = nil, want size limit error")
	}
}

func TestMarshalDataAttachments(t *testing.T) {
	t.Parallel()
	for _, baseURL := range []string{"", "https://feed.example.com"} {
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()
			entries := []entry.Published{
				{ID: "with-data", ContentHTML: mustParseHTML(t, "<p>report</p>"), HasDataJSON: true},
				{ID: "without-data", ContentHTML: mustParseHTML(t, "<p>legacy</p>")},
			}
			body, err := jsonfeed.Marshal(entries, baseURL, nil, 200, 4096)
			if err != nil {
				t.Fatal(err)
			}
			var feed struct {
				Items []struct {
					Attachments []struct {
						URL      string `json:"url"`
						MIMEType string `json:"mime_type"`
					} `json:"attachments"`
				} `json:"items"`
			}
			if err := json.Unmarshal(body, &feed); err != nil {
				t.Fatal(err)
			}
			if len(feed.Items) != 2 || len(feed.Items[0].Attachments) != 1 || len(feed.Items[1].Attachments) != 0 {
				t.Fatalf("unexpected attachments: %s", body)
			}
			attachment := feed.Items[0].Attachments[0]
			if attachment.URL != baseURL+"/entries/with-data/data.json" || attachment.MIMEType != "application/json" {
				t.Errorf("attachment = %+v", attachment)
			}
			newest, err := jsonfeed.Marshal(entries[:1], baseURL, nil, 200, len(body))
			if err != nil {
				t.Fatal(err)
			}
			limited, err := jsonfeed.Marshal(entries, baseURL, nil, 200, len(newest))
			if err != nil {
				t.Fatal(err)
			}
			assertTruncatedItems(t, limited, entries[:1])
			var limitedFeed struct {
				Items []struct {
					Attachments []json.RawMessage `json:"attachments"`
				} `json:"items"`
			}
			if err := json.Unmarshal(limited, &limitedFeed); err != nil {
				t.Fatal(err)
			}
			if len(limitedFeed.Items[0].Attachments) != 1 {
				t.Fatal("attachment metadata was lost when truncating")
			}
		})
	}
}

func assertTruncatedItems(t *testing.T, body []byte, want []entry.Published) {
	t.Helper()
	var feed struct {
		Items []struct {
			ID          string `json:"id"`
			ContentHTML string `json:"content_html"`
		} `json:"items"`
		Metadata struct {
			Truncated bool `json:"truncated"`
		} `json:"_feedway"`
	}
	if err := json.Unmarshal(body, &feed); err != nil {
		t.Fatal(err)
	}
	if !feed.Metadata.Truncated || len(feed.Items) != len(want) {
		t.Fatalf("truncated = %v, items = %d; want true, %d", feed.Metadata.Truncated, len(feed.Items), len(want))
	}
	for i, published := range want {
		if feed.Items[i].ID != published.ID || feed.Items[i].ContentHTML != published.ContentHTML.String() {
			t.Errorf("item %d was reordered or its content was changed: %+v", i, feed.Items[i])
		}
	}
}

func TestMarshalRejectsInvalidEntryLimit(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, -1} {
		if _, err := jsonfeed.Marshal(nil, "", nil, limit, 1024); err == nil {
			t.Errorf("Marshal() accepted entry limit %d", limit)
		}
	}
}
