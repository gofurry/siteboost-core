# Smoke Test

## Quick Verification Steps

Run these commands from the repository root:

```bash
git diff --check
go test ./...
go vet ./...
go test -race ./internal/hosts ./internal/privilege ./internal/engine ./cmd/steam-accelerator
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
go run ./cmd/steam-accelerator --version
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

These checks modify the Windows `LocalMachine\Root` certificate store by default and the Windows hosts file. Starting in v0.6.4, the recommended flow is to build a fixed local binary and run `apphost install` once from an Administrator PowerShell; later normal PowerShell runs must use the same binary path and request restricted system changes through the AppHost named pipe. Administrator PowerShell keeps the silent direct path. Run `stop` plus `cert uninstall` after testing.

Initialize AppHost:

```bash
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
./bin/steam-accelerator.exe apphost install
./bin/steam-accelerator.exe apphost status
```

### v0.6.4 Windows AppHost Main-Flow Record

On 2026-06-23, a real Windows machine on a China-network environment completed the AppHost + Hosts main-flow smoke. The record confirms that AppHost installs as an automatic delayed-start service, health checks work, Steam target hosts are taken over to `127.0.0.1` with local TCP 443 reachable, `stop` / `restore` keep AppHost healthy as the privileged standby service, and `apphost status` reports a clear error after uninstall.

Administrator PowerShell:

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost install
apphost installed and started

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
apphost: running start_type=automatic delayed_auto_start=true pid=98084 health=ok
```

From a normal PowerShell, `already running` is acceptable when a previous acceleration instance is active. Continue by checking hosts takeover and local 443 reachability:

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe start --mode hosts
error: steam-accelerator is already running

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection steamcommunity.com -Port 443
ComputerName     : steamcommunity.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection store.steampowered.com -Port 443
ComputerName     : store.steampowered.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection help.steampowered.com -Port 443
ComputerName     : help.steampowered.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True
```

Stop, restore, and uninstall:

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe stop
stopped

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe restore
restored

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
apphost: running start_type=automatic delayed_auto_start=true pid=98084 health=ok

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost uninstall
apphost uninstalled

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
error: open apphost service: The specified service does not exist as an installed service.
```

This record does not replace a dedicated real-reboot smoke. Before release, still record:

```powershell
.\bin\steam-accelerator.exe apphost install
# reboot Windows
.\bin\steam-accelerator.exe apphost status
.\bin\steam-accelerator.exe start --mode hosts
```

Optional pre-install for this project's root CA. When `cert.auto_install` is true, `start --mode hosts` can install it automatically during startup; from a normal PowerShell, this command requests restricted system changes through AppHost:

```bash
./bin/steam-accelerator.exe cert install
```

Start with high ports to avoid occupying 80 / 443:

```bash
./bin/steam-accelerator.exe start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

Check status from another terminal:

```bash
./bin/steam-accelerator.exe status --state ./tmp/runtime.json
```

The default Hosts + Direct loop should show `provider: id=steam status=stable rule_set=steam-web@2026.06.22 profiles=4 probes=6`, `resolver: doh`, `resolver_servers:`, `rule_set: steam-web@2026.06.22`, `upstream_profiles: 4`, and `startup_probes:` in status output. The standalone `rule_set:` line is kept for the default single Steam provider smoke-reading habit. That confirms outbound reverse-proxy resolution is not using the system resolver and will not loop back through the local hosts marker block. Starting in v0.6.0, the default Steam outbound profile also makes `steamcommunity.com` prefer `steamcommunity-a.akamaihd.net`, `store.steampowered.com` / `checkout.steampowered.com` / `help.steampowered.com` / `login.steampowered.com` / `media.steampowered.com` prefer `cdn-a.akamaihd.net`, and covers `community.steamstatic.com` plus `steamcdn-a.akamaihd.net`, while preserving the original HTTP Host.

To smoke the v0.7 provider skeleton, create a temporary config that explicitly enables GitHub:

```yaml
providers:
  enabled:
    - steam
    - github
```

Start with that config and check `status`. It should show both `provider: id=steam status=stable ...` and `provider: id=github status=experimental rule_set=github-web@2026.06.23 probes=3`. GitHub is a skeleton provider for architecture validation only; this smoke must not require live GitHub reachability or claim real GitHub acceleration.

`system_change:` lines should show the root CA check/install, hosts preflight, reverse-proxy listeners, and hosts apply result. When the normal PowerShell AppHost path succeeds, root CA or hosts details should include `helper=elevated`. `startup_probes: ok=6 failed=0` is the ideal result. If failures appear, inspect the `startup_probe_failed` lines before opening the browser; `stage=resolve`, `stage=tcp`, `stage=tls`, and `stage=http` narrow the failing layer. The default probe targets, exact hosts list, wildcard gaps, and manual record table are tracked in [Steam compatibility matrix](steam-compatibility.md).

If a page returns `upstream request failed`, the response body should include more than that generic message. It should append a summary such as `direct upstream resolve ... failed`, `resolve steamcommunity-a.akamaihd.net:443 failed`, `tcp 1.2.3.4:443 failed`, or `tls 1.2.3.4:443 failed`. That summary is the key check for locating whether failure happened in DoH, ForwardDestination resolution, direct TCP reachability, or TLS handshake.

Real browser testing with hosts requires default 80 / 443 and free local ports. High ports are mainly for reverse-server lifecycle checks.

Windows `curl.exe` uses Schannel and checks certificate revocation by default. Hosts reverse proxy mode dynamically issues local site certificates, which do not have public OCSP / CRL endpoints; if plain `curl.exe -I https://steamcommunity.com/` reports `CRYPT_E_NO_REVOCATION_CHECK`, the command-line client could not complete revocation checking and the outbound acceleration path is not necessarily failing. For command-line content checks, use:

```bash
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcommunity.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://store.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://community.steamstatic.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://media.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcdn-a.akamaihd.net/
```

Stop and uninstall the CA:

```bash
./bin/steam-accelerator.exe stop --state ./tmp/runtime.json
./bin/steam-accelerator.exe cert uninstall
```

When testing from a normal PowerShell, `stop` / `restore` for hosts recovery and `cert uninstall` for machine-scope root CA removal go through AppHost. If AppHost is missing or stopped, the command should return a clear error and should not create a new hosts marker block or a new partial rollback state.

## Expected Output

The version command should print project name, `v0.7.0-dev`, and module path.

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
- Hosts mode was started before `cert install`, with `cert.auto_install` set to false.
- Windows hosts preflight or writing failed; a normal PowerShell should go through AppHost named pipe, and AppHost must be installed once from an Administrator PowerShell. Custom `hosts.path` / `runtime.rollback_path` / `cert.dir` values require an Administrator terminal or default paths.
- `upstream request failed` followed by `direct upstream resolve ... failed`: DoH/DNS failed or was blocked.
- `upstream request failed` followed by `resolve steamcommunity-a.akamaihd.net:443 failed` or `resolve cdn-a.akamaihd.net:443 failed`: the default Steam profile ForwardDestination failed to resolve.
- `upstream request failed` followed by `tcp ... failed`: candidate real IPs or ForwardDestination IPs are not directly reachable.
- `upstream request failed` followed by `tls ... failed`: the IP is reachable, but TLS/SNI/certificate behavior failed.
- A rollback state remains after restore failure; run `restore` again after fixing the platform error.
- A stale state file points to an old process; `status` or `stop` should remove it.
