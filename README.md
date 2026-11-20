# Bunpō 分蜂

A small open-source program that splits a test suite across parallel CI
machines to balance their predicted load, and verifies that every input test
file was assigned exactly once.

Status: in development. The split engine and the timing file are built; the
`bunpo` commands are not, so nothing is usable yet.
[PROJECT.md](PROJECT.md) holds the specification and the roadmap, and
[docs/book.md](docs/book.md) explains how the code works today.

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
