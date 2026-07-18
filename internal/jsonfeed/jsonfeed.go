package jsonfeed

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/zewelor/feedway/internal/entry"
)

const version = "https://jsonfeed.org/version/1.1"

type feed struct {
	Version string `json:"version"`
	Title   string `json:"title"`
	Items   []item `json:"items"`
}

type item struct {
	ID            string  `json:"id"`
	Title         *string `json:"title,omitempty"`
	ContentHTML   string  `json:"content_html"`
	DatePublished string  `json:"date_published"`
}

func Marshal(entries []entry.Published, maxBytes int) ([]byte, error) {
	items := make([]item, 0, len(entries))
	body, err := marshal(items)
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, errors.New("maximum feed size is smaller than an empty feed")
	}

	currentBytes := len(body)
	for _, published := range entries {
		candidate := item{
			ID:            published.ID,
			Title:         published.Title,
			ContentHTML:   published.ContentHTML,
			DatePublished: published.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return nil, err
		}

		separatorBytes := 0
		if len(items) > 0 {
			separatorBytes = 1
		}
		if currentBytes+separatorBytes+len(encoded) > maxBytes {
			break
		}

		items = append(items, candidate)
		currentBytes += separatorBytes + len(encoded)
	}

	return marshal(items)
}

func marshal(items []item) ([]byte, error) {
	return json.Marshal(feed{
		Version: version,
		Title:   "Feedway",
		Items:   items,
	})
}
