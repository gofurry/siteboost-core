# Usage

## Installation

Clone the repository for local development:

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

## Basic Usage

Print version information:

```bash
go run ./cmd/steam-accelerator --version
```

Start ProxyOnly, PAC, or System Proxy mode in the foreground:

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
go run ./cmd/steam-accelerator start --mode pac
go run ./cmd/steam-accelerator start --mode system
```

In another terminal:

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
go run ./cmd/steam-accelerator restore
```

For Windows Hosts mode with AppHost, use a fixed binary path instead of `go run`; see the Windows Hosts section below.

## Configuration

ProxyOnly can run with defaults, or with a YAML file:

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  non_target_behavior: "reject" # reject | direct
  allow_lan: false
  read_header_timeout: "10s"
  idle_timeout: "2m"
  dial_timeout: "30s"
  shutdown_timeout: "5s"

providers:
  enabled:
    - steam

dns_intercept:
  enabled: false
  strategy: "manual" # manual; system/external are planned but not implemented in v0.7.1
  listen_addr: "127.0.0.1:53"
  allow_lan: false
  map_ipv4: "127.0.0.1"
  map_ipv6: ""
  ttl: "30s"
  block_https_records: true

rules:
  custom_domains: []

pac:
  listen_addr: "127.0.0.1:26502"
  allow_lan: false

hosts:
  map_ip: "127.0.0.1"
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
  allow_lan: false
  path: "C:\\Windows\\System32\\drivers\\etc\\hosts"
  extra_domains: []

cert:
  # dir defaults to the user config directory and usually does not need
  # to be configured manually.
  # dir: "C:\\path\\to\\certs"
  # Hosts mode checks root CA trust on start. When auto_install is true,
  # start --mode hosts installs the local root CA if it is not trusted yet.
  auto_install: true
  # machine uses LocalMachine\Root and is the default Windows Hosts path.
  # Install AppHost once from Administrator PowerShell for normal PowerShell
  # system writes through the local named pipe. user uses CurrentUser\Root
  # as a fallback.
  store_scope: "machine"

resolver:
  mode: "system" # system | udp | tcp | doh
  servers: []
  prefer_ipv4: true
  prefer_ipv6: false
  disable_ipv6: false
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct" # direct | http | socks5
  address: ""
  username: ""
  password: ""
  # Enabled providers may contribute outbound profiles in Hosts + Direct mode.
  profiles: []

runtime:
  # state_path defaults to the user cache directory.
  # rollback_path defaults to the user cache directory.
  control_addr: "127.0.0.1:0"
  stop_timeout: "5s"

system_proxy:
  # macOS only. Empty means all enabled network services.
  services: []
```

Configuration precedence:

```text
built-in defaults < YAML config < CLI flags
```

CLI overrides:

```bash
go run ./cmd/steam-accelerator start \
  --config config.yaml \
  --listen 127.0.0.1:26501 \
  --pac-listen 127.0.0.1:26502 \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --dns-listen 127.0.0.1:15353 \
  --non-target reject
```

Resolver, upstream, macOS system service, and most Hosts/DNSIntercept options are YAML-only. The CLI keeps lifecycle and local listen overrides small.

Note: `providers.enabled: [steam]`, `resolver.mode: system`, and `upstream.type: direct` are the general defaults. When `mode: hosts` or `mode: dns` needs loop-safe direct outbound resolution, runtime resolution automatically switches to the built-in DoH defaults and appends outbound profiles from enabled providers. This avoids resolving outbound reverse-proxy connections back to the local takeover address. External HTTP/SOCKS5 upstream proxies remain optional enhancements, not a default acceleration prerequisite.

v0.7 removed the old Steam-specific config keys. Replace `proxy.non_steam_behavior` with `proxy.non_target_behavior`, replace `rules.enable_default_steam_rules` with `providers.enabled`, and remove `upstream.enable_default_steam_profiles`. Loading old keys returns a migration error.

## Common Examples

Use browser manual proxy settings:

```text
HTTP proxy: 127.0.0.1
Port: 26501
```

Default behavior enables only the Steam provider. Non-target traffic is rejected unless `non_target_behavior` is set to `direct`. `direct` means "allow forwarding"; the actual outbound path is selected by `upstream.type`.

Enable the GitHub skeleton provider explicitly:

```yaml
providers:
  enabled:
    - steam
    - github
```

GitHub is `experimental` in v0.7. It participates in matching and status output, but it does not define a default outbound profile and should not be described as real acceleration.

Use DNSIntercept manual mode on a high port:

```yaml
mode: dns

dns_intercept:
  strategy: "manual"
  listen_addr: "127.0.0.1:15353"
  map_ipv4: "127.0.0.1"
  map_ipv6: ""
  ttl: "30s"
  block_https_records: true

hosts:
  http_listen_addr: "127.0.0.1:28080"
  https_listen_addr: "127.0.0.1:28443"
```

Or from CLI:

```bash
go run ./cmd/steam-accelerator start --mode dns --dns-listen 127.0.0.1:15353 --hosts-http 127.0.0.1:28080 --hosts-https 127.0.0.1:28443
```

Manual DNSIntercept starts a local UDP/TCP DNS server and reverse proxy. It does not change system DNS, hosts, certificate trust, browser settings, or any persistent system state. Test it by pointing a DNS client at the listener:

```powershell
dig @127.0.0.1 -p 15353 steamcommunity.com A
dig @127.0.0.1 -p 15353 example.com A
```

If `dig` is not installed, use any DNS client that can target a custom server port.

`status` should show `dns_intercept: strategy=manual listen=... system_dns=false target=... forwarded=... cache_hits=... blocked=... errors=...`. Target `A`/`AAAA` records map to the local reverse proxy address. Target `HTTPS`/`SVCB` records return NODATA by default; set `block_https_records: false` to forward them explicitly. Other target record types are not forwarded. Non-target records are forwarded to the configured resolver or loop-safe DoH defaults.

Use DoH with direct outbound dialing:

```yaml
resolver:
  mode: "doh"
  # Empty servers use the built-in DoH default list; explicit URLs can override it.
  servers: []
  prefer_ipv4: true
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct"
```

Customize Steam outbound profiles:

```yaml
upstream:
  type: "direct"
  profiles:
    - match_domains:
        - "steamcommunity.com"
        - "*.steamcommunity.com"
      forward_host: "steamcommunity-a.akamaihd.net"
      tls_server_name: "steamcommunity-a.akamaihd.net"
    - match_domains:
        - "store.steampowered.com"
        - "checkout.steampowered.com"
        - "help.steampowered.com"
        - "login.steampowered.com"
        - "media.steampowered.com"
      forward_host: "cdn-a.akamaihd.net"
      tls_server_name: "cdn-a.akamaihd.net"
    - match_domains:
        - "community.steamstatic.com"
      forward_host: "community.steamstatic.com"
      tls_server_name: "community.steamstatic.com"
    - match_domains:
        - "steamcdn-a.akamaihd.net"
      forward_host: "steamcdn-a.akamaihd.net"
      tls_server_name: "steamcdn-a.akamaihd.net"
```

Profiles follow the same practical shape as the Steam++ acceleration data model: `match_domains` matches the original Steam host, `forward_host` is resolved and connected first, `tls_server_name` controls outbound TLS SNI, and the reverse proxy still sends the original Steam HTTP Host upstream. `candidate_ips` can pin fixed IP candidates; `ignore_tls_name_mismatch: true` should only be used when the certificate chain is trusted but the name does not match.

Use an HTTP upstream proxy:

```yaml
upstream:
  type: "http"
  address: "127.0.0.1:8080"
  username: "optional-user"
  password: "optional-password"
```

Use a SOCKS5 upstream proxy:

```yaml
upstream:
  type: "socks5"
  address: "127.0.0.1:1080"
  username: "optional-user"
  password: "optional-password"
```

UDP and TCP DNS modes require explicit servers. If a DNS server address omits the port, port `53` is used.

Start PAC mode:

```yaml
mode: pac

pac:
  listen_addr: "127.0.0.1:26502"
```

PAC mode starts the local proxy, starts `http://127.0.0.1:26502/proxy.pac`, and writes the system PAC URL on Windows or macOS. `stop` restores the previous system setting. If the process crashes, run:

```bash
go run ./cmd/steam-accelerator restore
```

Start System Proxy mode:

```yaml
mode: system
```

System mode writes system HTTP and HTTPS proxy settings to the local proxy address. Non-target traffic still follows `proxy.non_target_behavior`, which defaults to `reject`.

Start Windows Hosts mode:

```yaml
mode: hosts

hosts:
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
```

By default, `start --mode hosts` checks this project's root CA and installs it into the Windows `LocalMachine\Root` store if it is not trusted yet. Administrator PowerShell uses the silent direct path. The recommended daily flow is to build a fixed local binary, run `apphost install` once from an Administrator PowerShell, then run `start --mode hosts` from a normal PowerShell using the same binary path; restricted root CA, hosts, and restore writes go through the AppHost named pipe `\\.\pipe\SiteBoostCoreAppHost`. The core does not bypass UAC, enterprise policy, or arbitrary system-change safeguards. If the root CA is already installed, startup skips the install step.

Do not use `go run` for AppHost service installation. `go run` creates temporary executables, while AppHost validates that the pipe client process image matches the installed service executable path.

Set `cert.store_scope: "user"` only when you explicitly want `CurrentUser\Root`; that compatibility path may still show a Windows root-certificate confirmation.

```bash
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
./bin/steam-accelerator.exe apphost install
./bin/steam-accelerator.exe start --mode hosts
```

For an explicit/manual workflow, or if `cert.auto_install: false` is set, install the root CA first:

```bash
./bin/steam-accelerator.exe cert install
./bin/steam-accelerator.exe start --mode hosts
```

`cert install` checks the configured Windows Root store by certificate thumbprint first. If this project's root CA is already installed, it returns without running the install action again. From a normal PowerShell, writing the default `machine` store goes through the installed AppHost Service; `cert uninstall` is always an explicit user action.

Hosts mode writes a project-owned marker block into the Windows hosts file and maps exact provider domains to the local reverse proxy. `*.domain` wildcard rules are not written to hosts. The default Hosts + Direct loop uses built-in DoH for real outbound resolution and does not require an external upstream proxy. Startup checks the root CA, hosts read/write access, rollback directory writability, and reverse-proxy listeners; `status` shows the runtime `provider`, `resolver`, `resolver_servers`, `rule_set`, `upstream_profiles`, `system_change`, and `startup_probes`. When the normal PowerShell AppHost path succeeds, `system_change` detail currently includes `helper=elevated` for compatibility with the existing status field. `stop` or `restore` removes the project marker block, but does not uninstall the trusted Root CA; restoring hosts from a normal PowerShell also uses AppHost. To uninstall it, run:

```bash
./bin/steam-accelerator.exe cert uninstall
```

AppHost only accepts the default Windows hosts path plus rollback and certificate files under the default project runtime/config directories. It listens on the Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`, uses a DACL for local access control, enables remote-client rejection when the platform supports it, checks the pipe client PID against the request parent PID, verifies the client process image matches the installed AppHost executable, and still enforces the system-change command whitelist. If YAML config points `hosts.path`, `runtime.rollback_path`, or `cert.dir` to custom locations and the process is not elevated, AppHost rejects the request; use an Administrator PowerShell for those advanced paths or a future controlled desktop integration.

If the browser or Steam embedded browser still shows `upstream request failed`, the response body and logs include an outbound diagnostic summary. It should indicate whether the failure came from DoH resolution, TCP connect attempts to candidate IPs, or TLS handshake. Use that message to decide whether the next issue is DNS/DoH, ForwardDestination reachability, certificate/SNI behavior, or missing rule/profile coverage.

In Hosts + Direct mode, `start` and `status` include a non-fatal startup probe summary. `startup_probes: ok=6 failed=0` means the default Steam probe targets passed DoH resolution, TCP 443, TLS, and a lightweight HTTPS `HEAD /` check through the active outbound profile path. Failed rows are printed as `startup_probe_failed` with `host`, `target`, `stage`, and a trimmed error. The exact host list, wildcard gaps, default probe targets, and manual smoke table are maintained in the [Steam compatibility matrix](steam-compatibility.md).

Windows `curl.exe` uses Schannel and checks certificate revocation by default. Because this project dynamically issues local site certificates without public OCSP / CRL endpoints, command-line checks may report `CRYPT_E_NO_REVOCATION_CHECK`. Use `--ssl-no-revoke` to skip revocation checking while keeping certificate-chain and hostname validation, which is a better local acceleration check than `-k/--insecure`:

```bash
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcommunity.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://store.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://community.steamstatic.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://media.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcdn-a.akamaihd.net/
```

For high-port smoke testing:

```bash
./bin/steam-accelerator.exe start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

Use an isolated state file for testing:

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

macOS/Linux Hosts and certificate-store setup remain explicitly unsupported in v0.7.1-dev.
