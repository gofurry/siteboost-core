# Changelog

All notable changes to this project will be documented in this file.

The project has not published a runtime release yet.

## Unreleased

### Added

- Repository scaffold for `steam-accelerator-core`.
- Go module `github.com/gofurry/go-steam-core`.
- ProxyOnly CLI at `cmd/steam-accelerator` with `start`, `status`, and `stop`.
- YAML configuration with safe loopback defaults.
- Steam domain rules matcher with exact, wildcard, port stripping, lowercase, and IDNA handling.
- HTTP proxy and HTTPS CONNECT tunnel for Steam rule domains.
- Configurable non-Steam behavior: `reject` by default, or `direct`.
- Direct upstream dialing through the system network stack.
- Engine lifecycle with status and active connection count.
- Local runtime state file and token-protected loopback control server.
- Unit tests for config, rules, proxy, engine, and runtime control.
- Basic runnable example.
- Bilingual README files.
- Chinese canonical roadmap in `ROADMAP.md`.
- English and Chinese maintenance docs.
- GitHub Actions workflow for `gofmt`, `go vet`, and `go test`.

### Notes

- v0.1.0 runtime remains internal; no stable public Go integration API is exposed yet.
- SteamTools is used as an architecture reference only; no source code is copied or ported.
