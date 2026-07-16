package jsonfeed

import (
	"encoding/json"
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

func Marshal(entries []entry.Published) ([]byte, error) {
	items := make([]item, 0, len(entries))
	for _, published := range entries {
		items = append(items, item{
			ID:            published.ID,
			Title:         published.Title,
			ContentHTML:   published.ContentHTML,
			DatePublished: published.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	return json.Marshal(feed{
		Version: version,
		Title:   "Feedway",
		Items:   items,
	})
}
