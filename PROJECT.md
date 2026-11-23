---
tags: [go, project, product, bunpo, ci, testing, test-splitting, cli, developer-tools]
created: 2026-11-11
updated: 2026-11-23
status: in development
license: Apache-2.0
domain: bunpo.org
domain_status: live
---

# Bunpō (分蜂) — The bees dividing their hive

**One line:** a small open-source program that splits a test suite across
parallel CI machines to balance their predicted load, and verifies that every
input test file was assigned exactly once.

A slow test suite is usually run on several machines at once. Someone has to
decide which tests go to which machine. Split them by file name and one machine
gets the slow tests while the others sit idle. Bunpō remembers how long each
test took and deals them out evenly.

**Type:** product. It is small and needs no server.

**First build:** Phases 1 to 3 (`0.0.1` to `0.0.6`). **Later ideas:** the hosted
queue, the dashboard, flaky-test reports and everything under "Later ideas".
They stay in this plan on purpose and do not block the first build.

**Release:** `0.2.0`, when every phase of the roadmap is done. The static CLI
and website shipped across `0.0.1` to `0.0.6`; the hosted queue pilot starts at
`0.1.0`.

The command-line tool, real-repository trials, and `bunpo.org` website (`0.0.6`)
are implemented; demand and the hosted queue are not yet validated. This plan
replaces the earlier donation-journal plan that used the same name.

## Why

- **Open splitters exist, but leave gaps.** Knapsack Pro sells timing-based
  splitting as a hosted service; its original open-source gem is archived.
  Several open tools exist: single-language ones (`pytest-split` for Python,
  `testris` for Go), CircleCI-oriented ones (`ci_split_tests`), and two active
  general-purpose JUnit splitters — `leonid-shevtsov/split_tests` (Go, no
  license file, sliding-window float averages, RSpec-focused docs) and
  `mtsmfm/split-test` (Rust, MIT, reads raw JUnit XML from the last run with
  `f64` times and `file`/`filepath` attributes only, no multi-run history).
  Neither has assignment verification, runner-output checks against a manifest,
  failure-time exclusion, integer-only determinism, or tested workflows for
  pytest, Jest, Vitest, RSpec and Playwright.
- **CI vendors keep it to themselves.** CircleCI has timing-based splitting
  built in, for CircleCI only. Depot added it in beta in July 2026. A team on
  GitHub Actions or a self-hosted runner has nothing built in.
- **Nobody checks the split.** If two machines see a different file list or a
  different timing file, assignments can overlap or omit files, and the build
  still goes green. Neither `split_tests` nor `split-test` detects this
  assignment mismatch, and neither checks that a runner ran only its assigned
  files. Bunpō's `verify` and `record` steps do; `verify` checks assignments
  and does not observe execution.
- **It is the right size for one person.** One command-line program, no
  server, no database.
- **The risk is plain.** Splitting is a feature, and CI vendors may keep
  adding it. The free tool has to be good enough that people choose it anyway.

## First useful journey

1. Inès maintains a Django application. Its 1,400 test files take 38 minutes
   on one machine. She already runs them on 8 machines, split by file name,
   and the slowest machine takes 14 minutes while the fastest takes 2.
2. She adds one step to her workflow that downloads Bunpō.
3. On each machine, `bunpo split --index 3 --total 8` reads the list of test
   files and prints the ones this machine should run. Her test command takes
   that list.
4. After the tests, `bunpo record` reads the test report on each machine and
   writes how long each file took.
5. A last job runs `bunpo merge` to combine the eight timing files into one,
   which the CI cache keeps for the next run.
6. The same job runs `bunpo verify`. It confirms that all eight machines saw
   the same input, and that the eight lists together contain every test file
   exactly once. A mismatch fails the build. Runner failures and missing
   reports fail separate workflow steps.
7. On the second run the timings exist. She compares predicted shard loads
   with actual runner times. About 5 minutes per machine is the target for
   this example, subject to file sizes, setup costs and runner variability.

Success is this journey working on a fresh repository from the instructions,
with `verify` catching a deliberately mismatched timing cache and the workflow
rejecting a missing runner report. On a declared benchmark with a demonstrated
feasible balance, require predicted `slowest / fastest <= 1.10` for nonempty
shards. Measure actual runner times against the same target and explain departures.
Publish predicted loads and measured times separately, including setup time,
unknown timings and variation across repeated runs. This ratio is not a
guarantee for arbitrary suites.

## Scope

| Stage | Scope |
| --- | --- |
| First build, `0.0.3`–`0.0.6` | `split`, `plan`, `record`, `merge` and `verify`. File-level assignment verification and splitting from JUnit XML timings. Binaries for Linux, macOS and Windows. A GitHub Action wrapper. Guides for pytest, Jest, Vitest, RSpec and Playwright. |
| Hosted service | A live queue that hands out tests one at a time, plus a dashboard. Paid. |
| Later ideas | Go package splitting, splitting inside a file, flaky-test detection, other CI systems' environment variables. Listed below. |

Bunpō decides which tests run where. It does not run tests, retry them, start
machines or replace a CI system in the first build. The later queue coordinates
retryable assignments; execution still belongs to the runner.

## Core rules

### The split

- The input is a list of test identifiers, one per line. In the first build an
  identifier is a file path.
- Use repository-relative identifiers with `/` separators. Convert native Windows separators at input; reject literal backslashes in POSIX filenames rather than reinterpreting them.
- Drop `.` and empty components. Reject an identifier that is empty, absolute,
  starts with a Windows drive, contains a `..` component, contains a NUL, a
  carriage return or a newline, or is not valid UTF-8. Check for a drive after
  dropping components, because `./C:/a.py` only shows its drive once the `.`
  is gone. The timing file is JSON, which cannot hold invalid UTF-8.
- Keep the original spelling until collision checks finish. If distinct input paths normalize to one identifier, fail and name both before emitting any assignment or digest. Then remove exact repeats and sort by bytes.
- Durations are whole milliseconds. No floating-point arithmetic takes part in
  the split, so every platform computes the same answer. One duration is at
  most 30 days, so a total of 200,000 of them fits in a 64-bit integer.
- Deal the tests out longest first. Each test goes to the machine with the
  least total time so far. On a tie between tests, the lower path goes first.
  On a tie between machines, the one with fewer tests wins, then the lower
  index. Without the test count, files recorded at 0 ms would all land on
  machine 0, because its total never rises.
- The split uses recorded times as they are, zero included.
- A test with no recorded time gets the median of the known times. With no
  history at all, every test counts as 0 ms, and the fewer-tests rule makes
  the split an even deal.
- The median of an even number of values is the average of the two middle
  values, rounded half up, in integer arithmetic. The same rule applies to a
  test's own samples and to the median of known times. It matches the median a
  spreadsheet or Python's `statistics.median` gives, so a user can check any
  prediction by hand. Taking the lower middle value would pick the faster of
  two samples and under-predict during the first runs.
- The same list, the same timings and the same machine count always give the
  same split. This is the property everything else depends on.
- Report the largest predicted file time and the average load per machine.
  Warn when one file exceeds that average or some shards must be empty.
  A 100-minute file and nine one-minute files cannot balance on two machines;
  file-level scheduling cannot divide the dominant file.

A worked example to check by hand: six files taking 9, 7, 6, 5, 4 and 3
minutes, on two machines. Splitting by name gives 22 and 12. Bunpō gives 9, 5
and 3 to one machine and 7, 6 and 4 to the other: 17 and 17.

This method does not always find the perfect split. Its worst case is known:
for identical machines and fixed nonnegative durations, the largest assigned
total is at most four thirds of the optimal makespan. This concerns the input
estimates, not actual CI wall time. Measure how close it gets on real suites.

### Verification of assignments

- Each `split` writes a small manifest: the machine index, the machine count,
  the list it was given, and a SHA-256 digest of everything the split depends
  on.
- The digest covers the split algorithm version, the machine count, the
  sorted input list, and the effective duration of each test: its median, or
  the fill-in value for a test with no history. Equal digests mean equal
  splits. Two timing files that differ but give the same effective durations
  produce the same digest, which is correct because the split is the same.
- The algorithm version is an integer, raised whenever a change could alter
  any split. A machine that downloaded a different Bunpō with a changed
  algorithm then shows a different digest.
- The digest input is a version line, a machine-count line, then one record
  per test in sorted order: the path, a NUL byte, the duration in decimal
  milliseconds, and a newline. Paths cannot contain NUL and the line-based
  input cannot carry a newline, so the encoding is unambiguous.
- `verify` reads every manifest. It fails unless all digests match, every
  index from 0 to the count minus one is present once, no test appears in two
  assigned lists, and their union equals the independently supplied canonical
  input list. Keep the full input distinct from each machine's assigned list.
- When digests differ, `verify` groups machines by digest and names the
  machines in each group, so the message points at the machine that differed.
- A success says "assignment verified". A manifest records what a runner was
  asked to do; it cannot prove that the runner started, completed or executed
  each case once. A skipped runner can still have a valid assignment manifest.
- The workflow separately checks runner exit statuses and expected report
  artifacts, including an explicit outcome for an empty shard. Reported file
  coverage is separate evidence from assignment coverage. Detecting missing
  individual cases requires an independent case inventory; JUnit timings alone
  do not provide it. Timing samples from passed cases are not execution proof.
- A team that prefers prevention to detection can run `bunpo plan` once in an
  early job and pass the plan file to every machine. Each machine then only
  reads its own list from the plan.

### Timings

- `record` reads JUnit XML and adds up the time of each test case per file.
  A file's total counts for a run only when every case in that file passed.
  A failed test's time says little about how long it normally takes, and
  adding up only the passed cases of a file with a failure records a total
  that is too short.
- A skipped case does not fail its file. Otherwise a file with one
  platform-specific skip would never get a time.
- JUnit writes times as decimal seconds. Convert them to milliseconds from the
  decimal text, rounding half up, and never through a float.
- Add up a file's case times exactly and round once per file. Rounding each
  case first turns a thousand 0.4 ms cases into 0 ms.
- The timing file keeps the last five times per test and uses their median.
  One slow run does not reshuffle everything.
- `merge` combines timing files from all machines and adds one to a run
  counter stored in the timing file. Each test stores the counter value of
  the last run that saw it.
- A test counts as seen when it appears in a report, whether it passed,
  failed or was skipped. A file with a failure records no time for that run
  but stays seen, so a file that keeps failing while a team fixes it keeps its
  history.
- A test in two reports from one run ran on two machines, so the split
  overlapped. `merge` fails rather than keep one of the two times, which would
  make the result depend on report order. A merge with no reports also fails,
  because it would age every test without seeing any.
- `merge` drops a test once the counter is more than 30 runs past its
  last-seen value, so the file cannot grow without limit. Counting merges
  instead of days needs no clock, and an idle repository keeps its timings.
- The timing file is versioned JSON. An unknown version is an error, not a
  guess.
- Timings should be saved from the main branch only. A pull request reads them
  and does not overwrite them.

### Reading reports

- A JUnit report must say which file each test came from. Some runners write
  that only when asked. Bunpō reports a clear error with the setting to change
  when the file is missing; it never guesses a file from a class name.
- Every test case must name its file. One case without a file fails `record`:
  leaving it out would also leave out its failure, and the file would record
  an incomplete time.
- A report must be one JUnit document: exactly one root element and no text
  outside it. An empty file, plain text or two concatenated reports fail, so a
  runner that crashed before writing cannot pass for an empty suite.
- `--file-from classname` or `--file-from suite` reads the path from that
  attribute instead, and `--path-prefix` goes in front of every path, after
  the path as written passes the identifier rules, so `/a` cannot become
  `tests/a`. The user
  sets them once, following the runner's guide; Playwright needs both.
- A plain two-column format, identifier and milliseconds, is accepted for any
  runner that has no usable report.
- Reports are untrusted input. Bound their size, depth and entry count.
- `record` compares the report with the machine's manifest. A file that was
  not assigned to the machine fails with status 1, because it may have run
  twice. If nothing matches, the flags are more likely wrong, and the error
  says so with status 2. An assigned file missing from the report is a
  warning.
- With no reports, `record` writes an empty run report, but only when the
  machine's share was empty.
- Golden reports from pytest, Jest, Vitest, RSpec and Playwright come from the real runners. A
  script in `testdata/` runs each one, at a pinned version, on a small sample
  suite, and its output is committed. CI reads the committed files and installs
  no runner.

### Command-line interface

- Every file Bunpō reads or writes lives in one directory, `.bunpo/` by
  default, set with `--dir`. CI caches or uploads that one folder.

  | File | Written by | Read by |
  | --- | --- | --- |
  | `timings.json` | `merge` | `split`, `plan`, `merge` |
  | `manifest-<index>.json` | `split` | `record`, `verify` |
  | `report-<index>.json` | `record` | `merge` |
  | `plan.json` | `plan` | `split` |

  Per-machine files carry the machine index in their names, so the final job
  can download every machine's folder into one without collisions.
- `split`, `plan` and `verify` read the test list from standard input, one
  identifier per line. Lines may end in LF or CRLF; a carriage return
  anywhere else stays in the identifier and fails the identifier rules.
- An empty test list is an error. A discovery command that found nothing, or
  failed, would otherwise pass with no tests run.
- Standard output carries only file lists, so it can be piped into a test
  command. Messages and warnings go to standard error.
- Every file Bunpō writes is versioned JSON, read as strictly as the timing
  file: unknown members, duplicate keys and invalid UTF-8 are errors, and so
  is an unknown version. Every member is required, because a missing one
  would read as zero: a run report without `index` would pass for machine 0.
- Files are written to a temporary name and renamed into place, so a failed
  write leaves the previous file intact.
- `split` writes the test list before the manifest, and fails if the list
  cannot be written. A list that never reached the runner leaves no manifest
  for `verify` to accept.
- `merge` needs a run report from every machine. A missing one fails with
  status 1 and leaves the timing file unchanged, because a run with a machine
  missing has incomplete timings.
- A report from an earlier run must never stand in for the current one.
  `merge` deletes the reports it merged, `split` starts by deleting its own
  machine's leftover manifest and report, and `plan` starts by deleting any
  leftover `plan.json`, so a reused directory cannot count a run twice and a
  failed split or plan leaves no stale file behind.
- Exit codes: 0 for success, 1 when a check fails (`verify` finds a mismatch,
  `merge` finds a test in two reports), 2 for bad usage or bad input (an
  unknown flag, an unreadable file, an invalid identifier). Warnings, such as a
  dominant file or empty shards, go to standard error and still exit 0. This
  matches `diff` and `grep`, so a workflow can tell a failed check from a
  mistake in the call.

## Technical choices

| Area | Choice | Notes |
| --- | --- | --- |
| Language and build | Go 1.27, one module, patch pinned by the `toolchain` directive in `go.mod` | The `go` line states the minimum. The `toolchain` line names the patch Go downloads automatically (1.27.4 on 2026-11-16). CI reads it through `setup-go`; release builds set `GOTOOLCHAIN` to that exact version. No extra version manager is required. |
| Dependencies | Standard library only | `flag`, `encoding/xml`, `crypto/sha256`. A tool that runs in other people's CI should bring nothing with it. |
| Timing file | `encoding/json/v2` | It rejects duplicate keys and invalid UTF-8, which is right for a new format. |
| File access | One `os.Root` per report or timing directory the user names | The directory can be anywhere, such as an absolute cache path. File names read inside it cannot leave it through `..` or a symlink. |
| Output | File lists on standard output, messages on standard error | Lets the list be piped straight into a test command. |
| Tools | Staticcheck and `govulncheck` pinned with `tool` directives | Run through `go tool`. |
| Release | Cross-compiled binaries with checksums on GitHub releases | Linux, macOS and Windows, x86-64 and ARM64. |
| Hosted queue, later | Cloudflare Workers with one Durable Object per build | A separate small codebase. Not part of the first build. |

Go is chosen because it produces one file per platform from one machine and
starts in milliseconds. People will download this at the start of every CI
run, so both matter.

### Package layout

```text
cmd/bunpo/               subcommands, flags, files, messages and exit codes
internal/split/          sort, deal out, digest; plain functions
internal/timing/         the timing file, median, merge, pruning
internal/junit/          bounded JUnit XML and two-column report reading
internal/manifest/       manifests, plans, run reports and the verify check
internal/jsonfile/       strict versioned-JSON encoding and decoding
```

`internal/split` takes a list and a map of durations and returns lists. It
reads no files and no environment variables, so the whole algorithm is tested
as plain functions.

### Verification

- Table-driven tests for the split, including the worked example, ties, an
  empty list, one test, more machines than tests and missing timings.
- A property test: for any list, timings and machine count, the lists are
  disjoint, their union is the input, and shuffling the input changes nothing.
- For small generated cases, enumerate assignments with an independent bounded
  solver to find the exact optimum. Check `3 * LPT <= 4 * optimum` with checked
  integer arithmetic. Bound the job count, machine count and durations so the
  oracle remains cheap.
- Keep four jobs of duration 3 on three machines as a regression case: LPT and
  the optimum are 6, while `max(longest, total / machines)` is only 4.
  Four thirds of that lower bound is not a valid ceiling for LPT.
- A report where one case in a file failed: `record` writes no time for that
  file in that run and keeps its earlier times.
- Fuzz the JUnit reader and the timing-file reader with bounded input.
- Golden tests for real reports from pytest, Jest, Vitest, RSpec and Playwright, kept
  in the repository.
- An end-to-end test that runs the built binary as eight processes and then
  `verify`, once clean and once with one machine given a stale timing file.
- A workflow fixture with valid manifests but one skipped runner: assignment
  verification passes with that label, while the missing-report check fails.
  Exercise a dominant file and empty shards without asserting a 10% balance.
- CI runs `gofmt`, `go vet`, Staticcheck, `govulncheck` and `go test -race`.
  Bunpō splits its own tests once it can.

## Initial limits

Starting guesses. Revise them from the first real users.

| Resource | Starting value | Why |
| --- | --- | --- |
| Tests in one split | 200,000 | Far above a large monorepo's file count. |
| Machines | 256 | More than any matrix I expect; bounds the manifest set. |
| One JUnit report / all reports | 64 MiB / 1 GiB | Real reports are a few megabytes. |
| JUnit nesting / test cases per report | 32 levels / 2,000,000 | Real reports nest four levels. |
| Test list on standard input | 64 MiB | 200,000 paths at about 300 bytes each. |
| Timing file | 64 MiB | 200,000 tests at about 250 bytes each: a path, five times and a last-seen run. |
| Times kept per test | 5 | Enough for a stable median. |
| Runs before an unseen test is dropped | 30 | Deleted tests leave the file within weeks. |
| Longest single duration | 30 days | Far above any real file, and 200,000 of them still fit in a 64-bit total. |
| Run time of `split` | Under 1 second for 50,000 tests | It runs on every machine, every build. Measure it. |

## Distribution and license

- **Download:** binaries on GitHub releases, with checksums. A two-line
  install step for any CI.
- **GitHub Action:** downloads the right binary, verifies it against
  `SHA256SUMS`, and puts `bunpo` on `PATH` for workflow `run` steps.
- **Hosting:** none. Timings live in the user's own CI cache.
- **Website:** a static Astro site on Cloudflare Pages at `bunpo.org`, in its
  own `bunpo-site` repository. It holds the pytest, Jest, Vitest, RSpec and Playwright guides, a
  before-and-after chart of machine times from a real suite, the install
  step, a changelog with one entry per release, and the GitHub link. The
  README carries a short terminal recording of split, record, merge and
  verify. The tool never contacts the site.
  Pin Bun for the site, commit `bun.lock`, and run `bun run build`. Before
  publishing, open the built site and check the guides and install links.
- **License: Apache-2.0.** Companies add CI tools without a legal review when
  the license is permissive. This differs from Bientôt's AGPL on purpose: this
  is a tool people run, not a service someone could resell.

## Making money

- Free, forever: the whole command-line tool.
- Paid, later: a hosted queue. Machines ask a central service for the next
  test as they go, so a slow machine automatically gets less work and a
  crashed machine's unfinished assignments can be handed to another. An
  uncertain attempt may already have executed, so reassignment can duplicate
  execution. The service also shows attempt history, the slowest tests, tests
  that fail at random, and how the suite's time changes.
- Knapsack Pro shows that teams pay for this. Their current prices are not
  yet checked.
- This may never be a business. A free tool that many teams use is still a
  good outcome and a strong portfolio piece.

## Open questions

- Do teams accept a last "merge and verify" job, or do they want a plan job
  at the start?
- How do CI caches behave when eight machines save at once? GitHub's cache
  keys cannot be overwritten, which is why merging happens in one final job.
- Is file-level splitting enough, or do the first users have one giant file
  that dominates?

## Roadmap

### Phase 1 — The split engine, `0.0.1`

Build `internal/split` and `internal/timing` as plain functions. Write the
table-driven and property tests first.

**Done when:** the worked example passes and the property tests find no
overlap, no missing test and no order dependence. A POSIX inventory containing both `tests/a\b` and `tests/a/b` must fail rather than verify a one-file split.

### Phase 2 — The command-line tool, `0.0.2`

Add `split`, `plan`, `record`, `merge` and `verify`, the JUnit reader, the
manifests and the limits. Run the eight-process end-to-end test.

**Done when:** `verify` passes on a clean run and fails on a stale timing
file, with a message that names the machine that differed.

### Phase 3 — First public release, `0.0.3`–`0.0.6`

Add cross-compiled binaries with checksums, the GitHub Action, and guides for
pytest, Jest, Vitest, RSpec and Playwright. Try it on two real open-source projects with slow CI,
with their maintainers' agreement. Publish the website and add the terminal
recording to the README.

**Done when:** someone else adds Bunpō to their project from the guide alone,
assignment verification and runner/report checks catch their respective failure
fixtures, and a published known-feasible benchmark meets the 10% predicted-load
target. Include an indivisible-file case; report repeated measured times as well
as predicted loads, explaining any measured imbalance.

### Phase 4 — Hosted queue pilot, `0.1.0`

Build the live queue on Cloudflare Workers, with one Durable Object owning
each build's list. Add a `--queue` mode to the program. Run it free for a few
invited teams.

Give each assignment a lease generation and each execution attempt a stable ID.
Accept at most one terminal result per logical test under the current lease;
duplicate submissions are idempotent and stale results remain attempt history.
A lost completion report leaves an uncertain attempt that may be reassigned.
Tests must tolerate repeat execution or use runner-side isolation. Bound retries;
exhaustion leaves an explicit failed or unknown build, never a fabricated pass.

**Done when:** a pilot team's build survives one machine being killed
before execution and after execution but before reporting. Every logical test
has one accepted terminal result or an explicit unresolved outcome; duplicate
executions and stale results remain visible. No missing result can pass the build.

### Phase 5 — Paid plan, `0.2.0`

Add accounts, the dashboard, usage limits and billing.

**Done when:** a stranger can sign up, pay, use the queue and leave with
their timing history.

## Later ideas

- Go support, where the unit is a package and not a file.
- Splitting inside one very large test file, for runners that can select
  single tests.
- Detecting the machine index and count from each CI system's own variables.
- Flaky-test reports from the history the tool already keeps.
- More runners: Cypress, Minitest, PHPUnit and Bun test. Each needs a golden
  report and a guide, and a check that it runs exactly the files it is given.

## What to expect

- **Time:** about four to six weeks of evenings and weekends to reach
  `0.0.3`. The hosted queue is a separate project of similar size.
- **The hardest part of the first build:** mapping each runner's report to
  files, and writing guides clear enough to follow without help.
- **Signs it is working:** GitHub stars, the Action's usage count, and
  projects that keep it after a month.

## Name, domain and mascot

**Bunpō** (分蜂) is the Japanese word for a bee colony dividing: part of the
hive leaves to work somewhere else. A test suite divides across machines the
same way. Use `bunpo` without the accent for the repository, the module and
the binary.

### Domain

The author owns `bunpo.org`. The website and guides go there.

### Mascot — Mitsu, the bee

Mitsu is a round honey-brown and cream bee with mint wings, amber pollen
baskets and a divided honeycomb satchel. She hands out honeycomb cells to a
row of smaller bees so that each carries the same weight.

A cell glows only when `verify` has passed. An unverified split shows plain
cells. Use honey brown, cream, mint and amber. The small icon is Mitsu's face
over one hexagon. Prepare still, monochrome and text-labelled versions.
Results and errors stay in plain text. Commission original licensed artwork
once the tool works.

## References

- [Knapsack Pro overview](https://docs.knapsackpro.com/overview/) and the [archived Knapsack gem](https://docs.knapsackpro.com/ruby/knapsack)
- [split_tests](https://github.com/leonid-shevtsov/split_tests), [split-test](https://github.com/mtsmfm/split-test), [ci_split_tests](https://github.com/payfazz/ci_split_tests), [pytest-split](https://pypi.org/project/pytest-split/) and [testris](https://pkg.go.dev/github.com/mcncl/testris)
- [Depot test splitting, July 2026](https://depot.dev/changelog/2026-07-15-test-splitting)
- [Go](https://go.dev/doc/), [Go releases](https://go.dev/doc/devel/release), [os.Root](https://pkg.go.dev/os#Root) and [encoding/json/v2](https://pkg.go.dev/encoding/json/v2)
