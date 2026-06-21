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

DoH and HTTP/SOCKS5 upstream behavior should be covered with local fake servers in `go test ./internal/resolver ./internal/upstream ./internal/proxy`. For manual checks, configure `resolver.mode: doh`; empty `servers` use the built-in DoH defaults, and explicit URLs can override them. Configure HTTP/SOCKS5 upstreams only when an external proxy enhancement is needed.

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

The default Hosts + Direct loop should show `resolver: doh`, `resolver_servers:`, and `upstream_profiles: 2` in status output. That confirms outbound reverse-proxy resolution is not using the system resolver and will not loop back through the local hosts marker block. Starting in v0.6.0-dev, the default outbound profile also makes `steamcommunity.com` prefer `steamcommunity-a.akamaihd.net`, and `store.steampowered.com` / `checkout.steampowered.com` / `help.steampowered.com` / `login.steampowered.com` prefer `cdn-a.akamaihd.net`, while preserving the original HTTP Host.

If a page returns `upstream request failed`, the response body should include more than that generic message. It should append a summary such as `direct upstream resolve ... failed`, `resolve steamcommunity-a.akamaihd.net:443 failed`, `tcp 1.2.3.4:443 failed`, or `tls 1.2.3.4:443 failed`. That summary is the key check for locating whether failure happened in DoH, ForwardDestination resolution, direct TCP reachability, or TLS handshake.

Real browser testing with hosts requires default 80 / 443 and free local ports. High ports are mainly for reverse-server lifecycle checks.

Windows `curl.exe` uses Schannel and checks certificate revocation by default. Hosts reverse proxy mode dynamically issues local site certificates, which do not have public OCSP / CRL endpoints; if plain `curl.exe -I https://steamcommunity.com/` reports `CRYPT_E_NO_REVOCATION_CHECK`, the command-line client could not complete revocation checking and the outbound acceleration path is not necessarily failing. For command-line content checks, use:

```bash
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcommunity.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://store.steampowered.com/
```

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
- UDP/TCP resolver mode is selected without `resolver.servers`.
- An HTTP or SOCKS5 upstream is selected without `upstream.address`.
- Port `127.0.0.1:26502` is already in use for PAC mode.
- The current OS is not Windows or macOS for PAC/System Proxy modes.
- Ports 80 / 443 are already in use for Hosts mode.
- Hosts mode was started before `cert install`.
- Windows hosts preflight or writing failed; use an Administrator terminal.
- `upstream request failed` followed by `direct upstream resolve ... failed`: DoH/DNS failed or was blocked.
- `upstream request failed` followed by `resolve steamcommunity-a.akamaihd.net:443 failed` or `resolve cdn-a.akamaihd.net:443 failed`: the default Steam profile ForwardDestination failed to resolve.
- `upstream request failed` followed by `tcp ... failed`: candidate real IPs or ForwardDestination IPs are not directly reachable.
- `upstream request failed` followed by `tls ... failed`: the IP is reachable, but TLS/SNI/certificate behavior failed.
- A rollback state remains after restore failure; run `restore` again after fixing the platform error.
- A stale state file points to an old process; `status` or `stop` should remove it.
