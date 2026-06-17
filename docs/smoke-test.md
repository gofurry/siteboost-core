# Smoke Test

## Quick Verification Steps

Run these commands from the repository root:

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine
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

The same lifecycle can be checked with an explicit v0.2.0 configuration file:

```yaml
mode: proxy_only

resolver:
  mode: "system"
  prefer_ipv4: true

upstream:
  type: "direct"
```

Then start with:

```bash
go run ./cmd/steam-accelerator start --config ./tmp/proxy-system-direct.yaml --state ./tmp/runtime.json
```

DoH and HTTP/SOCKS5 upstream behavior should be covered with local fake servers in `go test ./internal/resolver ./internal/upstream ./internal/proxy`. For manual checks, configure `resolver.mode: doh` with an explicit `servers` URL, or set `upstream.type` to `http`/`socks5` with a local proxy address.

## Expected Output

The version command should print project name, version, and module path.

The basic example should print the project name and module path.

`status` should show `running: true` while the foreground `start` process is active. `stop` should ask the foreground process to shut down and print `stopped` or `stop requested`.

## Common Failure Cases

- Go is not installed or is older than the `go.mod` directive.
- A generated file was not formatted by `gofmt`.
- A future package introduces a dependency but `go mod tidy` was not run.
- Port `127.0.0.1:26501` is already in use.
- A non-system resolver mode is selected without `resolver.servers`.
- An HTTP or SOCKS5 upstream is selected without `upstream.address`.
- A stale state file points to an old process; `status` or `stop` should remove it.
