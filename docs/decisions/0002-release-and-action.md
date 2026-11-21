# 2. Release binaries and an installing Action

Date: 2026-11-21. Status: accepted.

## Context

Teams adopt Bunpō by downloading it at the start of every CI run, so the
release must be small, verifiable and the same on every machine. PROJECT.md
also asks for a GitHub Action that "exposes split, plan, record, merge and
verify as steps".

## Decision

- **A plain build script, not a release tool.** `scripts/build-release.sh`
  runs `go build` for six targets and writes `SHA256SUMS`. The release
  workflow calls it with the exact toolchain from `go.mod`. GoReleaser would
  add a tool to learn and pin for what a loop does.
- **Raw binaries, not archives.** One file per platform is the simplest thing
  for the Action and a `curl` to download and check.
- **The Action lives in this repository.** One tag versions the binary and the
  Action together, and `v0` moves with each release. The installed binary
  follows the ref: an exact tag, prerelease included, installs that release,
  and a major tag the newest stable release of that major, so `@v0` never
  installs a v1.
- **The Action installs; workflows run.** `action.yml` puts `bunpo` on `PATH`,
  and each command is a `run` step. `split` and `verify` read the test list
  from standard input, and a list built by `find` or `jest --listTests` has no
  natural place in a step input. As `run` steps, the commands read exactly as
  they do in the guides and on a laptop, and every flag stays available.

## Consequences

- A workflow calls `bunpo` directly, so the guides show one way of using it
  everywhere.
- CI tests the Action on all three operating systems with every change, by
  pointing the installer at a fresh build through `BUNPO_RELEASE_DIR`, before
  any release exists.
- A per-command Action interface can still be added later if users ask; it
  would wrap the same installer.

## Alternatives considered

- **GoReleaser.** More features (archives, changelogs, Homebrew), none needed
  yet, and a new tool in CI.
- **One input per command** (`with: command: split`). It hides the standard
  input the commands need and every flag behind strings.
- **A separate `bunpo-action` repository.** A cleaner Marketplace listing, at
  the cost of two version numbers to keep in step.
