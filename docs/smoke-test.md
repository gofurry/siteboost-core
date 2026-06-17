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

## CLI Runtime Check

Start the proxy in one terminal:

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
```

In another terminal:

```bash
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

## Expected Output

The version command should print project name, version, and module path.

The basic example should print the project name and module path.

`status` should show `running: true` while the foreground `start` process is active. `stop` should ask the foreground process to shut down and print `stopped` or `stop requested`.

## Common Failure Cases

- Go is not installed or is older than the `go.mod` directive.
- A generated file was not formatted by `gofmt`.
- A future package introduces a dependency but `go mod tidy` was not run.
- Port `127.0.0.1:26501` is already in use.
- A stale state file points to an old process; `status` or `stop` should remove it.
