package jsonfeed_test

import (
	"testing"
	"time"

	"github.com/zewelor/feedway/internal/entry"
	"github.com/zewelor/feedway/internal/jsonfeed"
)

func TestMarshalEmptyFeed(t *testing.T) {
	t.Parallel()

	got, err := jsonfeed.Marshal(nil, 1024)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"version":"https://jsonfeed.org/version/1.1","title":"Feedway","items":[]}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshalEntries(t *testing.T) {
	t.Parallel()

	title := "Daily report"
	got, err := jsonfeed.Marshal([]entry.Published{
		{
			ID:          "sha256-v1:first",
			Title:       &title,
			ContentHTML: "<p>first</p>",
			CreatedAt:   time.Date(2026, 7, 16, 12, 30, 45, 123000000, time.FixedZone("WEST", 3600)),
		},
		{
			ID:          "sha256-v1:second",
			ContentHTML: "<p>second</p>",
			CreatedAt:   time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		},
	}, 1024)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"version":"https://jsonfeed.org/version/1.1","title":"Feedway","items":[` +
		`{"id":"sha256-v1:first","url":"/entries/sha256-v1:first","title":"Daily report",` +
		`"content_html":"\u003cp\u003efirst\u003c/p\u003e",` +
		`"date_published":"2026-07-16T11:30:45.123Z"},` +
		`{"id":"sha256-v1:second","url":"/entries/sha256-v1:second",` +
		`"content_html":"\u003cp\u003esecond\u003c/p\u003e",` +
		`"date_published":"2026-07-16T10:00:00Z"}]}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshalKeepsNewestEntriesThatFit(t *testing.T) {
	t.Parallel()

	entries := []entry.Published{
		{
			ID:          "sha256-v1:first",
			ContentHTML: "<p>first</p>",
			CreatedAt:   time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:          "sha256-v1:second",
			ContentHTML: "<p>second</p>",
			CreatedAt:   time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC),
		},
	}

	newest, err := jsonfeed.Marshal(entries[:1], 1024)
	if err != nil {
		t.Fatalf("Marshal() newest entry error = %v", err)
	}
	got, err := jsonfeed.Marshal(entries, len(newest))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if string(got) != string(newest) {
		t.Errorf("Marshal() = %s, want newest fitting entry %s", got, newest)
	}
}

func TestMarshalRejectsLimitSmallerThanEmptyFeed(t *testing.T) {
	t.Parallel()

	if _, err := jsonfeed.Marshal(nil, 1); err == nil {
		t.Fatal("Marshal() error = nil, want size limit error")
	}
}
