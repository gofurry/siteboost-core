# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

Language: [中文文档](./README_zh.md)

## Introduction

steam-accelerator-core is a Go-based Steam local acceleration core. It is designed to provide reusable network acceleration primitives for local desktop tools, sidecars, and future SteamScope or steam-go integrations.

The current v0.3.0 development line includes a runnable local acceleration core. It supports ProxyOnly, PAC, and System Proxy modes, Steam domain matching, YAML configuration, configurable DNS resolution with cache and IP policy, direct/HTTP/SOCKS5 upstream dialing, local rollback state, a foreground CLI lifecycle, a local state file, and a token-protected loopback control interface.

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
- Foreground `start`, `status`, `stop`, and `restore` CLI lifecycle.
- Local runtime state file and token-protected loopback control API.

Planned capabilities:

- Hosts patching with transaction-style restore.
- Local root CA and dynamic site certificate support.
- HTTPS reverse proxy for hosts mode.
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

Resolver, upstream, PAC, and system proxy options are configured through YAML. The defaults remain `resolver.mode: system` and `upstream.type: direct`.

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

Feature examples for Hosts mode will be added with the corresponding milestone.

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
5. `v0.5.0`: stability, safety, restore, and cross-platform hardening.
6. `v1.0.0`: stable API and integration release.

See [ROADMAP.md](./ROADMAP.md) for the canonical Chinese plan.

## Contributing

The project is early. Runtime implementation is intentionally internal until the integration API stabilizes. Keep changes small, testable, and aligned with the roadmap. Do not copy SteamTools implementation code; use independent Go implementations and document any external dependency decisions.

## License

GPL-3.0. See [LICENSE](./LICENSE).
