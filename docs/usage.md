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

Start ProxyOnly, PAC, System Proxy, or Hosts mode in the foreground:

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
go run ./cmd/steam-accelerator start --mode pac
go run ./cmd/steam-accelerator start --mode system
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

In another terminal:

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
go run ./cmd/steam-accelerator restore
```

## Configuration

ProxyOnly can run with defaults, or with a YAML file:

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  non_steam_behavior: "reject" # reject | direct
  allow_lan: false
  read_header_timeout: "10s"
  idle_timeout: "2m"
  dial_timeout: "30s"
  shutdown_timeout: "5s"

rules:
  enable_default_steam_rules: true
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
  # Hosts + Direct mode enables the built-in Steam outbound profile by default.
  enable_default_steam_profiles: true
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
  --non-steam reject
```

Resolver, upstream, macOS system service, and most Hosts options are YAML-only. The CLI keeps lifecycle and local listen overrides small.

Note: `resolver.mode: system` is the general default. When `mode: hosts` and `upstream.type: direct` are active, runtime resolution automatically switches to the built-in DoH defaults for real Steam IP lookup and enables the default Steam outbound profile. This avoids resolving outbound reverse-proxy connections back to `127.0.0.1` after the hosts marker block is written. External HTTP/SOCKS5 upstream proxies remain optional enhancements, not a default acceleration prerequisite.

## Common Examples

Use browser manual proxy settings:

```text
HTTP proxy: 127.0.0.1
Port: 26501
```

Default behavior only allows Steam rule domains. Non-Steam traffic is rejected unless `non_steam_behavior` is set to `direct`. `direct` means "allow forwarding"; the actual outbound path is selected by `upstream.type`.

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
  enable_default_steam_profiles: true
```

Customize Steam outbound profiles:

```yaml
upstream:
  type: "direct"
  enable_default_steam_profiles: true
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

System mode writes system HTTP and HTTPS proxy settings to the local proxy address. Non-Steam traffic still follows `proxy.non_steam_behavior`, which defaults to `reject`.

Start Windows Hosts mode:

```yaml
mode: hosts

hosts:
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
```

By default, `start --mode hosts` checks this project's root CA and installs it into the current-user Root store if it is not trusted yet. The normal Windows API path avoids the old `certutil` shell-out flow, but the core does not bypass UAC, enterprise policy, or arbitrary system-change safeguards. If the root CA is already installed, startup skips the install step.

```bash
go run ./cmd/steam-accelerator start --mode hosts
```

For an explicit/manual workflow, or if `cert.auto_install: false` is set, install the root CA first:

```bash
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

`cert install` checks the current-user Root store by certificate thumbprint first. If this project's root CA is already installed, it returns without running the install action again.

Hosts mode writes a project-owned marker block into the Windows hosts file and maps exact Steam domains to the local reverse proxy. `*.domain` wildcard rules are not written to hosts. The default Hosts + Direct loop uses built-in DoH for real outbound resolution and does not require an external upstream proxy. Startup checks the root CA, hosts read/write access, rollback directory writability, and reverse-proxy listeners; `status` shows the runtime `resolver`, `resolver_servers`, `rule_set`, `upstream_profiles`, `system_change`, and `startup_probes`. `stop` or `restore` removes the project marker block, but does not uninstall the trusted Root CA. To uninstall it, run:

```bash
go run ./cmd/steam-accelerator cert uninstall
```

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
go run ./cmd/steam-accelerator start --mode hosts \
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

macOS/Linux Hosts and certificate-store setup remain explicitly unsupported in v0.6.0.
