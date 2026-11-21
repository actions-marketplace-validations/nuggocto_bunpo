# Bunpō 分蜂

Bunpō splits a test suite across parallel CI machines by how long each test
file took last time, and checks that every file was assigned exactly once.

Split test files by name and one machine gets the slow ones while the others
sit idle. Bunpō remembers each file's time and deals them out longest first,
so every machine finishes at about the same moment. It is one small binary
with no dependencies and no server: timings live in your own CI cache.

```text
$ find tests -name 'test_*.py' | bunpo split --index 0 --total 4
bunpo: predicted machine loads from 5s to 5s, average 5s; longest file tests/test_f08.py at 900ms
bunpo: machine 0 of 4 gets 10 of 40 tests, predicted 5s
tests/test_f03.py
tests/test_f04.py
tests/test_f05.py
...

$ find tests -name 'test_*.py' | bunpo verify
bunpo: assignment verified: 40 tests across 4 machines, each assigned exactly once, all split from the same input and timings (digest ab97d584f852…)
```

The test list goes to standard output, ready for your test command; messages
go to standard error. In GitHub Actions, warnings and failures show up as
annotations on the run and the pull request.

## Get started

Follow the guide for your test runner. Each one is a complete GitHub Actions
workflow:

- [pytest](docs/guides/pytest.md)
- [Jest](docs/guides/jest.md)
- [Vitest](docs/guides/vitest.md)
- [RSpec](docs/guides/rspec.md)
- [Playwright](docs/guides/playwright.md)

In GitHub Actions, one step installs Bunpō:

```yaml
- uses: nuggocto/bunpo@v0
```

Elsewhere, download the binary for your platform from the
[releases](https://github.com/nuggocto/bunpo/releases) and check it against
`SHA256SUMS`. Releases start at `v0.0.3` and cover Linux, macOS and Windows on
x86-64 and ARM64.

## How it works

1. **`bunpo split`** on each machine reads the list of test files and prints
   that machine's share. The same list and timings give the same split on
   every machine.
2. Your test runner runs exactly those files and writes a JUnit report.
3. **`bunpo record`** turns the report into that machine's timings.
4. A final job runs **`bunpo verify`**, which checks that every machine split
   the same input and that each file was assigned exactly once, and
   **`bunpo merge`**, which folds the timings into the file your CI caches for
   next time.

`verify` checks what each machine was asked to run. Your workflow still checks
that each runner succeeded; the guides do both.

On a declared benchmark of 40 files, the slowest machine's predicted load was
within 2.1% of the fastest; [docs/benchmark.md](docs/benchmark.md) has the
numbers, including a suite where one file is too large to balance.

## Documentation

- [docs/guides/](docs/guides/): setup for each test runner.
- [docs/book.md](docs/book.md): how the code works, every command, file and
  rule.
- [docs/benchmark.md](docs/benchmark.md): measured results.
- [CHANGELOG.md](CHANGELOG.md): release history.
- [PROJECT.md](PROJECT.md): the specification and the roadmap.

## Development

Bunpō needs Go 1.21 or later installed. Go downloads the exact toolchain named
in `go.mod` on first use.

```sh
gofmt -l .                  # must print nothing
go vet ./...
go tool staticcheck ./...
go test -race ./...
go tool govulncheck ./...
go build -o bin/bunpo ./cmd/bunpo
```

Licensed under Apache-2.0.
