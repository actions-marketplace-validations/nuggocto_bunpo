// Command bunpo splits a test suite across parallel CI machines using
// recorded timings, and verifies that every test file was assigned once.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

const usage = `usage: bunpo <command> [flags]

Commands:
  split    print this machine's share of the test files
  plan     split once for every machine and write plan.json
  record   turn this machine's test report into a run report
  merge    fold every machine's run report into timings.json
  verify   check that every test file was assigned exactly once
  version  print the version

Run "bunpo <command> -h" for a command's flags.

Exit status: 0 on success, 1 when a check fails, 2 for bad usage or input.
`

type command func(args []string, stdin io.Reader, stdout, stderr io.Writer) error

var commands = map[string]command{
	"split":  cmdSplit,
	"plan":   cmdPlan,
	"record": cmdRecord,
	"merge":  cmdMerge,
	"verify": cmdVerify,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes one command and returns the exit status. Tests call it
// directly instead of starting a process.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	name := args[0]
	switch name {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return 0

	case "version":
		fmt.Fprintln(stdout, "bunpo", version())
		return 0
	}

	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "bunpo: unknown command %q\n\n%s", name, usage)
		return 2
	}

	err := cmd(args[1:], stdin, stdout, stderr)
	return exitCode(name, err, stderr)
}

func exitCode(name string, err error, stderr io.Writer) int {
	var exit *exitError

	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0

	case errors.As(err, &exit):
		if exit.err != nil {
			fmt.Fprintf(stderr, "bunpo %s: %v\n", name, exit.err)
		}

		return exit.code

	default:
		fmt.Fprintf(stderr, "bunpo %s: %v\n", name, err)
		return 2
	}
}

// exitError carries an exit status other than the default 2. A nil err means
// the message was already printed.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit status %d", e.code)
	}

	return e.err.Error()
}

// checkFailed reports a failed check, which exits with status 1.
func checkFailed(format string, a ...any) error {
	return &exitError{code: 1, err: fmt.Errorf(format, a...)}
}

// usageError prints a command-line mistake and the usage, and exits with 2.
func usageError(fs *flag.FlagSet, format string, a ...any) error {
	fmt.Fprintf(fs.Output(), "bunpo %s: %s\n", fs.Name(), fmt.Sprintf(format, a...))
	fs.Usage()
	return &exitError{code: 2}
}

func newFlagSet(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: bunpo %s\n\nFlags:\n", synopsis)
		fs.PrintDefaults()
	}

	return fs
}

// parseFlags parses args. The flag package prints its own error, so a
// mistake only needs exit status 2.
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)

	switch {
	case err == nil:
		return nil

	case errors.Is(err, flag.ErrHelp):
		return err

	default:
		return &exitError{code: 2}
	}
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}

	return info.Main.Version
}
