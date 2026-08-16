package core

import (
	"github.com/pelletier/go-toml/v2"
)

// marshalWithHash marshals v to TOML and computes its hash using hashFormat,
// returning a MarshalResult with Value/HashFormat/Hash populated. Callers are
// responsible for storing the resulting hash on their own receiver (e.g. via
// their own UpdateHash method), since where/how that hash is cached differs
// per type.
func marshalWithHash(v any, hashFormat string) (MarshalResult, error) {
	result := MarshalResult{
		HashFormat: hashFormat,
	}

	var err error

	result.Value, err = toml.Marshal(v)
	if err != nil {
		return result, err
	}

	stringer, err := GetHashImpl(result.HashFormat)
	if err != nil {
		return result, err
	}

	if _, err := stringer.Write(result.Value); err != nil {
		return result, err
	}

	result.Hash = stringer.String()

	return result, nil
}
