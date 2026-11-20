package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nuggocto/bunpo/internal/manifest"
	"github.com/nuggocto/bunpo/internal/split"
)

func cmdVerify(args []string, stdin io.Reader, _, stderr io.Writer) error {
	fs := newFlagSet("verify", "verify [--dir DIR] < tests.txt", stderr)
	dir := fs.String("dir", defaultDir, "directory holding every machine's manifest")

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	if fs.NArg() > 0 {
		return usageError(fs, "unexpected argument %q; the full test list comes on stdin", fs.Arg(0))
	}

	raw, err := readList(stdin)
	if err != nil {
		return err
	}

	inv, err := split.NewInventory(raw, pathStyle())
	if err != nil {
		return err
	}

	root, err := openDir(*dir, false)
	if err != nil {
		return err
	}
	defer root.Close()

	manifests, err := readManifests(root)
	if err != nil {
		return err
	}

	if problems := manifest.Verify(manifests, inv.IDs()); len(problems) > 0 {
		return checkFailed("assignment check failed:\n  - %s", strings.Join(problems, "\n  - "))
	}

	fmt.Fprintf(stderr, "bunpo: assignment verified: %d tests across %d machines, each assigned exactly once, all split from the same input and timings (digest %s…)\n",
		inv.Len(), len(manifests), manifests[0].Digest[:12])
	fmt.Fprintln(stderr, "bunpo: this checks what each machine was asked to run, not what ran; check each runner's result and report separately")
	return nil
}

func readManifests(root *os.Root) ([]manifest.Manifest, error) {
	indexes, err := indexedFiles(root, "manifest-")
	if err != nil {
		return nil, err
	}

	manifests := make([]manifest.Manifest, 0, len(indexes))
	for _, i := range indexes {
		data, _, err := readFile(root, manifestName(i), manifest.MaxFileBytes)
		if err != nil {
			return nil, err
		}

		m, err := manifest.DecodeManifest(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", manifestName(i), err)
		}

		if m.Index != i {
			return nil, fmt.Errorf("%s is for machine %d", manifestName(i), m.Index)
		}

		manifests = append(manifests, m)
	}

	return manifests, nil
}
