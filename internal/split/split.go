// Package split deals test files out to machines by predicted duration and
// computes the digest that lets machines prove they made the same split. It
// reads no files and no environment, so every platform gives the same answer.
package split

import (
	"cmp"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/nuggocto/bunpo/internal/timing"
)

const (
	// MaxMachines bounds the machine count.
	MaxMachines = 256

	// AlgorithmVersion is part of every digest. Raise it whenever a change
	// could alter any split, so machines running different versions show
	// different digests instead of silently disagreeing.
	AlgorithmVersion = 1
)

var (
	// ErrMachines reports a machine count outside 1 to MaxMachines.
	ErrMachines = errors.New("invalid machine count")
	// ErrInvalidDuration reports a duration outside 0 to timing.MaxDurationMS.
	ErrInvalidDuration = errors.New("invalid duration")
)

// Plan is the split of one inventory across machines.
type Plan struct {
	Machines int
	// Tests is the canonical input, in byte order. Durations holds the
	// effective duration of the test at the same index: its recorded
	// duration, or FillMS when it has none.
	Tests     []string
	Durations []int64
	// Shards holds each machine's tests in byte order, and Loads each
	// machine's predicted total in milliseconds.
	Shards [][]string
	Loads  []int64
	// Unknown counts tests with no recorded duration. Each was given FillMS:
	// the median of the known durations, or 0 with no history at all.
	Unknown int
	FillMS  int64
	// Digest identifies everything the split depends on. Equal digests mean
	// equal splits.
	Digest [sha256.Size]byte
}

// Split assigns every test in inv to exactly one machine. durations
// maps identifiers to recorded milliseconds; entries for tests outside inv
// are ignored. The same inventory, durations and machine count always give
// the same plan.
func Split(inv Inventory, durations map[string]int64, machines int) (Plan, error) {
	if machines < 1 || machines > MaxMachines {
		return Plan{}, fmt.Errorf("%w: %d, want 1 to %d", ErrMachines, machines, MaxMachines)
	}

	n := len(inv.ids)
	effective := make([]int64, n)
	recorded := make([]bool, n)
	known := make([]int64, 0, n)
	for i, id := range inv.ids {
		ms, ok := durations[id]
		if !ok {
			continue
		}
		if ms < 0 || ms > timing.MaxDurationMS {
			return Plan{}, fmt.Errorf("%w: test %q has %d ms, want 0 to %d", ErrInvalidDuration, id, ms, timing.MaxDurationMS)
		}
		effective[i], recorded[i] = ms, true
		known = append(known, ms)
	}
	// With no history at all every test counts as 0 ms, and the fewer-tests
	// tie rule below turns that into an even deal.
	fill, _ := timing.Median(known)
	unknown := 0
	for i := range effective {
		if !recorded[i] {
			effective[i] = fill
			unknown++
		}
	}

	// Longest first. inv.ids is in byte order and the sort is stable, so
	// equal durations keep path order: the lower path goes first.
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Compare(effective[b], effective[a])
	})

	loads := make([]int64, machines)
	counts := make([]int, machines)
	owner := make([]int, n)
	for _, t := range order {
		target := 0
		for m := 1; m < machines; m++ {
			// Least load wins; on equal load, fewer tests; then the lower
			// index, because both comparisons are strict. Without the count,
			// files recorded at 0 ms would all land on machine 0, whose load
			// never rises.
			if loads[m] < loads[target] ||
				(loads[m] == loads[target] && counts[m] < counts[target]) {
				target = m
			}
		}
		loads[target] += effective[t]
		counts[target]++
		owner[t] = target
	}

	shards := make([][]string, machines)
	for m := range shards {
		shards[m] = []string{}
	}
	// Walking the inventory in byte order leaves every shard sorted.
	for i, id := range inv.ids {
		shards[owner[i]] = append(shards[owner[i]], id)
	}

	return Plan{
		Machines:  machines,
		Tests:     slices.Clone(inv.ids),
		Durations: effective,
		Shards:    shards,
		Loads:     loads,
		Unknown:   unknown,
		FillMS:    fill,
		Digest:    digest(machines, inv.ids, effective),
	}, nil
}

// digest hashes a version line, a machine-count line, then one record per
// test in byte order: the path, a NUL byte, the effective duration in decimal
// milliseconds, and a newline. Identifiers cannot contain NUL or newline, so
// no two different inputs produce the same bytes.
func digest(machines int, ids []string, effective []int64) [sha256.Size]byte {
	h := sha256.New()
	// Writes to a hash never fail.
	_, _ = fmt.Fprintf(h, "bunpo split %d\nmachines %d\n", AlgorithmVersion, machines)
	buf := make([]byte, 0, 32)
	for i, id := range ids {
		_, _ = io.WriteString(h, id)
		buf = append(buf[:0], 0)
		buf = strconv.AppendInt(buf, effective[i], 10)
		buf = append(buf, '\n')
		_, _ = h.Write(buf)
	}
	return [sha256.Size]byte(h.Sum(nil))
}

// TotalMS returns the predicted total across all machines.
func (p Plan) TotalMS() int64 {
	var total int64
	for _, l := range p.Loads {
		total += l
	}
	return total
}

// LargestMS returns the largest effective duration of a single test, or 0
// for an empty plan.
func (p Plan) LargestMS() int64 {
	if len(p.Durations) == 0 {
		return 0
	}
	return slices.Max(p.Durations)
}

// Dominant reports whether one test is predicted to take longer than the
// average machine load. File-level splitting cannot divide that file, so no
// split balances.
func (p Plan) Dominant() bool {
	// Compare largest > total/machines without the rounding of a division.
	return p.LargestMS()*int64(p.Machines) > p.TotalMS()
}

// EmptyShards returns how many machines received no tests.
func (p Plan) EmptyShards() int {
	empty := 0
	for _, s := range p.Shards {
		if len(s) == 0 {
			empty++
		}
	}
	return empty
}
