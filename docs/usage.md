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

# cert.dir defaults to the user config directory and usually does not need
# to be configured manually.

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

Install this project's root CA explicitly before first use:

```bash
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

Hosts mode writes a project-owned marker block into the Windows hosts file and maps exact Steam domains to the local reverse proxy. `*.domain` wildcard rules are not written to hosts. `stop` or `restore` removes the project marker block, but does not uninstall the explicitly installed root CA. To uninstall it, run:

```bash
go run ./cmd/steam-accelerator cert uninstall
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

macOS/Linux Hosts and certificate-store setup are explicitly unsupported in v0.4.0.
