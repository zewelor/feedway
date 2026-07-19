package entry

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

const (
	maxTitleCharacters  = 1000
	maxContentHTMLBytes = 256 * 1024
)

var htmlPolicy = bluemonday.UGCPolicy()

type Values struct {
	ID          string
	Title       *string
	ContentHTML HTML
}

type HTML struct {
	value string
}

func (h HTML) String() string {
	return h.value
}

func Normalize(title, contentHTML string) (Values, error) {
	title = normalizeWhitespace(title)
	if utf8.RuneCountInString(title) > maxTitleCharacters {
		return Values{}, errors.New("title must not exceed 1000 characters")
	}

	contentHTML = normalizeWhitespace(contentHTML)
	if contentHTML == "" {
		return Values{}, errors.New("content_html is required")
	}
	if len(contentHTML) > maxContentHTMLBytes {
		return Values{}, errors.New("content_html must not exceed 256 KiB")
	}

	contentHTML = strings.TrimSpace(htmlPolicy.Sanitize(contentHTML))
	if contentHTML == "" {
		return Values{}, errors.New("content_html is empty after sanitization")
	}
	if len(contentHTML) > maxContentHTMLBytes {
		return Values{}, errors.New("sanitized content_html must not exceed 256 KiB")
	}

	var normalizedTitle *string
	if title != "" {
		normalizedTitle = &title
	}

	return Values{
		ID:          contentID(title, contentHTML),
		Title:       normalizedTitle,
		ContentHTML: HTML{value: contentHTML},
	}, nil
}

func ParseHTML(contentHTML string) (HTML, error) {
	if contentHTML == "" {
		return HTML{}, errors.New("content_html is required")
	}
	if len(contentHTML) > maxContentHTMLBytes {
		return HTML{}, errors.New("content_html must not exceed 256 KiB")
	}
	if strings.TrimSpace(htmlPolicy.Sanitize(contentHTML)) != contentHTML {
		return HTML{}, errors.New("content_html is not sanitized")
	}

	return HTML{value: contentHTML}, nil
}

func contentID(title, contentHTML string) string {
	input := []byte("feedway-entry-v1")
	input = binary.BigEndian.AppendUint64(input, uint64(len(title)))
	input = append(input, title...)
	input = binary.BigEndian.AppendUint64(input, uint64(len(contentHTML)))
	input = append(input, contentHTML...)
	sum := sha256.Sum256(input)

	return "sha256-v1:" + hex.EncodeToString(sum[:])
}

func normalizeWhitespace(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	return strings.TrimSpace(value)
}
