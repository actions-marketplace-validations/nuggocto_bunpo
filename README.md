# Bunpō 分蜂

A small open-source program that splits a test suite across parallel CI
machines to balance their predicted load, and verifies that every input test
file was assigned exactly once.

Status: planning. No commands work yet. [PROJECT.md](PROJECT.md) holds the
specification and the roadmap.

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
