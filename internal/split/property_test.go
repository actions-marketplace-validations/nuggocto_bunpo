package split_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/nuggocto/bunpo/internal/split"
)

// randomCase builds a small suite with repeated names, missing timings, zeros
// and ties: the inputs most likely to expose an order dependence.
func randomCase(r *rand.Rand) (raw []string, durations map[string]int64, machines int) {
	durations = make(map[string]int64)

	for range r.IntN(60) {
		id := fmt.Sprintf("dir%d/test_%d.py", r.IntN(4), r.IntN(40))
		raw = append(raw, id)

		switch r.IntN(4) {
		case 0:
			// no recorded time

		case 1:
			durations[id] = int64(r.IntN(3))

		default:
			durations[id] = int64(r.IntN(100_000))
		}
	}

	return raw, durations, 1 + r.IntN(12)
}

func splitSeed(t *testing.T, seed uint64, raw []string, durations map[string]int64, machines int) split.Plan {
	t.Helper()

	inv, err := split.NewInventory(raw, split.POSIX)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}

	p, err := split.Split(inv, durations, machines)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}

	return p
}

func TestSplitProperties(t *testing.T) {
	for seed := range uint64(2000) {
		r := rand.New(rand.NewPCG(seed, 0))
		raw, durations, machines := randomCase(r)
		p := splitSeed(t, seed, raw, durations, machines)

		// The generator emits canonical names, so the expected set comes from
		// raw and not from the plan.
		want := make(map[string]bool, len(raw))
		for _, id := range raw {
			want[id] = true
		}

		duration := make(map[string]int64, len(p.Tests))
		for i, id := range p.Tests {
			duration[id] = p.Durations[i]

			if recorded, ok := durations[id]; ok && p.Durations[i] != recorded {
				t.Fatalf("seed %d: %q predicted at %d ms; its recorded duration is %d", seed, id, p.Durations[i], recorded)
			}
		}

		assigned := make(map[string]int)
		for m, shard := range p.Shards {
			if !slices.IsSorted(shard) {
				t.Fatalf("seed %d: shard %d is not sorted: %q", seed, m, shard)
			}

			var load int64
			for _, id := range shard {
				assigned[id]++
				load += duration[id]
			}

			if load != p.Loads[m] {
				t.Fatalf("seed %d: shard %d sums to %d but Loads says %d", seed, m, load, p.Loads[m])
			}
		}

		for id := range want {
			if assigned[id] != 1 {
				t.Fatalf("seed %d: %q assigned %d times; want exactly once", seed, id, assigned[id])
			}
		}

		if len(assigned) != len(want) {
			t.Fatalf("seed %d: shards hold %d distinct tests; the input has %d", seed, len(assigned), len(want))
		}

		if wantEmpty := max(0, machines-len(want)); p.EmptyShards() != wantEmpty {
			t.Fatalf("seed %d: %d empty shards for %d tests on %d machines; want %d",
				seed, p.EmptyShards(), len(want), machines, wantEmpty)
		}

		shuffled := slices.Clone(raw)
		r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		if again := splitSeed(t, seed, shuffled, durations, machines); !reflect.DeepEqual(p, again) {
			t.Fatalf("seed %d: shuffling the input changed the plan", seed)
		}
	}
}

// TestSplitWithinFourThirdsOfOptimum checks longest-first dealing against an
// exhaustive search for the best split.
func TestSplitWithinFourThirdsOfOptimum(t *testing.T) {
	const (
		maxJobs     = 7
		maxMachines = 4
		maxDuration = 50 // keeps 4 * maxJobs * maxDuration far from overflow
	)

	for seed := range uint64(500) {
		r := rand.New(rand.NewPCG(seed, 1))
		machines := 1 + r.IntN(maxMachines)

		ids := make([]string, 1+r.IntN(maxJobs))
		durations := make(map[string]int64, len(ids))
		for i := range ids {
			ids[i] = fmt.Sprintf("t%d.py", i)
			durations[ids[i]] = int64(r.IntN(maxDuration + 1))
		}

		p := splitSeed(t, seed, ids, durations, machines)
		lpt := slices.Max(p.Loads)
		best := optimalMakespan(p.Durations, machines)

		if best > lpt {
			t.Fatalf("seed %d: exhaustive search found %d, worse than LPT's %d", seed, best, lpt)
		}

		if 3*lpt > 4*best {
			t.Fatalf("seed %d: LPT makespan %d exceeds 4/3 of the optimum %d (durations %v, %d machines)",
				seed, lpt, best, p.Durations, machines)
		}
	}
}

func TestOptimalMakespanRegression(t *testing.T) {
	// The optimum is 6, not the lower bound 4.
	if got := optimalMakespan([]int64{3, 3, 3, 3}, 3); got != 6 {
		t.Fatalf("optimalMakespan = %d; want 6", got)
	}
}

// optimalMakespan tries every assignment. It shares no code with the split,
// so it can judge it.
func optimalMakespan(jobs []int64, machines int) int64 {
	loads := make([]int64, machines)
	best := int64(-1)

	var assign func(i int)
	assign = func(i int) {
		if i == len(jobs) {
			if m := slices.Max(loads); best < 0 || m < best {
				best = m
			}

			return
		}

		for m := range loads {
			loads[m] += jobs[i]
			assign(i + 1)
			loads[m] -= jobs[i]
		}
	}

	assign(0)
	return best
}
