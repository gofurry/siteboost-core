# Smoke Test

## Quick Verification Steps

Run these commands from the repository root:

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

## Required Commands

- `go mod tidy`: verifies module metadata.
- `gofmt -w .`: formats Go source files.
- `go vet ./...`: runs Go static checks.
- `go test ./...`: runs all tests and builds all packages.
- `go run ./cmd/steam-accelerator --version`: verifies the CLI entry builds.
- `go run ./examples/basic`: verifies the basic example builds.

## Expected Output

The CLI should print project name, version, and module path.

The basic example should print the project name and module path.

## Common Failure Cases

- Go is not installed or is older than the `go.mod` directive.
- A generated file was not formatted by `gofmt`.
- A future package introduces a dependency but `go mod tidy` was not run.
- A command imports an internal package before the package is implemented.
