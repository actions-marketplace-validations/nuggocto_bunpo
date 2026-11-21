// Command guidecheck runs the shell steps of every guide in docs/guides
// against the real test runners, the way a GitHub Actions "shell: bash" step
// runs them, and checks that each guide selects exactly the assigned files
// and fails when it should. It needs the runners that
// internal/junit/testdata/generate.sh installs.
//
//	internal/junit/testdata/generate.sh
//	go build -o bin/bunpo ./cmd/bunpo
//	go run ./guidecheck -bunpo bin/bunpo
package main

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// runner describes one guide and the sample suite it runs on.
type runner struct {
	name    string
	testDir string // removed to make discovery fail
	failing string // the sample file with a failing test
	// decoys are extra test files. One path contains an assigned path and
	// one name holds glob characters, so a runner that reads arguments as
	// patterns runs a file it was not given.
	decoys map[string]string
}

var runners = []runner{
	{name: "pytest", testDir: "tests", failing: "tests/sub/test_fails.py"},
	{
		name: "jest", testDir: "tests", failing: "tests/sub/fails.test.js",
		decoys: map[string]string{
			"other/tests/fast.test.js": jsTest(""),
			"tests/[ab].test.js":       jsTest(""),
			"tests/a.test.js":          jsTest(""),
		},
	},
	{
		name: "vitest", testDir: "tests", failing: "tests/sub/fails.test.js",
		decoys: map[string]string{
			"other/tests/fast.test.js": jsTest(`import { test } from "vitest";`),
			"tests/[ab].test.js":       jsTest(`import { test } from "vitest";`),
			"tests/a.test.js":          jsTest(`import { test } from "vitest";`),
		},
	},
	{
		name: "rspec", testDir: "spec", failing: "spec/sub/fails_spec.rb",
		decoys: map[string]string{
			"spec/x/spec/fast_spec.rb": rubyTest(),
			"spec/[ab]_spec.rb":        rubyTest(),
			"spec/a_spec.rb":           rubyTest(),
		},
	},
	{
		name: "playwright", testDir: "tests", failing: "tests/sub/fails.spec.js",
		decoys: map[string]string{
			"tests/x/tests/sub/fails.spec.js": jsTest(`const { test } = require("@playwright/test");`),
			"tests/[ab].spec.js":              jsTest(`const { test } = require("@playwright/test");`),
			"tests/a.spec.js":                 jsTest(`const { test } = require("@playwright/test");`),
		},
	},
}

func jsTest(imports string) string {
	return imports + "\n\ntest(\"decoy\", () => {});\n"
}

func rubyTest() string {
	return "RSpec.describe(\"decoy\") { it(\"passes\") {} }\n"
}

// guide holds the parts of a guide's workflow that guidecheck runs.
type guide struct {
	machineScript string
	machineEnv    map[string]string
	listCommand   string
	vitestConfig  string
}

type scenario struct {
	name     string
	machines int
	decoys   bool
	noTests  bool // remove the test directory, so discovery fails
}

var scenarios = []scenario{
	{name: "two machines", machines: 2},
	{name: "decoy files", machines: 2, decoys: true},
	{name: "failed discovery", machines: 2, noTests: true},
	{name: "empty shards", machines: 6},
}

func main() {
	bunpo := flag.String("bunpo", "bin/bunpo", "path to the bunpo binary")
	fixtures := flag.String("fixtures", "/tmp/bunpo-fixtures", "where generate.sh installed the runners")
	flag.Parse()

	work, err := os.MkdirTemp("", "bunpo-guidecheck")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(work)

	path, err := toolPath(work, *bunpo, *fixtures)
	if err != nil {
		fail(err)
	}

	failed := false
	for _, r := range runners {
		g, err := readGuide(filepath.Join("docs", "guides", r.name+".md"))
		if err != nil {
			fail(err)
		}

		for _, s := range scenarios {
			if s.decoys && r.decoys == nil {
				continue
			}

			dir := filepath.Join(work, r.name, strings.ReplaceAll(s.name, " ", "-"))
			problem := check(dir, r, g, s, path, *fixtures)

			result := "ok"
			if problem != nil {
				result, failed = "FAIL: "+problem.Error(), true
			}
			fmt.Printf("%-11s %-17s %s\n", r.name, s.name, result)
		}
	}

	if failed {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "guidecheck:", err)
	os.Exit(1)
}

// check runs one scenario and returns what went wrong, if anything.
func check(dir string, r runner, g guide, s scenario, path, fixtures string) error {
	var reports []string

	for i := range s.machines {
		m := filepath.Join(dir, fmt.Sprintf("machine-%d", i))
		if err := prepareSuite(m, r, g, s, fixtures); err != nil {
			return err
		}

		env := map[string]string{"INDEX": fmt.Sprint(i), "TOTAL": fmt.Sprint(s.machines), "PATH": path}
		for k, v := range g.machineEnv {
			env[k] = v
		}

		code, output := shell(m, g.machineScript, "", env)

		if s.noTests {
			if code == 0 {
				return fmt.Errorf("machine %d passed with no test files to find", i)
			}
			continue
		}

		share, err := readShare(filepath.Join(m, ".bunpo", fmt.Sprintf("manifest-%d.json", i)))
		if err != nil {
			return fmt.Errorf("machine %d: %v\n%s", i, err, output)
		}

		want := 0
		if slices.Contains(share, r.failing) {
			want = 1 // the sample suite fails on purpose
		}

		if code != want {
			return fmt.Errorf("machine %d exited %d, want %d\n%s", i, code, want, output)
		}

		report := filepath.Join(m, ".bunpo", fmt.Sprintf("report-%d.json", i))
		if _, err := os.Stat(report); err != nil {
			return fmt.Errorf("machine %d wrote no run report\n%s", i, output)
		}
		reports = append(reports, report, filepath.Join(m, ".bunpo", fmt.Sprintf("manifest-%d.json", i)))
	}

	final := filepath.Join(dir, "final")
	if err := prepareSuite(final, r, g, s, fixtures); err != nil {
		return err
	}
	for _, f := range reports {
		if err := copyFile(f, filepath.Join(final, ".bunpo", filepath.Base(f))); err != nil {
			return err
		}
	}

	verifyCode, output := shell(final, g.listCommand+" | bunpo verify", "", map[string]string{"PATH": path})
	switch {
	case s.noTests && verifyCode == 0:
		return errors.New("verify passed with no test files to find")

	case !s.noTests && verifyCode != 0:
		return fmt.Errorf("verify exited %d\n%s", verifyCode, output)
	}

	if s.noTests {
		return nil
	}

	if code, output := shell(final, "bunpo merge", "", map[string]string{"PATH": path}); code != 0 {
		return fmt.Errorf("merge exited %d\n%s", code, output)
	}
	return nil
}

// prepareSuite copies the runner's sample suite into dir, with the guide's
// config and the scenario's decoys, and links the installed node modules.
func prepareSuite(dir string, r runner, g guide, s scenario, fixtures string) error {
	src := filepath.Join("internal", "junit", "testdata", r.name, "suite")
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, ".bunpo"), 0o755); err != nil {
		return err
	}

	modules := filepath.Join(fixtures, r.name, "node_modules")
	if _, err := os.Stat(modules); err == nil {
		if err := os.Symlink(modules, filepath.Join(dir, "node_modules")); err != nil {
			return err
		}
	}

	if g.vitestConfig != "" {
		if err := os.WriteFile(filepath.Join(dir, "vitest.config.js"), []byte(g.vitestConfig), 0o644); err != nil {
			return err
		}
	}

	if s.decoys {
		for name, body := range r.decoys {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return err
			}
		}
	}

	if s.noTests {
		return os.RemoveAll(filepath.Join(dir, r.testDir))
	}
	return nil
}

var (
	yamlBlock    = regexp.MustCompile("(?s)```yaml\n(.*?)```")
	tsBlock      = regexp.MustCompile("(?s)```ts\n(.*?)```")
	machineStep  = regexp.MustCompile(`(?s)- name: Run this machine's tests\n        env:\n(.*?)        run: \|\n(.*?)\n      - `)
	verifyStep   = regexp.MustCompile(`- name: Verify the assignment\n        run: (.*) \| bunpo verify\n`)
	bashDefault  = regexp.MustCompile(`\ndefaults:\n  run:\n    shell: bash\n`)
	envAssignRow = regexp.MustCompile(`^\s+(\w+): "?([^"$]*)"?$`)
)

func readGuide(path string) (guide, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return guide{}, err
	}

	yaml := yamlBlock.FindSubmatch(data)
	if yaml == nil {
		return guide{}, fmt.Errorf("%s has no workflow", path)
	}

	// guidecheck runs every step with pipefail, so the guide must ask GitHub
	// for the same; without it, a failed discovery command passes.
	if !bashDefault.Match(yaml[1]) {
		return guide{}, fmt.Errorf("%s: the workflow does not set defaults.run.shell to bash", path)
	}

	step := machineStep.FindSubmatch(yaml[1])
	verify := verifyStep.FindSubmatch(yaml[1])
	if step == nil || verify == nil {
		return guide{}, fmt.Errorf("%s: cannot find the machine or verify step", path)
	}

	g := guide{machineEnv: map[string]string{}, listCommand: string(verify[1])}

	// GitHub expressions are left out; guidecheck sets INDEX and TOTAL itself.
	for line := range strings.Lines(string(step[1])) {
		if m := envAssignRow.FindStringSubmatch(strings.TrimRight(line, "\n")); m != nil && !strings.Contains(line, "${{") {
			g.machineEnv[m[1]] = m[2]
		}
	}

	var script strings.Builder
	for line := range strings.Lines(string(step[2])) {
		script.WriteString(strings.TrimPrefix(line, "          "))
	}
	g.machineScript = script.String()

	if ts := tsBlock.FindSubmatch(data); ts != nil {
		g.vitestConfig = strings.Replace(string(ts[1]), "    // ...your existing options\n", "", 1)
	}
	return g, nil
}

// toolPath builds a PATH with bunpo, pytest, and a "bundle" that runs the
// RSpec installed by generate.sh, ahead of the system PATH.
func toolPath(work, bunpo, fixtures string) (string, error) {
	bin := filepath.Join(work, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}

	if err := copyFile(bunpo, filepath.Join(bin, "bunpo")); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Join(bin, "bunpo"), 0o755); err != nil {
		return "", err
	}

	gems := filepath.Join(fixtures, ".gems")
	bundle := fmt.Sprintf("#!/usr/bin/env bash\n# Stands in for \"bundle exec\" with the gems generate.sh installed.\nshift\nGEM_HOME=%[1]s GEM_PATH=\"%[1]s:$(gem env gempath)\" exec %[1]s/bin/\"$@\"\n", gems)
	if err := os.WriteFile(filepath.Join(bin, "bundle"), []byte(bundle), 0o755); err != nil {
		return "", err
	}

	return strings.Join([]string{bin, filepath.Join(fixtures, ".venv", "bin"), os.Getenv("PATH")}, string(os.PathListSeparator)), nil
}

// shell runs script the way a GitHub Actions "shell: bash" step does.
func shell(dir, script, stdin string, env map[string]string) (int, string) {
	cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)

	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out

	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, out.String()

	case errors.As(err, &exit):
		return exit.ExitCode(), out.String()

	default:
		return -1, err.Error()
	}
}

func readShare(manifest string) ([]string, error) {
	data, err := os.ReadFile(manifest)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("no manifest")
	}
	if err != nil {
		return nil, err
	}

	var m struct {
		Tests []string `json:"tests"`
	}
	err = json.Unmarshal(data, &m)
	return m.Tests, err
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
