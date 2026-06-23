# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

Language: [中文文档](./README_zh.md)

## Introduction

steam-accelerator-core is an experimental Go-based local site acceleration core. It is designed to validate reusable network acceleration primitives for local desktop tools, sidecars, and the future standalone Go library [gofurry/web-boost](https://github.com/gofurry/web-boost).

The current v0.7.3-dev line includes a provider registry, DNSIntercept manual mode, explicit Windows system DNS takeover with rollback, and an opt-in Page Enhance response transform pipeline. Steam is the default stable provider. GitHub is available only as an explicit experimental skeleton provider for architecture validation, not as a real acceleration promise. The runtime supports ProxyOnly, PAC, System Proxy, Windows-first Hosts reverse proxy, manual DNSIntercept, Windows DNSIntercept system mode, generic provider matching, YAML configuration, configurable DNS resolution with cache and IP policy, direct/HTTP/SOCKS5 upstream dialing, local rollback state, foreground CLI lifecycle, local state file, token-protected runtime control interface, and transparent response transforms for local reverse-proxy flows. Hosts + Direct mode uses built-in DoH defaults to avoid local hosts loopback and applies the enabled providers' outbound profiles. DNSIntercept manual mode starts a local UDP/TCP DNS server but does not modify system DNS, hosts, certificate trust, or any persistent system setting. DNSIntercept system mode is Windows-only, explicit, interface-scoped, and restores DHCP/static DNS through rollback. Page Enhance is disabled by default; when enabled, all header edits, HTML injection, local assets, and replacements come from explicit configuration or registered transformers and can be disabled by removing `page_enhance`. Windows system writes use a Steam++-style AppHost Service path: install `SiteBoostCoreAppHost` once with administrator authorization, then normal PowerShell runs can request narrow root CA, hosts, system DNS, and restore actions through the local named pipe `\\.\pipe\SiteBoostCoreAppHost`. HTTP/SOCKS5 upstreams are optional enhancements, not the default acceleration prerequisite.

This project references the network acceleration architecture ideas of Watt Toolkit / SteamTools, including local reverse proxy, PAC, system proxy, hosts mode, certificate handling, DNS, and outbound proxy modes. It does not include, copy, translate, or port SteamTools source code.

## Features

Current capabilities:

- Local HTTP proxy and HTTPS CONNECT proxy.
- Provider registry with Steam stable provider and GitHub experimental skeleton provider.
- Generic provider domain rules and safe host matching.
- YAML configuration with safe defaults.
- System DNS, UDP DNS, TCP DNS, DoH, DNS cache, and IPv4/IPv6 policy.
- Direct, HTTP CONNECT upstream, and SOCKS5 upstream dialing.
- PAC generation and local PAC server.
- Windows and macOS system PAC/manual proxy setup with rollback.
- Windows hosts marker block setup and restore.
- Local root CA generation plus Windows machine/user certificate-store install/uninstall.
- Local HTTP/HTTPS reverse proxy with dynamic site certificates for Hosts mode.
- Windows Hosts one-click flow with root CA auto-install, AppHost named pipe IPC, hosts preflight, rollback, and system-change status output.
- Hosts + Direct default DoH outbound resolution, hosts preflight, and resolver status output.
- Hosts + Direct provider outbound profiles with ForwardDestination, TLS SNI, candidate IP, and original-domain fallback support.
- Hosts + Direct startup probes for DoH resolution, TCP 443, TLS handshake, and lightweight HTTPS smoke status.
- DNSIntercept manual mode with local UDP/TCP DNS server, target-domain mapping, non-target forwarding, response cache, status counters, and no automatic system DNS takeover.
- Windows DNSIntercept system mode with explicit interface selection, AppHost allowlisted system DNS writes, rollback, and restore.
- Opt-in Page Enhance pipeline for reverse-proxy responses: provider/host/path/content-type/status matching, header set/remove, HTML injection, local asset serving, simple replacement, custom transformer hooks, and observable apply/skip/error counters.
- Outbound failure diagnostics with candidate IPs, TCP / TLS stages, and trimmed 502 error summaries.
- Foreground `start`, `status`, `stop`, and `restore` CLI lifecycle.
- Local runtime state file and token-protected loopback control API.

Planned capabilities:

- macOS/Linux Hosts and certificate-store support.
- VPN/TUN adapters and deeper traffic capture modes.
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

Windows Hosts mode checks and installs the local root CA inside the start flow by default. Administrator PowerShell uses the silent direct path. For the Steam++-style daily path, build a fixed local binary first, run `apphost install` once from an Administrator PowerShell, and keep using that same binary path; later normal PowerShell runs request restricted root CA, hosts, and restore actions through the AppHost named pipe:

```bash
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
./bin/steam-accelerator.exe apphost install
./bin/steam-accelerator.exe start --mode hosts
```

From another terminal:

```bash
./bin/steam-accelerator.exe status
./bin/steam-accelerator.exe stop
./bin/steam-accelerator.exe restore
```

DNSIntercept manual mode should be tested on a high port first. It does not change system DNS; point a DNS client at the printed `dns_intercept` listener manually:

```bash
./bin/steam-accelerator.exe start --mode dns --dns-listen 127.0.0.1:15353 --hosts-http 127.0.0.1:28080 --hosts-https 127.0.0.1:28443
```

Windows DNSIntercept system mode requires YAML with `dns_intercept.strategy: system`, `listen_addr: "127.0.0.1:53"`, and explicit `dns_intercept.interfaces`. It changes the selected adapter DNS and should be tested with the documented smoke flow.

Run the basic module example:

```bash
go run ./examples/basic
```

Resolver, upstream, provider, PAC, system proxy, Hosts, and DNSIntercept options are configured through YAML. The general defaults remain `providers.enabled: [steam]`, `resolver.mode: system`, and `upstream.type: direct`; `start --mode hosts` and `start --mode dns` use loop-safe resolver behavior when needed, and `status` shows `provider:`, `resolver:`, `rule_set:`, and `dns_intercept:` when active.

## Documentation

- [Chinese roadmap](./ROADMAP.md)
- [English roadmap](./docs/roadmap.md)
- [Usage](./docs/usage.md)
- [Smoke test](./docs/smoke-test.md)
- [web-boost library extraction plan](./docs/web-boost-library-plan.md)
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
8. `v0.6.1`: Windows certificate and one-click flow packaging.
9. `v0.6.2`: Windows machine-scope certificate default.
10. `v0.6.3`: Windows privileged helper foundation.
11. `v0.6.4`: Windows AppHost Service and named pipe IPC.
12. `v0.7.0`: provider registry, Steam stable provider, and GitHub experimental skeleton.
13. `v0.7.1`: DNSIntercept manual local DNS server.
14. `v0.7.2`: explicit Windows system DNS takeover and restore.
15. `v0.7.3`: page enhancement transform pipeline.
16. `v0.8.0`: public Go library extraction preparation.
17. `v1.0.0`: stable API and integration release.

See [ROADMAP.md](./ROADMAP.md) for the canonical Chinese plan.

## Contributing

The project is early. Runtime implementation is intentionally internal until the integration API stabilizes. Keep changes small, testable, and aligned with the roadmap. Do not copy SteamTools implementation code; use independent Go implementations and document any external dependency decisions.

## License

GPL-3.0. See [LICENSE](./LICENSE).
