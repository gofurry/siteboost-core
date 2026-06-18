# Changelog

All notable changes to this project will be documented in this file.

The project has not published a runtime release yet.

## Unreleased

### Added

- Repository scaffold for `steam-accelerator-core`.
- Go module `github.com/gofurry/go-steam-core`.
- CLI at `cmd/steam-accelerator` with `start`, `status`, `stop`, and `restore`.
- YAML configuration with safe loopback defaults.
- Steam domain rules matcher with exact, wildcard, port stripping, lowercase, and IDNA handling.
- HTTP proxy and HTTPS CONNECT tunnel for Steam rule domains.
- Configurable non-Steam behavior: `reject` by default, or `direct`.
- Configurable resolver modes: system DNS, UDP DNS, TCP DNS, and DoH.
- DNS cache, resolver timeout, server fallback, and IPv4/IPv6 selection policy.
- Direct upstream dialing through the configured resolver.
- HTTP CONNECT upstream and SOCKS5 upstream dialing with optional authentication.
- PAC generation and local PAC server.
- Windows and macOS PAC/System Proxy setup with rollback state.
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

- Version metadata now reports `v0.3.0-dev`.
- `non_steam_behavior: direct` still means non-Steam traffic is allowed, but the outbound path is now selected by `upstream.type`.

### Notes

- Runtime, resolver, upstream, PAC, and system proxy implementations remain internal; no stable public Go integration API is exposed yet.
- `github.com/miekg/dns` is used for DNS wire message handling.
- SteamTools is used as an architecture reference only; no source code is copied or ported.
