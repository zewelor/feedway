package httpserver

import (
	"bytes"
	"context"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zewelor/feedway/internal/entry"
)

//go:embed home.html
var homeHTML string

var homeTemplate = template.Must(template.New("home").Parse(homeHTML))

type loadFeeds func(context.Context) ([]entry.Feed, error)

type homeFeed struct {
	Feed entry.Feed
	Name string
}

func homePage(load loadFeeds, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		identifiers, err := load(request.Context())
		if err != nil {
			logger.ErrorContext(request.Context(), "load feeds", "error", err)
			http.Error(response, "internal server error", http.StatusInternalServerError)
			return
		}
		feeds := make([]homeFeed, 0, len(identifiers))
		for _, identifier := range identifiers {
			words := strings.Split(identifier.String(), "-")
			for i, word := range words {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
			feeds = append(feeds, homeFeed{Feed: identifier, Name: strings.Join(words, " ")})
		}
		var body bytes.Buffer
		if err := homeTemplate.Execute(&body, feeds); err != nil {
			logger.ErrorContext(request.Context(), "render home page", "error", err)
			http.Error(response, "internal server error", http.StatusInternalServerError)
			return
		}
		writeHTML(response, request, body.Bytes())
	}
}
