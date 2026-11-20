package junit

import (
	"errors"
	"io"
	"math/big"
	"strings"
	"testing"
)

func TestParseSeconds(t *testing.T) {
	// want is the exact value as a fraction of seconds.
	tests := []struct {
		in   string
		want string
	}{
		{"", "0"},
		{"0", "0"},
		{"0.000", "0"},
		{"1", "1"},
		{"1.5", "3/2"},
		{".5", "1/2"},
		{"1.", "1"},
		{" 0.25 ", "1/4"},
		{"0.000340", "17/50000"},
		{"0.0004999996", "1249999/2500000000"},
		// JavaScript writes tiny numbers with an exponent.
		{"1e-7", "1/10000000"},
		{"2.5E-3", "1/400"},
		{"1.2e+1", "12"},
		{"2592000", "2592000"},
	}

	for _, tc := range tests {
		got, err := parseSeconds(tc.in)
		if err != nil || got.RatString() != tc.want {
			t.Errorf("parseSeconds(%q) = %v, %v; want %s", tc.in, got, err, tc.want)
		}
	}
}

func TestParseSecondsRejects(t *testing.T) {
	for _, in := range []string{
		"-1", "+1", "abc", "1.2.3", "0x10", "NaN", "Inf", "1e", "1e999", ".", "1 2",
		"2592000.000000001", "12345678901234567890123456789012345678901",
	} {
		if got, err := parseSeconds(in); err == nil {
			t.Errorf("parseSeconds(%q) = %v; want an error", in, got)
		}
	}
}

func TestFileMS(t *testing.T) {
	tests := []struct {
		seconds *big.Rat
		want    int64
	}{
		{nil, 0},
		{big.NewRat(0, 1), 0},
		{big.NewRat(1, 2000), 1},              // 0.5 ms rounds up
		{big.NewRat(4999999, 10000000000), 0}, // 0.4999999 ms rounds down
		{big.NewRat(3, 2), 1500},
	}

	for _, tc := range tests {
		if got := (File{Seconds: tc.seconds}).MS(); got != tc.want {
			t.Errorf("MS of %v s = %d; want %d", tc.seconds, got, tc.want)
		}
	}
}

// endless reaches the size limit without holding the input in memory.
type endless struct{}

func (endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}

	return len(p), nil
}

func TestReadStopsAtSizeLimit(t *testing.T) {
	r := io.MultiReader(strings.NewReader("<testsuite>"), endless{})
	_, err := read(r, FileAttribute, 1<<16)
	if !errors.Is(err, ErrInvalidReport) || !strings.Contains(err.Error(), "over the size limit of 65536 bytes") {
		t.Fatalf("error = %v; want the size limit", err)
	}
}
