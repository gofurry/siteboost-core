# Roadmap

The canonical roadmap is maintained in [../ROADMAP.md](../ROADMAP.md). This file is an English summary for contributors who do not read the Chinese roadmap.

## Current Position

The repository has moved from a Steam-only local acceleration core toward `gofurry/siteboost-core`, a general local site acceleration core inspired by the architecture ideas behind Steam++ / Watt Toolkit.

Current facts:

- The remote repository is `gofurry/siteboost-core`.
- The Go module is still `github.com/gofurry/go-steam-core`.
- The CLI is still `steam-accelerator`.
- `version.go` still reports `v0.6.3`.
- The main branch already contains `v0.6.4-dev` level Windows AppHost Service work.

Steam is currently the only real provider. The Windows Hosts + DoH + HTTPS Reverse Proxy path has been manually validated in a China-network environment for Steam store, community, help, chat/login, static assets, and common CDN hosts. GitHub is not implemented as a real acceleration provider yet; it should first appear as a skeleton provider for architecture validation.

## Direction

The long-term goal is a maintainable Go library for local site acceleration. CLI and desktop shells should become thin callers of the library.

The core should be split around these concepts:

- provider and rule packs
- resolver and DoH
- upstream and outbound profiles
- takeover modes: ProxyOnly, PAC, System Proxy, Hosts, later DNSIntercept or TUN
- local reverse proxy
- root CA and dynamic certificates
- privilege boundary and restore
- diagnostics and smoke tests

HTTP and SOCKS5 upstreams are optional enhancements. They are not the default acceleration prerequisite.

## Version Plan

### v0.6.4 - Windows AppHost Service Validation

**Status:** Code completed, real-machine validation pending.

Validate the Steam++-style privilege boundary:

- one administrator `apphost install`
- automatic service startup after reboot
- normal PowerShell `start --mode hosts`
- normal PowerShell `stop` / `restore`
- clear diagnostics when the service is missing, stopped, or unhealthy

### v0.7.0 - Provider Architecture and Generic Site Skeleton

Refactor Steam-specific rules, outbound profiles, probes, and smoke targets into a built-in Steam provider. Add a GitHub skeleton provider marked experimental. Core reverse proxy, resolver, and upstream packages should depend on generic provider data instead of Steam-specific assumptions.

### v0.8.0 - Public Go Library Candidate

Introduce the first public API candidate for:

- `Config`
- `Engine`
- `Provider`
- `Mode`
- `Status`
- `Start`
- `Stop`
- `Restore`

The CLI should become a caller of the public API instead of assembling internal packages directly.

### v0.9.0 - Reliability, Recovery, and Release Engineering

Harden AppHost IPC, add diagnostics, version rollback state, expand CI, and prepare installer / upgrade / uninstall / signing documentation.

### v1.0.0-alpha.1 - API and Architecture Freeze Candidate

Freeze the v1 public API candidate, provider schema, configuration migration path, and security boundary documentation.

### v1.0.0 - Stable Release

Ship the first stable general site acceleration core. Steam remains the stable provider. Windows Hosts + DoH + AppHost is the stable one-click path. Go library APIs should remain compatible within v1.

### v1.1+

Possible post-v1 work:

- real GitHub provider validation
- DNSIntercept
- VPN / TUN
- JS injection or page enhancement, disabled by default
- macOS / Linux Hosts, certificate, and privilege loops

## Non-goals Before v1

- Do not promise real GitHub acceleration before provider validation.
- Do not treat an upstream proxy as required for the default loop.
- Do not copy, translate, or port SteamTools source code.
- Do not present AppHost as UAC bypass; it is a controlled privileged service installed with administrator authorization.
