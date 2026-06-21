# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

Language: [中文文档](./README_zh.md)

## Introduction

steam-accelerator-core is a Go-based Steam local acceleration core. It is designed to provide reusable network acceleration primitives for local desktop tools, sidecars, and future SteamScope or steam-go integrations.

The current v0.6.0 development line includes a runnable local acceleration core. It supports ProxyOnly, PAC, System Proxy, and Windows-first Hosts reverse proxy modes, Steam domain matching, YAML configuration, configurable DNS resolution with cache and IP policy, direct/HTTP/SOCKS5 upstream dialing, local rollback state, a foreground CLI lifecycle, a local state file, and a token-protected loopback control interface. Hosts + Direct mode uses built-in DoH defaults for real Steam IP lookup to avoid local hosts loopback, and now includes a default Steam outbound profile: community domains prefer `steamcommunity-a.akamaihd.net`, while store / checkout / help / login domains prefer `cdn-a.akamaihd.net`; the original Steam HTTP Host is preserved and TLS SNI follows the profile target. HTTP/SOCKS5 upstreams are optional enhancements, not the default acceleration prerequisite.

This project references the network acceleration architecture ideas of Watt Toolkit / SteamTools, including local reverse proxy, PAC, system proxy, hosts mode, certificate handling, DNS, and outbound proxy modes. It does not include, copy, translate, or port SteamTools source code.

## Features

Current capabilities:

- Local HTTP proxy and HTTPS CONNECT proxy.
- Steam domain rules and safe host matching.
- YAML configuration with safe defaults.
- System DNS, UDP DNS, TCP DNS, DoH, DNS cache, and IPv4/IPv6 policy.
- Direct, HTTP CONNECT upstream, and SOCKS5 upstream dialing.
- PAC generation and local PAC server.
- Windows and macOS system PAC/manual proxy setup with rollback.
- Windows hosts marker block setup and restore.
- Local root CA generation plus Windows current-user install/uninstall.
- Local HTTP/HTTPS reverse proxy with dynamic site certificates for Hosts mode.
- Hosts + Direct default DoH outbound resolution, hosts preflight, and resolver status output.
- Hosts + Direct default Steam outbound profile with ForwardDestination, TLS SNI, candidate IP, and original-domain fallback support.
- Outbound failure diagnostics with candidate IPs, TCP / TLS stages, and trimmed 502 error summaries.
- Foreground `start`, `status`, `stop`, and `restore` CLI lifecycle.
- Local runtime state file and token-protected loopback control API.

Planned capabilities:

- macOS/Linux Hosts and certificate-store support.
- DNSIntercept, VPN/TUN, and deeper traffic capture modes.
- Restore lifecycle for system-modifying modes.

Current repository foundation:

- Go module at `github.com/gofurry/go-steam-core`.
- CLI entry at `cmd/steam-accelerator`.
- Runnable basic example.
- Bilingual README and documentation layout.
- GitHub Actions for `gofmt`, `go vet`, and `go test`.
- Chinese canonical roadmap.

## Installation

For local development:

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

The CLI can be run from source. A stable public Go library API is not exposed yet:

```bash
go get github.com/gofurry/go-steam-core
```

## Quick Start

Print version information:

```bash
go run ./cmd/steam-accelerator --version
```

Start ProxyOnly mode in the foreground:

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
```

Start PAC or System Proxy mode:

```bash
go run ./cmd/steam-accelerator start --mode pac
go run ./cmd/steam-accelerator start --mode system
```

Windows Hosts mode requires an explicit local root CA install first:

```bash
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

From another terminal:

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
go run ./cmd/steam-accelerator restore
```

Run the basic module example:

```bash
go run ./examples/basic
```

Resolver, upstream, PAC, system proxy, and Hosts options are configured through YAML. The general defaults remain `resolver.mode: system` and `upstream.type: direct`; `start --mode hosts` automatically uses built-in DoH plus the default Steam outbound profile with Direct outbound dialing, and `status` shows `resolver: doh`.

## Documentation

- [Chinese roadmap](./ROADMAP.md)
- [English roadmap](./docs/roadmap.md)
- [Usage](./docs/usage.md)
- [Smoke test](./docs/smoke-test.md)
- [Hotfix workflow](./docs/hotfix.md)
- [Todo](./docs/todo.md)
- [Security policy](./SECURITY.md)
- [SteamTools reference boundary](./docs/zh/steamtools-reference.md)

## Examples

Examples live under `examples/`.

- `examples/basic`: verifies that the module can be imported and executed.

Hosts mode is currently Windows-first. For high-port smoke tests, use `--hosts-http 127.0.0.1:28080 --hosts-https 127.0.0.1:28443`.

## Project Structure

```text
.
├── cmd/steam-accelerator/     # CLI entry
├── docs/                      # English maintenance docs
├── docs/zh/                   # Chinese maintenance docs
├── examples/basic/            # Minimal runnable example
├── internal/                  # Private runtime implementation packages
├── ROADMAP.md                 # Canonical Chinese roadmap
├── README.md
├── README_zh.md
└── go.mod
```

## Development

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
```

The same format, vet, and test checks run in GitHub Actions on push and pull request.

## Roadmap

The implementation order is foundation-first:

1. `v0.1.0`: ProxyOnly plus CONNECT core.
2. `v0.2.0`: Resolver, DoH, and upstream outbound support.
3. `v0.3.0`: PAC and System Proxy.
4. `v0.4.0`: Hosts plus HTTPS reverse proxy.
5. `v0.5.0`: one-click Hosts + DoH default loop.
6. `v0.5.1`: outbound failure diagnostics patch.
7. `v0.6.0`: real Steam outbound profiles, smoke tests, and rule coverage.
8. `v1.0.0`: stable API and integration release.

See [ROADMAP.md](./ROADMAP.md) for the canonical Chinese plan.

## Contributing

The project is early. Runtime implementation is intentionally internal until the integration API stabilizes. Keep changes small, testable, and aligned with the roadmap. Do not copy SteamTools implementation code; use independent Go implementations and document any external dependency decisions.

## License

GPL-3.0. See [LICENSE](./LICENSE).
