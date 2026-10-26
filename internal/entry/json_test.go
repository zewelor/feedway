package entry_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zewelor/feedway/internal/entry"
)

func TestNormalizePreservesDataJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "absent"},
		{name: "null", input: " null "},
		{name: "empty object", input: "{}", want: "{}"},
		{name: "empty array", input: "[]", want: "[]"},
		{name: "boolean", input: "false", want: "false"},
		{name: "string", input: `"<script>data</script>"`, want: `"<script>data</script>"`},
		{name: "number", input: "9007199254740993", want: "9007199254740993"},
		{
			name: "document preserved without decoding",
			input: ` { "z": [false,null], "n":9007199254740993,"decimal":0.1234567890123456789,` +
				`"escaped":"\u0000","text":"<script>data</script>","a":1,"a":2 } `,
			want: ` { "z": [false,null], "n":9007199254740993,"decimal":0.1234567890123456789,` +
				`"escaped":"\u0000","text":"<script>data</script>","a":1,"a":2 } `,
		},
		{name: "byte limit", input: `"` + strings.Repeat("x", 256*1024-2) + `"`,
			want: `"` + strings.Repeat("x", 256*1024-2) + `"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			values, err := entry.Normalize("title", "<p>report</p>", json.RawMessage(test.input))
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got := values.DataJSON.String(); got != test.want {
				t.Errorf("data_json = %.200q, want %.200q", got, test.want)
			}
		})
	}
}

func TestNormalizeRejectsInvalidDataJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "invalid json", input: `{"a":`},
		{name: "invalid utf8", input: "\"\xff\""},
		{name: "literal nul", input: "\"\x00\""},
		{name: "byte limit", input: `"` + strings.Repeat("x", 256*1024-1) + `"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := entry.Normalize("", "<p>report</p>", json.RawMessage(test.input)); err == nil {
				t.Fatal("Normalize() error = nil, want invalid data_json")
			}
		})
	}
}

func TestNormalizeDataJSONDoesNotAffectIdentity(t *testing.T) {
	t.Parallel()

	without, err := entry.Normalize("title", "<p>report</p>", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		"null", "{}", `{"a":1}`, ` { "a": 2 } `,
	} {
		values, err := entry.Normalize("title", "<p>report</p>", json.RawMessage(data))
		if err != nil {
			t.Fatal(err)
		}
		if values.ContentHash != without.ContentHash {
			t.Errorf("data_json %q changed the content hash from %q to %q", data, without.ContentHash, values.ContentHash)
		}
	}
}
