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

Start ProxyOnly mode in the foreground:

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
```

In another terminal:

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
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

runtime:
  # state_path defaults to the user cache directory.
  control_addr: "127.0.0.1:0"
  stop_timeout: "5s"
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
  --non-steam reject
```

Resolver and upstream options are YAML-only in v0.2.0. The CLI keeps only simple lifecycle and ProxyOnly overrides.

## Common Examples

Use browser manual proxy settings:

```text
HTTP proxy: 127.0.0.1
Port: 26501
```

Default behavior only allows Steam rule domains. Non-Steam traffic is rejected unless `non_steam_behavior` is set to `direct`. In v0.2.0, `direct` means "allow forwarding"; the actual outbound path is selected by `upstream.type`.

Use DoH with direct outbound dialing:

```yaml
resolver:
  mode: "doh"
  servers:
    - "https://dns.example/dns-query"
  prefer_ipv4: true
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct"
```

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

Use an isolated state file for testing:

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

Future commands for PAC, System Proxy, Hosts, certificate management, and restore are planned in later milestones.
