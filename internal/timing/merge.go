package timing

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
)

// ErrInvalidReport reports a run report that cannot be merged.
var ErrInvalidReport = errors.New("invalid run report")

// Observation is what one run's report says about one test file.
type Observation struct {
	// Timed is true when every case in the file passed. Only then is MS a
	// usable sample: a failed case says little about normal duration, and
	// summing the passed cases alone would record a total that is too short.
	Timed bool
	// MS is the file's total duration in milliseconds. It is ignored unless
	// Timed is true.
	MS int64
}

// Report holds one machine's observations for one run, keyed by test
// identifier.
type Report map[string]Observation

// Merge folds one run's reports into base and returns the new timing file.
// It adds one to the run counter, marks every reported test as seen, appends
// timed samples, and drops tests unseen for more than PruneAfterRuns merges.
// base is not modified.
func Merge(base File, reports []Report) (File, error) {
	if err := base.validate(); err != nil {
		return File{}, err
	}
	// Merging nothing would age every test without seeing any of them.
	if len(reports) == 0 {
		return File{}, fmt.Errorf("%w: merge needs at least one report", ErrInvalidReport)
	}
	if base.Run == math.MaxInt64 {
		return File{}, fmt.Errorf("%w: run counter is at its maximum", ErrInvalidFile)
	}
	if err := checkReports(reports); err != nil {
		return File{}, err
	}

	run := base.Run + 1
	out := File{Run: run, Tests: make(map[string]Entry, len(base.Tests))}
	for id, e := range base.Tests {
		out.Tests[id] = Entry{Samples: slices.Clone(e.Samples), LastSeen: e.LastSeen}
	}
	for _, r := range reports {
		for id, obs := range r {
			e := out.Tests[id]
			e.LastSeen = run
			if obs.Timed {
				e.Samples = append(e.Samples, obs.MS)
				if len(e.Samples) > MaxSamples {
					e.Samples = slices.Clone(e.Samples[len(e.Samples)-MaxSamples:])
				}
			}
			out.Tests[id] = e
		}
	}
	for id, e := range out.Tests {
		if run-e.LastSeen > PruneAfterRuns {
			delete(out.Tests, id)
		}
	}
	return out, nil
}

// checkReports rejects bad durations and any test that appears in two
// reports. A test in two reports ran on two machines, so the split overlapped;
// picking one of its samples would make the result depend on report order.
func checkReports(reports []Report) error {
	owner := make(map[string]int)
	for i, r := range reports {
		// Sorted so the same bad input always reports the same test.
		for _, id := range slices.Sorted(maps.Keys(r)) {
			if j, dup := owner[id]; dup {
				return fmt.Errorf("%w: test %q appears in reports %d and %d", ErrInvalidReport, id, j, i)
			}
			owner[id] = i
			if obs := r[id]; obs.Timed && (obs.MS < 0 || obs.MS > MaxDurationMS) {
				return fmt.Errorf("%w: test %q has duration %d ms outside 0 to %d", ErrInvalidReport, id, obs.MS, MaxDurationMS)
			}
		}
	}
	return nil
}
