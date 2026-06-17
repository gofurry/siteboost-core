# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

Language: [中文文档](./README_zh.md)

## Introduction

steam-accelerator-core is a Go-based Steam local acceleration core. It is designed to provide reusable network acceleration primitives for local desktop tools, sidecars, and future SteamScope or steam-go integrations.

The current repository is in the scaffold stage. Runtime acceleration features are planned but not implemented yet.

This project references the network acceleration architecture ideas of Watt Toolkit / SteamTools, including local reverse proxy, PAC, system proxy, hosts mode, certificate handling, DNS, and outbound proxy modes. It does not include, copy, translate, or port SteamTools source code.

## Features

Planned capabilities:

- Local HTTP proxy and HTTPS CONNECT proxy.
- Steam domain rules and safe host matching.
- PAC generation and local PAC server.
- System proxy setup and rollback.
- Hosts patching with transaction-style restore.
- Local root CA and dynamic site certificate support.
- HTTPS reverse proxy for hosts mode.
- DNS, DoH, caching, and IPv4 or IPv6 policy.
- Direct, HTTP proxy, and SOCKS5 upstream dialing.
- Start, stop, restore, and status lifecycle.

Current scaffold:

- Go module at `github.com/gofurry/go-steam-core`.
- Minimal CLI entry at `cmd/steam-accelerator`.
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

Library installation will become useful after the v0.1.0 API appears:

```bash
go get github.com/gofurry/go-steam-core
```

## Quick Start

Run the scaffold CLI:

```bash
go run ./cmd/steam-accelerator --version
```

Run the basic example:

```bash
go run ./examples/basic
```

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

Feature examples for ProxyOnly, PAC, System Proxy, and Hosts mode will be added with the corresponding milestones.

## Project Structure

```text
.
├── cmd/steam-accelerator/     # CLI entry, currently a scaffold
├── docs/                      # English maintenance docs
├── docs/zh/                   # Chinese maintenance docs
├── examples/basic/            # Minimal runnable example
├── internal/                  # Planned private implementation packages
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

The project is early. Keep changes small, testable, and aligned with the roadmap. Do not copy SteamTools implementation code; use independent Go implementations and document any external dependency decisions.

## License

GPL-3.0. See [LICENSE](./LICENSE).
