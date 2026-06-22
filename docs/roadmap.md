# Roadmap

The canonical roadmap is maintained in [../ROADMAP.md](../ROADMAP.md). This file is an English summary for contributors who do not read the Chinese roadmap.

## Current Position

The repository has moved from a Steam-only local acceleration core toward `gofurry/siteboost-core`, an experimental local site acceleration core inspired by the architecture ideas behind Steam++ / Watt Toolkit.

This repository is not intended to become the final public Go library itself. It is the experimental proving ground. After the core behavior and architecture are validated, a separate repository should be created for the formal reusable Go library, reusing or porting the proven pieces from this repository.

Current facts:

- The remote repository is `gofurry/siteboost-core`.
- The Go module is still `github.com/gofurry/go-steam-core`.
- The CLI is still `steam-accelerator`.
- `version.go` reports `v0.6.4-dev`.
- The main branch contains Windows AppHost Service and named pipe IPC work.

Steam is currently the only real provider. The Windows Hosts + DoH + HTTPS Reverse Proxy path has been manually validated in a China-network environment for Steam store, community, help, chat/login, static assets, and common CDN hosts. GitHub is not implemented as a real acceleration provider yet; it should first appear as a skeleton provider for architecture validation.

## Direction

The long-term goal is to produce enough validated implementation, smoke records, and architecture boundaries here so a future dedicated Go library repository can be created with less guesswork.

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
- named pipe IPC at `\\.\pipe\SiteBoostCoreAppHost`
- pipe DACL, local-client-only pipe mode, pipe client PID checks, and client executable path checks
- normal PowerShell `start --mode hosts`
- normal PowerShell `stop` / `restore`
- clear diagnostics when the service is missing, stopped, or unhealthy

### v0.7.0 - Provider Architecture and Generic Site Skeleton

Refactor Steam-specific rules, outbound profiles, probes, and smoke targets into a built-in Steam provider. Add a GitHub skeleton provider marked experimental. Core reverse proxy, resolver, and upstream packages should depend on generic provider data instead of Steam-specific assumptions.

### v0.8.0 - Library Extraction Readiness

Prepare the future library API draft and migration inventory for:

- `Config`
- `Engine`
- `Provider`
- `Mode`
- `Status`
- `Start`
- `Stop`
- `Restore`

The current CLI should be decoupled from core assembly enough that future extraction does not carry CLI-specific details into the new library.

### v0.9.0 - Reliability, Recovery, and Release Engineering

Harden AppHost IPC, add diagnostics, version rollback state, expand CI, and prepare installer / upgrade / uninstall / signing documentation.

### v1.0.0-alpha.1 - Experimental Architecture Freeze Candidate

Freeze this repository's migration-ready architecture boundary, provider schema, configuration migration path, and security boundary documentation.

### v1.0.0 - Experimental Validation Baseline

Ship a stable validation baseline for this experimental repository. Steam remains the stable provider. Windows Hosts + DoH + AppHost is the stable one-click path. The release should include migration notes for the future dedicated Go library repository; it is not the final library release.

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
- Do not describe this repository as the final Go library repository.
- Do not copy, translate, or port SteamTools source code.
- Do not present AppHost as UAC bypass; it is a controlled privileged service installed with administrator authorization.
