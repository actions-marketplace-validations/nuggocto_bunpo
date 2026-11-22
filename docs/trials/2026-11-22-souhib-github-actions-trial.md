# Live GitHub Actions trial on LaTabdhir

Date: 2026-11-22. Bunpō `v0.0.5`, installed through `nuggocto/bunpo@v0`.

Souhib and his agents wired Bunpō into [Souhib/LaTabdhir](https://github.com/Souhib/LaTabdhir)'s
Chromium Playwright workflow (`.github/workflows/e2e.yml`, commit `8a1f1cf5`)
using only the [Playwright guide](../guides/playwright.md), with no help from
the Bunpō repository or author.

## Summary

- `nuggocto/bunpo@v0` resolved and installed `v0.0.5` on both self-hosted Linux
  runners (machine 0: `linux/amd64`; machine 1: `linux/arm64`) with standard
  `contents: read` permissions.
- All **68 specs / 232 cases** (`--project=chromium`, `--grep-invert @payments`)
  ran in both the cold and warm GitHub Actions runs. `bunpo verify` and
  `bunpo merge` exited `0` in both runs, taking 16–17 seconds total.
- The cold 2-machine workflow took **9m 58s** end-to-end vs. **19m 47s** for
  the single-machine baseline (**49.6% shorter**).
- The warm run restored `timings.json` from the cache saved by the cold run
  (`bunpo-e2e-chromium-timings-v1-44997889476-1`), predicted an exact
  **4m42s vs. 4m42s** split (31 vs. 37 specs; longest file
  `tests/ui/accessibility.spec.ts` at 57s), and cut the slowest browser job
  from **8m 46s** to **7m 32s** (ARM64 Playwright wall dropped from 7m 31s to
  6m 29s).

## Setup

- Bun `1.4.4`, Playwright `1.58.2`, `workers: 1` per machine (each runner
  starts and seeds its own Docker Compose stack).
- Commands run in `e2e/`; cache and artifact steps use repository-relative
  `e2e/.bunpo/` paths with Chromium-specific cache and artifact prefixes.
- Filtered discovery writes Playwright's JSON listing to a file
  (`PLAYWRIGHT_JSON_OUTPUT_NAME`) and extracts sorted spec paths for both
  `split` and `verify`. Shards run exact files through `--test-list`, and an
  empty shard records with no report and skips Compose startup.
- Only default-branch pushes save `timings.json`. The warm run was triggered
  manually on `main`, so it restored the cache and skipped the save step.

## Measurements

Playwright wall time includes global setup and teardown. Job wall time also
includes checkout, dependencies, Docker Compose startup, Bunpō, and cleanup.
Start delay is measured from workflow creation to job start.

| Phase | Host | Specs | Cases | Compose startup | Playwright wall | Job wall | Start delay |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Baseline | ARM64 | 68 | 232 | 5m 22s | 13m 22s | 19m 12s | 34s |
| Cold 0 | x64 | 34 | 114 | 1m 45s | 3m 35s | 5m 48s | 10s |
| Cold 1 | ARM64 | 34 | 118 | 39s | 7m 31s | 8m 46s | 53s |
| Warm 0 | x64 | 31 | 100 | 22s | 3m 32s | 4m 21s | 13m 09s |
| Warm 1 | ARM64 | 37 | 132 | 33s | 6m 29s | 7m 32s | 7s |

| Run | Slowest browser job | Browser-job span | Browser-job overlap | Verify + merge job | Workflow elapsed (incl. queue) |
| --- | ---: | ---: | ---: | ---: | ---: |
| Single machine | 19m 12s | 19m 12s | — | — | 19m 47s |
| Cold | 8m 46s | 9m 29s | 5m 05s | 17s | 9m 58s |
| Warm | 7m 32s | 17m 23s | 0s | 16s | 17m 49s |

Run links (collaborators):
[Baseline](https://github.com/Souhib/LaTabdhir/actions/runs/44924648220) ·
[Cold run](https://github.com/Souhib/LaTabdhir/actions/runs/44997889476) ·
[Warm run](https://github.com/Souhib/LaTabdhir/actions/runs/44998666297).

Two operational factors separate predicted balance from measured workflow wall
time on this self-hosted setup:

- **Heterogeneous hardware:** for the exact same 4m42s predicted weight in the
  warm run, x64 finished Playwright in 3m 32s while ARM64 took 6m 29s.
- **Runner queue contention:** during the warm run, another CI job occupied the
  x64 runner for 13m 09s, so the two shards did not overlap. Static file-level
  splitting cannot shift work across mixed-speed or delayed runners; that is
  what a live queue addresses.

## Failure guards verified on `v0.0.5`

| Injected condition | Observed result |
| --- | --- |
| Machine 1 omits `record` (manifest kept) | `verify` exits 0; `merge` exits 1, names machine 1, and leaves `timings.json` byte-identical. |
| Machine 1's entire artifact is omitted | `verify` exits 1 because `manifest-1.json` is also missing. |
| One file dropped from machine 1's input before `split` | `verify` exits 1; with `GITHUB_ACTIONS=true`, the error annotation names machine 1 and the unassigned file. |
| One file omitted from execution after `split` | `verify` exits 0 (assignment complete); `record` warns that the file had no reported cases. |
| One spec total across two machines | Empty shard records with no report; `verify` and `merge` exit 0. |

## Friction found and addressed in Bunpō

1. **Playwright JUnit retry timings:** Souhib's fixture on Playwright `1.58.2`
   showed attempts of 104, 105, and 23 ms summed to `0.232` s in the JUnit XML
   (excluding worker replacement and Compose overhead), contradicting our
   earlier note in `docs/guides/playwright.md`. Upstream Playwright's JUnit
   reporter sums `test.results` durations by default and records only the final
   passing attempt when `includeRetries: true` is set; updated the guide to
   reflect both.
2. **`bunpo --version` exited 2:** Only `bunpo version` was accepted. Added
   `-version` and `--version` aliases in `cmd/bunpo/main.go`.
3. **Heterogeneous self-hosted runners and queue delays:** Documented in
   `docs/guides/playwright.md` so teams on mixed hardware know to track slowest
   shard wall time separately from runner queue delay.
