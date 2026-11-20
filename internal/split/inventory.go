package split

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxTests bounds the number of distinct tests in one split.
const MaxTests = 200_000

var (
	// ErrInvalidID reports an identifier that cannot name a repository file.
	ErrInvalidID = errors.New("invalid test identifier")
	// ErrCollision reports two different spellings that name the same file.
	ErrCollision = errors.New("test identifiers collide")
	// ErrTooManyTests reports an inventory above MaxTests.
	ErrTooManyTests = errors.New("too many tests")
	// ErrPathStyle reports a PathStyle other than POSIX or Windows.
	ErrPathStyle = errors.New("unknown path style")
)

// PathStyle says how to read separators in raw identifiers.
type PathStyle int

const (
	// POSIX identifiers use / only. A backslash is an ordinary character in a
	// POSIX file name, so it is rejected rather than read as a separator.
	POSIX PathStyle = iota + 1
	// Windows identifiers may use \ or /; both become /.
	Windows
)

// Inventory is a canonical list of test identifiers: repository-relative,
// / separated, free of repeats and sorted by bytes. The only way to build one
// is NewInventory, so a split never sees unchecked input.
type Inventory struct {
	ids []string
}

// NewInventory canonicalizes raw identifiers. It fails if two different
// spellings name the same file, naming both, before anything is assigned or
// hashed. Exact repeats of one spelling are removed.
func NewInventory(raw []string, style PathStyle) (Inventory, error) {
	if style != POSIX && style != Windows {
		return Inventory{}, fmt.Errorf("%w %d", ErrPathStyle, style)
	}
	// Repeated lines can make raw far longer than the distinct tests it holds,
	// so never reserve more than the limit allows.
	size := min(len(raw), MaxTests)
	spelling := make(map[string]string, size) // identifier → first spelling
	ids := make([]string, 0, size)
	for _, original := range raw {
		id, err := canonicalize(original, style)
		if err != nil {
			return Inventory{}, err
		}
		if first, ok := spelling[id]; ok {
			if first == original {
				continue
			}
			return Inventory{}, fmt.Errorf("%w: %q and %q both name %q", ErrCollision, first, original, id)
		}
		if len(ids) == MaxTests {
			return Inventory{}, fmt.Errorf("%w: more than %d distinct tests", ErrTooManyTests, MaxTests)
		}
		spelling[id] = original
		ids = append(ids, id)
	}
	// Go compares strings bytewise, so this order is the same on every
	// platform and in every locale.
	slices.Sort(ids)
	return Inventory{ids: ids}, nil
}

// IDs returns a copy of the canonical identifiers in byte order.
func (inv Inventory) IDs() []string {
	return slices.Clone(inv.ids)
}

// Len returns the number of identifiers.
func (inv Inventory) Len() int {
	return len(inv.ids)
}

func canonicalize(original string, style PathStyle) (string, error) {
	invalid := func(reason string) error {
		return fmt.Errorf("%w %q: %s", ErrInvalidID, original, reason)
	}
	if original == "" {
		return "", invalid("empty")
	}
	// The timing file is JSON, which cannot hold invalid UTF-8, so such a
	// test could never get a recorded duration.
	if !utf8.ValidString(original) {
		return "", invalid("not valid UTF-8")
	}
	// The digest separates fields with NUL and records with newlines, and the
	// input is one identifier per line, so none of these can be part of a name.
	if strings.ContainsAny(original, "\x00\r\n") {
		return "", invalid("contains a NUL, carriage return or newline")
	}

	p := original
	switch style {
	case POSIX:
		if strings.Contains(p, `\`) {
			return "", invalid(`contains a backslash; POSIX input must separate with "/"`)
		}
	case Windows:
		p = strings.ReplaceAll(p, `\`, "/")
	}
	if strings.HasPrefix(p, "/") {
		return "", invalid("is absolute; use a repository-relative path")
	}

	parts := strings.Split(p, "/")
	kept := parts[:0]
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			// Resolving .. lexically can name a different file than the
			// filesystem would through a symlink, so refuse it.
			return "", invalid(`contains a ".." component`)
		}
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return "", invalid("names no file")
	}
	// Check the cleaned path, not the raw one: "./C:/a.py" only shows its
	// drive once the leading "." is gone. "C:a.py" is drive-relative, which
	// is no more repository-relative than "C:/a.py".
	if style == Windows && hasDrive(kept[0]) {
		return "", invalid("is an absolute Windows path; use a repository-relative path")
	}
	return strings.Join(kept, "/"), nil
}

func hasDrive(component string) bool {
	if len(component) < 2 || component[1] != ':' {
		return false
	}
	c := component[0]
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
