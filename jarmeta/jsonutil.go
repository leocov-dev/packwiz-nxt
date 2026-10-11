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
