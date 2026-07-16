package entry_test

import (
	"strings"
	"testing"

	"github.com/zewelor/feedway/internal/entry"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	got, err := entry.Normalize(
		"  Daily\rreport  ",
		" \r\n<p>one\r\ntwo   three</p>\r ",
	)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	const wantID = "sha256-v1:8506cee5fab15db87988e0f15c82fb23e29d31473dfd503fdd5caccb909de3e5"
	if got.ID != wantID {
		t.Errorf("ID = %q, want %q", got.ID, wantID)
	}
	if got.Title == nil {
		t.Fatal("Title = nil, want a title")
	}
	if *got.Title != "Daily\nreport" {
		t.Errorf("Title = %q, want %q", *got.Title, "Daily\nreport")
	}
	if got.ContentHTML != "<p>one\ntwo   three</p>" {
		t.Errorf(
			"ContentHTML = %q, want %q",
			got.ContentHTML,
			"<p>one\ntwo   three</p>",
		)
	}
}

func TestNormalize_EmptyTitleBecomesNil(t *testing.T) {
	t.Parallel()

	got, err := entry.Normalize(" \r\n ", "<p>content</p>")
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got.Title != nil {
		t.Errorf("Title = %q, want nil", *got.Title)
	}
}

func TestNormalize_SanitizesUntrustedHTML(t *testing.T) {
	t.Parallel()

	got, err := entry.Normalize(
		"",
		`<style>body { display: none }</style>`+
			`<p style="color: red" onclick="alert(1)">Hello<script>alert(1)</script>`+
			`<a href="javascript:alert(1)">bad</a>`+
			`<a href="https://example.com">safe</a></p>`,
	)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	for _, unsafe := range []string{
		"onclick",
		"<script",
		"javascript:",
		"<style",
		`style="`,
	} {
		if strings.Contains(got.ContentHTML, unsafe) {
			t.Errorf("ContentHTML contains unsafe %q: %q", unsafe, got.ContentHTML)
		}
	}
	if !strings.Contains(
		got.ContentHTML,
		`<a href="https://example.com" rel="nofollow">safe</a>`,
	) {
		t.Errorf("ContentHTML does not preserve the safe link: %q", got.ContentHTML)
	}
}

func TestNormalize_ValidatesInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		title       string
		contentHTML string
		wantError   string
	}{
		{
			name:        "title character limit",
			title:       strings.Repeat("ą", 1001),
			contentHTML: "<p>content</p>",
			wantError:   "title must not exceed 1000 characters",
		},
		{
			name:        "content is required",
			contentHTML: " \r\n ",
			wantError:   "content_html is required",
		},
		{
			name:        "content byte limit before sanitization",
			contentHTML: strings.Repeat("x", 256*1024+1),
			wantError:   "content_html must not exceed 256 KiB",
		},
		{
			name:        "empty content after sanitization",
			contentHTML: "<script>alert(1)</script>",
			wantError:   "content_html is empty after sanitization",
		},
		{
			name:        "content byte limit after sanitization",
			contentHTML: strings.Repeat(`<a href="https://x">x</a>`, 7000),
			wantError:   "sanitized content_html must not exceed 256 KiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := entry.Normalize(tt.title, tt.contentHTML)
			if err == nil {
				t.Fatal("Normalize() error = nil, want an error")
			}
			if err.Error() != tt.wantError {
				t.Errorf("Normalize() error = %q, want %q", err, tt.wantError)
			}
		})
	}
}

func TestNormalize_AcceptsLimits(t *testing.T) {
	t.Parallel()

	got, err := entry.Normalize(
		strings.Repeat("ą", 1000),
		strings.Repeat("x", 256*1024),
	)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(got.ContentHTML) != 256*1024 {
		t.Errorf("ContentHTML size = %d, want %d", len(got.ContentHTML), 256*1024)
	}
}

func TestNormalize_GeneratesDeterministicIDFromFinalContent(t *testing.T) {
	t.Parallel()

	first, err := entry.Normalize(
		" title\r\n",
		`<p onclick="alert(1)">content</p>`,
	)
	if err != nil {
		t.Fatalf("first Normalize() error = %v", err)
	}
	second, err := entry.Normalize(
		"title",
		`<p>content</p>`,
	)
	if err != nil {
		t.Fatalf("second Normalize() error = %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("IDs differ: %q and %q", first.ID, second.ID)
	}

	changed, err := entry.Normalize("different title", `<p>content</p>`)
	if err != nil {
		t.Fatalf("changed Normalize() error = %v", err)
	}
	if first.ID == changed.ID {
		t.Errorf("ID = %q for different final content", first.ID)
	}

	left, err := entry.Normalize("ab", "c")
	if err != nil {
		t.Fatalf("left Normalize() error = %v", err)
	}
	right, err := entry.Normalize("a", "bc")
	if err != nil {
		t.Fatalf("right Normalize() error = %v", err)
	}
	if left.ID == right.ID {
		t.Errorf("ID = %q for different field boundaries", left.ID)
	}
}
