// Package timing holds the timing file: the recent durations of each test,
// how runs from several machines fold into it, and when stale tests leave it.
// It reads no files itself; callers pass bytes in and get bytes back.
package timing

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"
)

const (
	// FormatVersion is the only timing file version this build reads and writes.
	FormatVersion = 1

	// MaxSamples is how many recent durations each test keeps.
	MaxSamples = 5

	// PruneAfterRuns is how many merges a test may go unseen before a merge
	// drops it.
	PruneAfterRuns = 30

	// MaxFileBytes bounds an encoded timing file.
	MaxFileBytes = 64 << 20

	// MaxDurationMS bounds one duration. Thirty days is far beyond any real
	// test file, and it keeps the sum of split.MaxTests durations well inside
	// int64, so load totals cannot overflow.
	MaxDurationMS int64 = 30 * 24 * 60 * 60 * 1000
)

var (
	// ErrUnsupportedVersion reports a timing file version other than FormatVersion.
	ErrUnsupportedVersion = errors.New("unsupported timing file version")
	// ErrInvalidFile reports a timing file that is malformed or breaks a limit.
	ErrInvalidFile = errors.New("invalid timing file")
)

// File is a decoded timing file.
type File struct {
	// Run counts merges. Each merge adds one.
	Run int64
	// Tests maps a test identifier to its history.
	Tests map[string]Entry
}

// Entry is the history of one test.
type Entry struct {
	// Samples holds recent durations in milliseconds, oldest first. It is
	// empty for a test that has been seen but has never fully passed.
	Samples []int64 `json:"samples"`
	// LastSeen is the Run of the last merge whose reports included the test.
	LastSeen int64 `json:"last_seen"`
}

type fileJSON struct {
	Version int64            `json:"version"`
	Run     int64            `json:"run"`
	Tests   map[string]Entry `json:"tests"`
}

// Decode parses and validates a timing file.
func Decode(data []byte) (File, error) {
	if len(data) > MaxFileBytes {
		return File{}, fmt.Errorf("%w: %d bytes exceeds the %d byte limit", ErrInvalidFile, len(data), MaxFileBytes)
	}
	// Read the version on its own first. A file from a newer release should
	// fail with its version number, not with an unknown member from a format
	// this build has never seen.
	var header struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}
	if header.Version != FormatVersion {
		return File{}, fmt.Errorf("%w %d; this build reads version %d", ErrUnsupportedVersion, header.Version, FormatVersion)
	}

	var raw fileJSON
	if err := json.Unmarshal(data, &raw, json.RejectUnknownMembers(true)); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}
	f := File{Run: raw.Run, Tests: raw.Tests}
	if f.Tests == nil {
		f.Tests = map[string]Entry{}
	}
	if err := f.validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

// Encode validates f and returns its JSON form. The output is the same bytes
// for the same file, so caches and diffs stay stable.
func Encode(f File) ([]byte, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	tests := f.Tests
	if tests == nil {
		tests = map[string]Entry{}
	}
	data, err := json.Marshal(fileJSON{Version: FormatVersion, Run: f.Run, Tests: tests}, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode timing file: %w", err)
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%w: encoded size %d bytes exceeds the %d byte limit", ErrInvalidFile, len(data), MaxFileBytes)
	}
	return data, nil
}

// Durations returns the median duration of every test that has samples.
// Tests seen but never fully passed are left out, so a split treats them as
// unknown.
func (f File) Durations() map[string]int64 {
	out := make(map[string]int64, len(f.Tests))
	for id, e := range f.Tests {
		if ms, ok := Median(e.Samples); ok {
			out[id] = ms
		}
	}
	return out
}

func (f File) validate() error {
	if f.Run < 0 {
		return fmt.Errorf("%w: run %d is negative", ErrInvalidFile, f.Run)
	}
	// Check in sorted order so the same bad file always reports the same test.
	for _, id := range slices.Sorted(maps.Keys(f.Tests)) {
		e := f.Tests[id]
		if len(e.Samples) > MaxSamples {
			return fmt.Errorf("%w: test %q has %d samples; the limit is %d", ErrInvalidFile, id, len(e.Samples), MaxSamples)
		}
		for _, ms := range e.Samples {
			if ms < 0 || ms > MaxDurationMS {
				return fmt.Errorf("%w: test %q has duration %d ms outside 0 to %d", ErrInvalidFile, id, ms, MaxDurationMS)
			}
		}
		if e.LastSeen < 0 || e.LastSeen > f.Run {
			return fmt.Errorf("%w: test %q was last seen at run %d, outside 0 to %d", ErrInvalidFile, id, e.LastSeen, f.Run)
		}
	}
	return nil
}

// Median returns the median of values. For an even count it averages the two
// middle values and rounds half up, so it matches a spreadsheet's median.
// It reports false when values is empty. Values must not be negative.
func Median(values []int64) (int64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	if sorted[0] < 0 {
		panic(fmt.Sprintf("timing.Median: negative value %d", sorted[0]))
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid], true
	}
	lo, hi := sorted[mid-1], sorted[mid]
	// Both are nonnegative, so hi-lo cannot overflow, and the result stays
	// between lo and hi. Adding the remainder rounds half up.
	d := hi - lo
	return lo + d/2 + d%2, true
}
