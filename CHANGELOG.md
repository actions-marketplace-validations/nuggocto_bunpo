# Changelog

All notable changes to Bunpō are recorded here. Release binaries and checksums
start at `v0.0.3` on [GitHub Releases](https://github.com/nuggocto/bunpo/releases).

## [v0.0.4] - 2026-11-21

### Fixed

- Require nested JSON members in `timings.json` (`samples`, `last_seen`) and run reports (`timed`, `ms`), reject `null` shards in `plan.json`, and return zero values on decode validation errors.
- Reject non-canonical test identifiers in `timings.json` and `merge` reports.
- Flag out-of-range manifest indexes and sort missing test lists deterministically in `manifest.Verify`.
- Validate `--path-prefix` upfront in `bunpo record` (including on empty shards) and sort report map keys so errors are deterministic.
- Delete any leftover `plan.json` at the start of `bunpo plan` so a failed run never leaves a stale plan behind.
- Fix singular grammar (`1 of N tests has ... and counts`) and avoid printing `0ms` predictions when `timings.json` has no matching tests.
- Validate the Action's `version` input and `releases/latest` redirect against a release-tag pattern, and retry the `releases/latest` lookup.
- Check the `v0` major tag using GitHub's exact-match ref endpoint in the release workflow.
- Record passing packages and upload `.bunpo/` artifacts in dogfood CI even when a Go test fails.

### Changed

- Vitest guide now passes the test list through `BUNPO_TESTS_FILE=mine.txt` instead of an environment variable containing every path, avoiding Linux's 128 KiB per-variable limit on large suites.
- Renamed the GitHub Action from `Bunpō` to `Bunpo` in `action.yml` for ASCII searchability, and documented passing `with: version` when pinning the Action to a commit SHA.
- Updated `PROJECT.md` with a verified comparison against `split_tests` and `split-test`, and updated `docs/book.md` to match current behavior.

### Added

- Direct unit tests for `internal/jsonfile`, size and line-limit tests for two-column report reading, and regression tests for nested JSON validation.

## [v0.0.3] - 2026-11-21

First release with cross-compiled binaries, checksums, GitHub Action, runner guides, and benchmarks.

### Added

- Cross-compiled static binaries (`CGO_ENABLED=0`, `-trimpath`) with `SHA256SUMS` for Linux, macOS, and Windows on x86-64 and ARM64 (`scripts/build-release.sh` and `.github/workflows/release.yml`).
- Installing GitHub Action (`action.yml` and `scripts/install.sh`) that resolves exact tags, major tags (`@v0`), or `latest`, verifies `SHA256SUMS`, and puts `bunpo` on `PATH`.
- Complete GitHub Actions guides for pytest, Jest, Vitest, RSpec, and Playwright (`docs/guides/`), verified against real runners with `guidecheck`.
- Terminal label colors (`NO_COLOR`-aware, truecolor and 256-color) and GitHub Actions workflow annotations (`::warning::`, `::error::`, `::notice::`).
- Synthetic pytest benchmark suite (`benchmark/`) and published results (`docs/benchmark.md`), meeting the 1.10 predicted-load target (1.021 on the 40-file suite) alongside tight and indivisible-file cases.
- Dogfood CI (`scripts/dogfood.sh`) splitting Bunpō's own Go test packages across parallel runners and testing `action.yml` across Linux, macOS, and Windows.
- Architectural decision record for releases and the installing Action (`docs/decisions/0002-release-and-action.md`).

### Fixed

- Clear a machine's old `manifest-<index>.json` and `report-<index>.json` at the start of `bunpo split`, and reject empty or malformed test lists.
- Hash the downloaded binary from standard input in `scripts/install.sh` so checksum verification works on Windows paths.
- Test each published release through `action.yml` on Linux, macOS, and Windows before moving the `v0` major tag.

## [v0.0.2] - 2026-11-20

### Added

- Full command-line tool (`cmd/bunpo`): `split`, `plan`, `record`, `merge`, `verify`, and `version` with distinct exit codes (0 success, 1 failed check, 2 bad usage or input).
- Bounded JUnit XML and two-column (`--format lines`) report readers (`internal/junit`) using exact rational sums rounded half up once per file, with golden reports generated from real pytest, Jest, Vitest, RSpec, and Playwright suites plus fuzz testing.
- Versioned manifest, plan, and run report formats and the `manifest.Verify` checker (`internal/manifest`), plus a shared strict JSON helper (`internal/jsonfile`).
- Atomic file writes (temp file, `fsync`, rename) and directory confinement through `os.Root`.
- Multi-process end-to-end tests covering 8 parallel machines, stale timing caches, skipped runners, dominant files, empty shards, and timing carryover.
- Architectural decision record for file paths in test reports (`docs/decisions/0001-file-paths-in-test-reports.md`).

## [v0.0.1] - 2026-11-20

### Added

- Core split engine (`internal/split`): canonical repository-relative identifiers with POSIX/Windows path rules and collision detection, deterministic longest-first integer-only dealing with test-count tie-breaking, and SHA-256 split digests.
- Timing file engine (`internal/timing`): strict versioned JSON (`encoding/json/v2`), integer half-up median over the last 5 samples, order-independent `Merge` that keeps failed files seen without recording their time, and pruning after 30 unseen runs.
- Table-driven tests, 2,000-seed property tests, a brute-force optimal solver oracle verifying the 4/3 makespan bound, fuzzing for timing decoding, and a 50,000-test benchmark.
- Go 1.27 module with pinned `staticcheck` and `govulncheck` tools, GitHub Actions CI, specification (`PROJECT.md`), and developer guide (`docs/book.md`).

[v0.0.4]: https://github.com/nuggocto/bunpo/compare/v0.0.3...v0.0.4
[v0.0.3]: https://github.com/nuggocto/bunpo/compare/v0.0.2...v0.0.3
[v0.0.2]: https://github.com/nuggocto/bunpo/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/nuggocto/bunpo/releases/tag/v0.0.1
