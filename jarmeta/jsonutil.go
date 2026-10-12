package jarmeta

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// stringList decodes a JSON string or array of strings.
type stringList []string

// UnmarshalJSON implements json.Unmarshaler.
func (s *stringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*s = stringList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("expected string or array of strings: %w", err)
	}
	*s = many
	return nil
}

// unmarshalLenient decodes data like json.Unmarshal, but when strict decoding
// fails it retries after escaping raw control characters inside string
// literals. Fabric's loader accepts such files, so mods in the wild ship them.
func unmarshalLenient(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	if err == nil {
		return nil
	}
	fixed, changed := escapeControlChars(data)
	if !changed {
		return err
	}
	if err2 := json.Unmarshal(fixed, v); err2 != nil {
		return err
	}
	return nil
}

// escapeControlChars escapes raw control characters found inside JSON string
// literals and reports whether anything changed.
func escapeControlChars(data []byte) ([]byte, bool) {
	out := make([]byte, 0, len(data)+16)
	inString, escaped, changed := false, false, false
	for _, c := range data {
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			case c < 0x20:
				out = append(out, fmt.Sprintf("\\u%04x", c)...)
				changed = true
				continue
			}
		} else if c == '"' {
			inString = true
		}
		out = append(out, c)
	}
	return out, changed
}
