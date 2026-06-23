# Changelog

All notable changes to this project will be documented in this file.

The project has not published a runtime release yet.

## Unreleased

### Added

- Repository scaffold for `steam-accelerator-core`.
- Go module `github.com/gofurry/go-steam-core`.
- CLI at `cmd/steam-accelerator` with `start`, `status`, `stop`, and `restore`.
- YAML configuration with safe loopback defaults.
- Provider registry with Steam as the default stable provider and GitHub as an explicit experimental skeleton provider.
- DNSIntercept manual mode with a local UDP/TCP DNS server, target-domain mapping, non-target forwarding, response cache, timeout handling, listen-conflict detection, and status counters.
- Generic domain rules matcher with exact, wildcard, port stripping, lowercase, and IDNA handling.
- HTTP proxy and HTTPS CONNECT tunnel for enabled provider rule domains.
- Configurable non-target behavior: `reject` by default, or `direct`.
- Configurable resolver modes: system DNS, UDP DNS, TCP DNS, and DoH.
- DNS cache, resolver timeout, server fallback, and IPv4/IPv6 selection policy.
- Direct upstream dialing through the configured resolver.
- HTTP CONNECT upstream and SOCKS5 upstream dialing with optional authentication.
- PAC generation and local PAC server.
- Windows and macOS PAC/System Proxy setup with rollback state.
- Windows Hosts mode with project-owned hosts marker block and restore support.
- Local root CA generation, Windows current-user install/uninstall, and dynamic site certificates.
- Local HTTP/HTTPS reverse proxy for Hosts mode with Host/SNI preservation and WebSocket upgrade support.
- Engine lifecycle with status and active connection count.
- Local runtime state file and token-protected loopback control server.
- Unit tests for config, rules, resolver, upstream, proxy, engine, and runtime control.
- Proxy integration tests for direct resolver, HTTP upstream, and SOCKS5 upstream paths.
- Basic runnable example.
- Bilingual README files.
- Chinese canonical roadmap in `ROADMAP.md`.
- English and Chinese maintenance docs.
- GitHub Actions workflow for `gofmt`, `go vet`, and `go test`.

### Changed

- Version metadata now reports `v0.7.1-dev`.
- Default configuration uses `providers.enabled: [steam]` and `proxy.non_target_behavior: reject`.
- Steam default rules, outbound profiles, and startup probes now live behind the Steam provider instead of the generic rules/upstream packages.
- `start --non-target reject|direct` replaces the old Steam-specific CLI flag.
- `start --mode dns --dns-listen ...` starts DNSIntercept manual mode without changing system DNS.

### Removed

- `proxy.non_steam_behavior`, `rules.enable_default_steam_rules`, and `upstream.enable_default_steam_profiles` now fail with migration guidance instead of being silently accepted.
- `start --non-steam` now fails with guidance to use `--non-target`.

### Notes

- Runtime, resolver, upstream, PAC, and system proxy implementations remain internal; no stable public Go integration API is exposed yet.
- `github.com/miekg/dns` is used for DNS wire message handling.
- Hosts mode is Windows-first in v0.7.1-dev. macOS/Linux Hosts and certificate-store setup return unsupported.
- Hosts files cannot express wildcard rules, so Hosts mode writes exact domains only; DNSIntercept manual mode can cover wildcard rules when a DNS client is explicitly pointed at the local listener.
- GitHub is a skeleton provider for architecture validation only and does not promise real acceleration.
- SteamTools is used as an architecture reference only; no source code is copied or ported.
