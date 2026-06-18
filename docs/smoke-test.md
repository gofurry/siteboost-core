# Smoke Test

## Quick Verification Steps

Run these commands from the repository root:

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/hosts ./internal/certstore ./internal/reverse ./internal/pac ./internal/systemproxy ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine ./internal/runtime
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

The same lifecycle can be checked with an explicit proxy_only configuration file:

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

## PAC and System Proxy Check

These checks modify the current user's system proxy settings on Windows or macOS and should restore them on `stop`.

PAC mode:

```bash
go run ./cmd/steam-accelerator start --mode pac --state ./tmp/runtime.json
curl http://127.0.0.1:26502/proxy.pac
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

System Proxy mode:

```bash
go run ./cmd/steam-accelerator start --mode system --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

Crash recovery:

```bash
go run ./cmd/steam-accelerator restore
```

## Windows Hosts and HTTPS Reverse Proxy Check

These checks modify the current user's Root certificate store and the Windows hosts file. Use an Administrator PowerShell for Hosts mode, and run `stop` plus `cert uninstall` after testing.

Install this project's root CA:

```bash
go run ./cmd/steam-accelerator cert install
```

Start with high ports to avoid occupying 80 / 443:

```bash
go run ./cmd/steam-accelerator start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

Check status from another terminal:

```bash
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
```

Real browser testing with hosts requires default 80 / 443 and free local ports. High ports are mainly for reverse-server lifecycle checks.

Stop and uninstall the CA:

```bash
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
go run ./cmd/steam-accelerator cert uninstall
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
- A non-system resolver mode is selected without `resolver.servers`.
- An HTTP or SOCKS5 upstream is selected without `upstream.address`.
- Port `127.0.0.1:26502` is already in use for PAC mode.
- The current OS is not Windows or macOS for PAC/System Proxy modes.
- Ports 80 / 443 are already in use for Hosts mode.
- Hosts mode was started before `cert install`.
- Windows hosts writing failed; use an Administrator terminal.
- A rollback state remains after restore failure; run `restore` again after fixing the platform error.
- A stale state file points to an old process; `status` or `stop` should remove it.
