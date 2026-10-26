package jsonfeed

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/zewelor/feedway/internal/entry"
)

const version = "https://jsonfeed.org/version/1.1"

type feed struct {
	Version  string   `json:"version"`
	Title    string   `json:"title"`
	Items    []item   `json:"items"`
	Metadata metadata `json:"_feedway"`
}

type metadata struct {
	Truncated bool `json:"truncated"`
}

type item struct {
	ID            string       `json:"id"`
	URL           string       `json:"url"`
	Title         *string      `json:"title,omitempty"`
	ContentHTML   string       `json:"content_html"`
	DatePublished string       `json:"date_published"`
	Attachments   []attachment `json:"attachments,omitempty"`
}

type attachment struct {
	URL      string `json:"url"`
	MIMEType string `json:"mime_type"`
}

func Marshal(entries []entry.Published, baseURL string, maxItems, maxBytes int) ([]byte, error) {
	if maxItems < 1 {
		return nil, errors.New("maximum feed entries must be positive")
	}
	items := make([]item, 0, min(len(entries), maxItems))
	empty, err := marshal(items, false)
	if err != nil {
		return nil, err
	}
	if len(empty) > maxBytes {
		return nil, errors.New("maximum feed size is smaller than an empty feed")
	}
	truncatedEmpty, err := marshal(items, true)
	if err != nil {
		return nil, err
	}

	itemBytes := 0
	truncated := false
	for index, published := range entries {
		if index == maxItems {
			truncated = true
			break
		}
		candidate := item{
			ID:            published.ID,
			URL:           baseURL + "/entries/" + published.ID,
			Title:         published.Title,
			ContentHTML:   published.ContentHTML.String(),
			DatePublished: published.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if published.HasDataJSON {
			candidate.Attachments = []attachment{{
				URL:      candidate.URL + "/data.json",
				MIMEType: "application/json",
			}}
		}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return nil, err
		}

		separatorBytes := 0
		if len(items) > 0 {
			separatorBytes = 1
		}
		candidateBytes := itemBytes + separatorBytes + len(encoded)
		envelopeBytes := len(empty)
		if index+1 < len(entries) {
			envelopeBytes = len(truncatedEmpty)
		}
		if envelopeBytes+candidateBytes > maxBytes {
			truncated = true
			break
		}

		items = append(items, candidate)
		itemBytes = candidateBytes
	}

	return marshal(items, truncated)
}

func marshal(items []item, truncated bool) ([]byte, error) {
	return json.Marshal(feed{
		Version:  version,
		Title:    "Feedway",
		Items:    items,
		Metadata: metadata{Truncated: truncated},
	})
}
