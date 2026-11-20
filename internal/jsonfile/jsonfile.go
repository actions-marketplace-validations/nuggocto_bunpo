// Package jsonfile reads and writes Bunpō's versioned JSON files with one
// set of strict rules.
package jsonfile

import (
	"encoding/json/v2"
	"errors"
	"fmt"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported version")
	ErrInvalid            = errors.New("invalid file")
)

// Decode reads v, which must have a "version" member. The version is read
// first, so a newer file fails with its version rather than an unknown
// member. Unknown members, duplicate names and invalid UTF-8 are errors.
func Decode(kind string, data []byte, maxBytes int, version int64, v any) error {
	if len(data) > maxBytes {
		return fmt.Errorf("%w: %s is %d bytes, over the %d byte limit", ErrInvalid, kind, len(data), maxBytes)
	}

	var header struct {
		Version int64 `json:"version"`
	}

	if err := json.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalid, kind, err)
	}

	if header.Version != version {
		return fmt.Errorf("%w: %s version %d; this build reads version %d", ErrUnsupportedVersion, kind, header.Version, version)
	}

	if err := json.Unmarshal(data, v, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalid, kind, err)
	}

	return nil
}

// Member names a required JSON member and whether the file had it.
type Member struct {
	Name    string
	Present bool
}

// Require fails on the first absent member. Decoding fills absent members
// with zero values, which would let an incomplete file pass for a valid one.
func Require(kind string, members ...Member) error {
	for _, m := range members {
		if !m.Present {
			return fmt.Errorf("%w: %s has no %q member", ErrInvalid, kind, m.Name)
		}
	}
	return nil
}

// Encode sorts map keys, so the same value always gives the same bytes.
func Encode(kind string, v any, maxBytes int) ([]byte, error) {
	data, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", kind, err)
	}

	if len(data) > maxBytes {
		return nil, fmt.Errorf("%w: encoded %s is %d bytes, over the %d byte limit", ErrInvalid, kind, len(data), maxBytes)
	}

	return data, nil
}
