# Bunpō with Playwright

Bunpō splits your test files across parallel GitHub Actions machines by how
long each took last time, and checks that every file was assigned exactly
once. This guide sets it up for Playwright in one workflow file.

The first run has no timings yet, so files are dealt out evenly. From the
second run on, each machine gets a balanced share.

## What you need

- Playwright's built-in JUnit reporter. It writes each test's path into the
  `classname` attribute, relative to your `testDir`, so `bunpo record` takes
  `--file-from classname --path-prefix tests`. Change `tests` in both places
  if your `testDir` is different.
- Playwright reads file arguments as regular expressions, so
  `tests/a.spec.ts` would also run `tests/x/tests/a.spec.ts`. The share goes
  through `--test-list` instead, which matches files exactly.

## The workflow

Save this as `.github/workflows/tests.yml`:

```yaml
name: Tests

on:
  push:
    branches: [main]
  pull_request:

# "shell: bash" turns on pipefail, so a failing command that lists the test
# files fails the step instead of passing an empty list to bunpo.
defaults:
  run:
    shell: bash

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        machine: [0, 1, 2, 3]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: 24
      - run: npm ci
      - run: npx playwright install --with-deps
      - uses: nuggocto/bunpo@v0
      - uses: actions/cache/restore@v6
        with:
          path: .bunpo/timings.json
          key: bunpo-timings-${{ github.sha }}
          restore-keys: bunpo-timings-
      - name: Run this machine's tests
        env:
          INDEX: ${{ strategy.job-index }}
          TOTAL: ${{ strategy.job-total }}
          PLAYWRIGHT_JUNIT_OUTPUT_FILE: junit.xml
        run: |
          find tests -name '*.spec.ts' -o -name '*.spec.js' | bunpo split --index "$INDEX" --total "$TOTAL" > mine.txt
          mine=()
          while IFS= read -r path; do mine+=("$path"); done < mine.txt

          # Given no files, Playwright would run the whole suite.
          if [ "${#mine[@]}" -eq 0 ]; then
            bunpo record --index "$INDEX"
            exit 0
          fi

          status=0
          # --test-list takes paths relative to testDir.
          sed 's|^tests/||' mine.txt > playwright-list.txt
          npx playwright test --test-list playwright-list.txt --reporter=list,junit || status=$?
          bunpo record --index "$INDEX" --file-from classname --path-prefix tests junit.xml
          exit "$status"
      - if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v7
        with:
          name: bunpo-${{ strategy.job-index }}
          path: |
            .bunpo/manifest-*.json
            .bunpo/report-*.json
          include-hidden-files: true

  bunpo:
    needs: test
    if: ${{ !cancelled() }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: nuggocto/bunpo@v0
      - uses: actions/cache/restore@v6
        with:
          path: .bunpo/timings.json
          key: bunpo-timings-${{ github.sha }}
          restore-keys: bunpo-timings-
      - uses: actions/download-artifact@v8
        with:
          pattern: bunpo-*
          path: .bunpo
          merge-multiple: true
      - name: Verify the assignment
        run: find tests -name '*.spec.ts' -o -name '*.spec.js' | bunpo verify
      - name: Merge timings
        run: bunpo merge
      - if: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
        uses: actions/cache/save@v6
        with:
          path: .bunpo/timings.json
          key: bunpo-timings-${{ github.run_id }}-${{ github.run_attempt }}
```

## How it works

- **Each `test` machine** restores the timing file, lists every test file,
  and runs `bunpo split` to get its share in `mine.txt`. It runs Playwright on
  exactly those files, then `bunpo record` turns the report into timings.
  The test exit status is kept and returned last, so a failing test still
  fails the job after its times are recorded.
- **A machine with no files** skips Playwright, which would otherwise run the
  whole suite, and records an empty report.
- **The `bunpo` job** collects every machine's files. `bunpo verify` checks
  that all machines split the same input and that each file was assigned
  exactly once. `bunpo merge` folds the times into the timing file and fails
  if a machine's report is missing.
- **Only `main` saves timings.** Pull requests read them and never overwrite
  them. Each save uses a new cache key, and `restore-keys` picks the newest.

Change `machine: [0, 1, 2, 3]` to use more or fewer machines. The actions are
shown by tag; pin them to commit SHAs if your policy asks for it.

## If something fails

- **`record` says files were not assigned to this machine.** The runner ran
  more files than it was given, so some ran twice. Check that the run
  command is exactly the one above.
- **`merge` says a machine's report is missing.** A `test` job stopped before
  uploading its files. Its own log says why.
- **`verify` says machines used different inputs or timings.** The machines
  listed different test files or restored different timing files. The
  message names the machines that differed.
- **`record` says none of the files were assigned to this machine.** The
  `--path-prefix` does not match your `testDir`.
