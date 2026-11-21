package timing

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
)

var ErrInvalidReport = errors.New("invalid run report")

// Observation is what one run says about one test file. MS counts only when
// Timed, which means every case in the file passed.
type Observation struct {
	Timed bool
	MS    int64
}

// Report holds one machine's observations for one run.
type Report map[string]Observation

// Merge folds one run's reports into base: it adds one to Run, marks
// reported tests as seen, appends timed samples and drops tests unseen for
// more than PruneAfterRuns merges. base is not modified.
func Merge(base File, reports []Report) (File, error) {
	if err := base.validate(); err != nil {
		return File{}, err
	}

	// Merging nothing would age every test without seeing any.
	if len(reports) == 0 {
		return File{}, fmt.Errorf("%w: merge needs at least one report", ErrInvalidReport)
	}

	if base.Run == math.MaxInt64 {
		return File{}, fmt.Errorf("%w: %s: run counter is at its maximum", ErrInvalidFile, kind)
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

// checkReports rejects bad durations and tests in two reports. Such a test
// ran on two machines, and keeping either time would depend on report order.
func checkReports(reports []Report) error {
	owner := make(map[string]int)

	for i, r := range reports {
		// Sorted, so bad input always reports the same test.
		for _, id := range slices.Sorted(maps.Keys(r)) {
			if !validID(id) {
				return fmt.Errorf("%w: non-canonical test identifier %q", ErrInvalidReport, id)
			}

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
