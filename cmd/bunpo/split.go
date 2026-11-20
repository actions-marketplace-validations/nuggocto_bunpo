package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/nuggocto/bunpo/internal/manifest"
	"github.com/nuggocto/bunpo/internal/split"
)

func cmdSplit(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := newFlagSet("split", "split --index N --total M [--dir DIR] [--plan] < tests.txt", stderr)
	dir := fs.String("dir", defaultDir, "directory for Bunpō's files")
	index := fs.Int("index", -1, "this machine's index, from 0 to total-1 (required)")
	total := fs.Int("total", 0, "number of machines (required unless --plan)")
	fromPlan := fs.Bool("plan", false, "take this machine's share from plan.json instead of stdin")

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	switch {
	case fs.NArg() > 0:
		return usageError(fs, "unexpected argument %q; the test list comes on stdin", fs.Arg(0))

	case *total == 0 && !*fromPlan:
		return usageError(fs, "--total is required")

	case *total < 0 || *total > split.MaxMachines:
		return usageError(fs, "--total must be from 1 to %d", split.MaxMachines)

	case *index < 0:
		return usageError(fs, "--index is required")

	case *total > 0 && *index >= *total:
		return usageError(fs, "--index must be from 0 to %d", *total-1)
	}

	root, err := openDir(*dir, true)
	if err != nil {
		return err
	}
	defer root.Close()

	var m manifest.Manifest
	if *fromPlan {
		m, err = shareFromPlan(root, *dir, *index, *total, stderr)
	} else {
		m, err = shareFromList(root, *dir, stdin, *index, *total, stderr)
	}
	if err != nil {
		return err
	}

	// A report left by an earlier run on this machine must not pass for this
	// run's.
	if err := removeFile(root, reportName(m.Index)); err != nil {
		return err
	}

	// The list goes out before the manifest, so a list that never arrived
	// leaves no manifest for verify to accept.
	if err := printTests(stdout, m.Tests); err != nil {
		return fmt.Errorf("writing the test list: %w", err)
	}

	data, err := manifest.EncodeManifest(m)
	if err != nil {
		return err
	}

	if err := writeFile(root, manifestName(m.Index), data); err != nil {
		return err
	}

	if len(m.Tests) == 0 {
		fmt.Fprintf(stderr, "bunpo: machine %d has no tests: skip the test command, then run bunpo record with no reports\n", m.Index)
	}

	return nil
}

func printTests(stdout io.Writer, ids []string) error {
	w := bufio.NewWriter(stdout)

	for _, id := range ids {
		if _, err := w.WriteString(id + "\n"); err != nil {
			return err
		}
	}

	return w.Flush()
}

func shareFromList(root *os.Root, dir string, stdin io.Reader, index, total int, stderr io.Writer) (manifest.Manifest, error) {
	p, found, err := splitList(root, dir, stdin, total, stderr)
	if err != nil {
		return manifest.Manifest{}, err
	}

	shard := p.Shards[index]
	predicted := ""
	if found {
		predicted = ", predicted " + formatMS(p.Loads[index])
	}

	fmt.Fprintf(stderr, "bunpo: machine %d of %d gets %d of %d tests%s\n", index, total, len(shard), len(p.Tests), predicted)

	return manifest.Manifest{
		Index:    index,
		Machines: total,
		Digest:   manifest.HexDigest(p.Digest),
		Tests:    shard,
	}, nil
}

func shareFromPlan(root *os.Root, dir string, index, total int, stderr io.Writer) (manifest.Manifest, error) {
	data, ok, err := readFile(root, planName, manifest.MaxFileBytes)
	if err != nil {
		return manifest.Manifest{}, err
	}

	if !ok {
		return manifest.Manifest{}, fmt.Errorf("no %s in %s; run bunpo plan first", planName, dir)
	}

	p, err := manifest.DecodePlan(data)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("%s: %w", planName, err)
	}

	switch {
	case total != 0 && total != p.Machines:
		return manifest.Manifest{}, fmt.Errorf("--total is %d but %s splits for %d machines", total, planName, p.Machines)

	case index >= p.Machines:
		return manifest.Manifest{}, fmt.Errorf("--index is %d but %s has machines 0 to %d", index, planName, p.Machines-1)
	}

	shard := p.Shards[index]
	fmt.Fprintf(stderr, "bunpo: machine %d of %d gets %d tests from %s\n", index, p.Machines, len(shard), planName)

	return manifest.Manifest{Index: index, Machines: p.Machines, Digest: p.Digest, Tests: shard}, nil
}

func cmdPlan(args []string, stdin io.Reader, _, stderr io.Writer) error {
	fs := newFlagSet("plan", "plan --total M [--dir DIR] < tests.txt", stderr)
	dir := fs.String("dir", defaultDir, "directory for Bunpō's files")
	total := fs.Int("total", 0, "number of machines (required)")

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	switch {
	case fs.NArg() > 0:
		return usageError(fs, "unexpected argument %q; the test list comes on stdin", fs.Arg(0))

	case *total < 1 || *total > split.MaxMachines:
		return usageError(fs, "--total is required, from 1 to %d", split.MaxMachines)
	}

	root, err := openDir(*dir, true)
	if err != nil {
		return err
	}
	defer root.Close()

	p, _, err := splitList(root, *dir, stdin, *total, stderr)
	if err != nil {
		return err
	}

	plan := manifest.Plan{Machines: *total, Digest: manifest.HexDigest(p.Digest), Shards: p.Shards}
	data, err := manifest.EncodePlan(plan)
	if err != nil {
		return err
	}

	if err := writeFile(root, planName, data); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "bunpo: planned %d tests across %d machines in %s\n", len(p.Tests), *total, planName)
	return nil
}

// splitList reads the test list and the timing file, splits, and prints a
// summary. It reports whether a timing file existed.
func splitList(root *os.Root, dir string, stdin io.Reader, total int, stderr io.Writer) (split.Plan, bool, error) {
	raw, err := readList(stdin)
	if err != nil {
		return split.Plan{}, false, err
	}

	inv, err := split.NewInventory(raw, pathStyle())
	if err != nil {
		return split.Plan{}, false, err
	}

	timings, found, err := readTimings(root)
	if err != nil {
		return split.Plan{}, false, err
	}

	p, err := split.Split(inv, timings.Durations(), total)
	if err != nil {
		return split.Plan{}, false, err
	}

	printSummary(stderr, p, found, dir)
	return p, found, nil
}

func printSummary(stderr io.Writer, p split.Plan, found bool, dir string) {
	switch {
	case !found:
		fmt.Fprintf(stderr, "bunpo: no %s in %s yet, so every test counts the same\n", timingsName, dir)

	case p.Unknown > 0:
		fmt.Fprintf(stderr, "bunpo: %d of %d tests %s no recorded time and count as the median, %s\n",
			p.Unknown, len(p.Tests), plural(p.Unknown, "has", "have"), formatMS(p.FillMS))
	}

	if found && len(p.Tests) > 0 {
		longest := p.Tests[slices.Index(p.Durations, p.LargestMS())]
		average := formatMS(p.TotalMS() / int64(p.Machines))

		fmt.Fprintf(stderr, "bunpo: predicted machine loads from %s to %s, average %s; longest file %s at %s\n",
			formatMS(slices.Min(p.Loads)), formatMS(slices.Max(p.Loads)), average, longest, formatMS(p.LargestMS()))

		if p.Dominant() {
			fmt.Fprintf(stderr, "bunpo: warning: %s alone takes longer than the average machine load; splitting by file cannot balance this suite\n", longest)
		}
	}

	if n := p.EmptyShards(); n > 0 {
		fmt.Fprintf(stderr, "bunpo: warning: %d of %d machines %s no tests\n", n, p.Machines, plural(n, "gets", "get"))
	}
}
