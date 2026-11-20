# 1. How `record` finds each test's file

Date: 2026-11-20. Status: accepted.

## Context

`record` turns a machine's JUnit report into timings per test file, so it has
to know which file each test case came from. Reports from the five runners in
the first build keep that path in different places:

| Runner | Where the path is |
| --- | --- |
| pytest | `file` attribute on each case, with `junit_family=xunit1` only |
| Jest | `file` attribute, with `jest-junit`'s `addFileAttribute` only |
| Vitest | `file` attribute, with the junit reporter's `addFileAttribute` only |
| RSpec | `file` attribute, by default, written as `./spec/...` |
| Playwright | only in `classname` and the suite name, relative to `testDir` |

PROJECT.md says Bunpō never guesses a file from a class name. A runner can
also run more files than it was given: Vitest treats file arguments as
substring patterns.

## Decision

- `record` reads the `file` attribute by default, falling back to the nearest
  enclosing suite's.
- `--file-from classname` and `--file-from suite` read those attributes
  instead, and `--path-prefix` goes in front of every path. The user chooses
  them once in their workflow, following the runner's guide. Bunpō never picks
  one by looking at the report, so a class name is only read as a path when
  the user said it is one.
- Every path goes through the same canonical rules as the test list, so a
  report's spelling (`./spec/a_spec.rb`) matches the list's (`spec/a_spec.rb`).
- `record` compares the report with the machine's manifest. A file that was
  not assigned to the machine fails the step with status 1, because it may
  have run on two machines. If nothing in the report matches, the cause is more
  likely a wrong flag, and the message says so with status 2. An assigned file
  missing from the report is only a warning.

## Consequences

- Playwright works without changing its configuration, at the cost of two
  flags in its guide.
- The pattern-matching problem is caught on the machine where it happens,
  with a message naming the files, instead of later as a duplicate in `merge`.
- The Vitest and Playwright guides still need a way to pass files that cannot
  match other paths. That belongs to the guides, not to `record`.

## Alternatives considered

- **Detect the runner from the report.** Reports carry no reliable runner
  marker, and a wrong guess would record timings under the wrong paths with
  no error.
- **Read a class name as a path whenever there is no `file` attribute.** That
  is exactly the guessing PROJECT.md rules out: for pytest's default report it
  would turn `tests.sub.test_a` into a path that does not exist.
- **Accept extra files with a warning.** `merge` would then fail on the
  duplicate in a later job, far from the cause.
