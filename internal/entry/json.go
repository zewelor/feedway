package entry

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const maxDataJSONBytes = 256 * 1024

// JSON is validated JSON data. Its zero value means no attachment.
type JSON struct {
	value string
}

func (j JSON) String() string {
	return j.value
}

func parseJSON(raw json.RawMessage) (JSON, error) {
	if len(raw) == 0 {
		return JSON{}, nil
	}
	if len(raw) > maxDataJSONBytes {
		return JSON{}, errors.New("data_json must not exceed 256 KiB")
	}
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return JSON{}, errors.New("data_json must be valid JSON encoded as UTF-8")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return JSON{}, nil
	}
	return JSON{value: string(raw)}, nil
}
