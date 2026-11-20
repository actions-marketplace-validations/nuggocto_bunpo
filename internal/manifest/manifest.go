// Package manifest holds the files machines exchange (manifests, the plan,
// run reports) and the verify check.
package manifest

import (
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"github.com/nuggocto/bunpo/internal/jsonfile"
	"github.com/nuggocto/bunpo/internal/split"
	"github.com/nuggocto/bunpo/internal/timing"
)

const (
	FormatVersion = 1
	MaxFileBytes  = 64 << 20
)

var ErrInvalidFile = jsonfile.ErrInvalid

// Manifest records what one machine was asked to run.
type Manifest struct {
	Index    int
	Machines int
	Digest   string   // lowercase hex
	Tests    []string // byte order
}

// Plan is a split for every machine, made once and shared.
type Plan struct {
	Machines int
	Digest   string
	Shards   [][]string
}

// RunReport records what one machine's test report said.
type RunReport struct {
	Index    int
	Machines int
	Tests    timing.Report
}

func HexDigest(d [32]byte) string {
	return hex.EncodeToString(d[:])
}

// Pointer fields tell an absent member from a zero one.
type manifestJSON struct {
	Version  int64    `json:"version"`
	Index    *int     `json:"index"`
	Machines *int     `json:"machines"`
	Digest   *string  `json:"digest"`
	Tests    []string `json:"tests"`
}

type planJSON struct {
	Version  int64      `json:"version"`
	Machines *int       `json:"machines"`
	Digest   *string    `json:"digest"`
	Shards   [][]string `json:"shards"`
}

type observationJSON struct {
	Timed bool  `json:"timed"`
	MS    int64 `json:"ms"`
}

type reportJSON struct {
	Version  int64                      `json:"version"`
	Index    *int                       `json:"index"`
	Machines *int                       `json:"machines"`
	Tests    map[string]observationJSON `json:"tests"`
}

func EncodeManifest(m Manifest) ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}

	return jsonfile.Encode("manifest", manifestJSON{FormatVersion, &m.Index, &m.Machines, &m.Digest, nonNil(m.Tests)}, MaxFileBytes)
}

func DecodeManifest(data []byte) (Manifest, error) {
	var raw manifestJSON
	if err := jsonfile.Decode("manifest", data, MaxFileBytes, FormatVersion, &raw); err != nil {
		return Manifest{}, err
	}

	err := jsonfile.Require("manifest",
		jsonfile.Member{Name: "index", Present: raw.Index != nil},
		jsonfile.Member{Name: "machines", Present: raw.Machines != nil},
		jsonfile.Member{Name: "digest", Present: raw.Digest != nil},
		jsonfile.Member{Name: "tests", Present: raw.Tests != nil})
	if err != nil {
		return Manifest{}, err
	}

	m := Manifest{*raw.Index, *raw.Machines, *raw.Digest, raw.Tests}
	return m, m.validate()
}

func EncodePlan(p Plan) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	shards := make([][]string, len(p.Shards))
	for i, s := range p.Shards {
		shards[i] = nonNil(s)
	}

	return jsonfile.Encode("plan", planJSON{FormatVersion, &p.Machines, &p.Digest, shards}, MaxFileBytes)
}

func DecodePlan(data []byte) (Plan, error) {
	var raw planJSON
	if err := jsonfile.Decode("plan", data, MaxFileBytes, FormatVersion, &raw); err != nil {
		return Plan{}, err
	}

	err := jsonfile.Require("plan",
		jsonfile.Member{Name: "machines", Present: raw.Machines != nil},
		jsonfile.Member{Name: "digest", Present: raw.Digest != nil},
		jsonfile.Member{Name: "shards", Present: raw.Shards != nil})
	if err != nil {
		return Plan{}, err
	}

	p := Plan{*raw.Machines, *raw.Digest, raw.Shards}
	for i := range p.Shards {
		p.Shards[i] = nonNil(p.Shards[i])
	}

	return p, p.validate()
}

func EncodeRunReport(r RunReport) ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	tests := make(map[string]observationJSON, len(r.Tests))
	for id, o := range r.Tests {
		tests[id] = observationJSON{o.Timed, o.MS}
	}

	return jsonfile.Encode("run report", reportJSON{FormatVersion, &r.Index, &r.Machines, tests}, MaxFileBytes)
}

func DecodeRunReport(data []byte) (RunReport, error) {
	var raw reportJSON
	if err := jsonfile.Decode("run report", data, MaxFileBytes, FormatVersion, &raw); err != nil {
		return RunReport{}, err
	}

	err := jsonfile.Require("run report",
		jsonfile.Member{Name: "index", Present: raw.Index != nil},
		jsonfile.Member{Name: "machines", Present: raw.Machines != nil},
		jsonfile.Member{Name: "tests", Present: raw.Tests != nil})
	if err != nil {
		return RunReport{}, err
	}

	r := RunReport{*raw.Index, *raw.Machines, make(timing.Report, len(raw.Tests))}
	for id, o := range raw.Tests {
		r.Tests[id] = timing.Observation{Timed: o.Timed, MS: o.MS}
	}

	return r, r.validate()
}

func (m Manifest) validate() error {
	if err := checkMachine("manifest", m.Index, m.Machines); err != nil {
		return err
	}

	if err := checkDigest("manifest", m.Digest); err != nil {
		return err
	}

	return checkSorted("manifest", m.Tests)
}

func (p Plan) validate() error {
	if err := checkMachine("plan", 0, p.Machines); err != nil {
		return err
	}

	if err := checkDigest("plan", p.Digest); err != nil {
		return err
	}

	if len(p.Shards) != p.Machines {
		return fmt.Errorf("%w: plan has %d shards for %d machines", ErrInvalidFile, len(p.Shards), p.Machines)
	}

	seen := make(map[string]int)
	for i, shard := range p.Shards {
		if err := checkSorted("plan", shard); err != nil {
			return err
		}

		for _, id := range shard {
			if j, dup := seen[id]; dup {
				return fmt.Errorf("%w: plan assigns %q to machines %d and %d", ErrInvalidFile, id, j, i)
			}

			seen[id] = i
		}
	}

	if len(seen) > split.MaxTests {
		return fmt.Errorf("%w: plan has more than %d tests", ErrInvalidFile, split.MaxTests)
	}

	return nil
}

func (r RunReport) validate() error {
	if err := checkMachine("run report", r.Index, r.Machines); err != nil {
		return err
	}

	if len(r.Tests) > split.MaxTests {
		return fmt.Errorf("%w: run report has more than %d tests", ErrInvalidFile, split.MaxTests)
	}

	for _, id := range slices.Sorted(maps.Keys(r.Tests)) {
		if err := checkID("run report", id); err != nil {
			return err
		}

		o := r.Tests[id]
		if o.MS < 0 || o.MS > timing.MaxDurationMS || (!o.Timed && o.MS != 0) {
			return fmt.Errorf("%w: run report gives %q %d ms (timed %t)", ErrInvalidFile, id, o.MS, o.Timed)
		}
	}

	return nil
}

func checkMachine(kind string, index, machines int) error {
	if machines < 1 || machines > split.MaxMachines {
		return fmt.Errorf("%w: %s has %d machines, want 1 to %d", ErrInvalidFile, kind, machines, split.MaxMachines)
	}

	if index < 0 || index >= machines {
		return fmt.Errorf("%w: %s has machine index %d, want 0 to %d", ErrInvalidFile, kind, index, machines-1)
	}

	return nil
}

func checkDigest(kind, d string) error {
	if len(d) != 64 {
		return fmt.Errorf("%w: %s digest %q is not 64 hex digits", ErrInvalidFile, kind, d)
	}

	for _, c := range d {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("%w: %s digest %q is not lowercase hex", ErrInvalidFile, kind, d)
		}
	}

	return nil
}

// checkSorted requires canonical, unique identifiers in byte order, as the
// split writes them.
func checkSorted(kind string, ids []string) error {
	if len(ids) > split.MaxTests {
		return fmt.Errorf("%w: %s has more than %d tests", ErrInvalidFile, kind, split.MaxTests)
	}

	for i, id := range ids {
		if err := checkID(kind, id); err != nil {
			return err
		}

		if i > 0 && ids[i-1] >= id {
			return fmt.Errorf("%w: %s lists %q after %q; tests must be unique and in byte order", ErrInvalidFile, kind, id, ids[i-1])
		}
	}

	return nil
}

// checkID requires canonical form. Canonical identifiers hold no backslash,
// so the POSIX rules work on every platform.
func checkID(kind, id string) error {
	c, err := split.Canonical(id, split.POSIX)
	if err != nil || c != id {
		return fmt.Errorf("%w: %s has non-canonical test identifier %q", ErrInvalidFile, kind, id)
	}

	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}
