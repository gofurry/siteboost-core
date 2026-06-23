# Roadmap

The canonical roadmap is maintained in [../ROADMAP.md](../ROADMAP.md). This file is an English summary for contributors who do not read the Chinese roadmap.

## Current Position

The repository has moved from a Steam-only local acceleration core toward `gofurry/siteboost-core`, an experimental local site acceleration core inspired by the architecture ideas behind Steam++ / Watt Toolkit.

This repository is not intended to become the final public Go library itself. It is the experimental proving ground. After the core behavior and architecture are validated, a separate repository should be created for the formal reusable Go library, reusing or porting the proven pieces from this repository.

Current facts:

- The remote repository is `gofurry/siteboost-core`.
- The Go module is still `github.com/gofurry/go-steam-core`.
- The CLI is still `steam-accelerator`.
- `version.go` reports `v0.7.0-dev`.
- The main branch contains Windows AppHost Service and named pipe IPC work.
- Local real-machine validation has passed for AppHost health, named pipe RPC, the normal-user Hosts loop, China-network Steam access, `stop`, `restore`, and uninstall behavior. A dedicated reboot auto-start smoke is still recommended.

Steam is currently the only real provider. The Windows Hosts + DoH + HTTPS Reverse Proxy path has been manually validated in a China-network environment for Steam store, community, help, chat/login, static assets, and common CDN hosts. GitHub is available as an explicit experimental skeleton provider for architecture validation only.

## Direction

The long-term goal is to produce enough validated implementation, smoke records, and architecture boundaries here so a future dedicated Go library repository can be created with less guesswork.

The core should be split around these concepts:

- provider and rule packs
- resolver and DoH
- upstream and outbound profiles
- takeover modes: ProxyOnly, PAC, System Proxy, Hosts, DNSIntercept; TUN/VPN is deferred to external libraries or separate integrations
- local reverse proxy
- root CA and dynamic certificates
- privilege boundary and restore
- diagnostics and smoke tests

HTTP and SOCKS5 upstreams are optional enhancements. They are not the default acceleration prerequisite.

## Version Plan

### v0.6.4 - Windows AppHost Service Validation

**Status:** Main user flow recorded; dedicated reboot auto-start smoke still recommended.

Validate the Steam++-style privilege boundary:

- one administrator `apphost install`
- named pipe IPC at `\\.\pipe\SiteBoostCoreAppHost`
- pipe DACL, local-client-only pipe mode, pipe client PID checks, and client executable path checks
- normal PowerShell `start --mode hosts`
- normal PowerShell `stop` / `restore`
- Steam target hosts resolving to `127.0.0.1` with local TCP 443 reachable in Hosts mode
- `apphost status` health check reporting `health=ok`
- AppHost remaining `running` after `stop` / `restore` by design, because it is the privileged standby service and not the active acceleration state
- automatic service startup after reboot, still recommended as a dedicated record
- clear diagnostics when the service is missing, stopped, or unhealthy

### v0.6.5 - Capability Boundary Freeze and v0.7 Preflight

Freeze capability ownership before the provider refactor. The decision is that real GitHub acceleration, DNSIntercept, TUN/VPN, JS injection, and cross-platform privilege loops stay in `v1.1+` as future extension points and non-goals. They should not be implemented before `v0.7.0`.

The boundary is documented in [capability-boundary.md](capability-boundary.md):

- Providers define sites, rules, outbound profiles, exact hosts, probes, and smoke targets.
- Providers must not write hosts, install Root CA, call AppHost, create TUN/DNSIntercept state, or perform system changes.
- Takeover modes own traffic capture behavior.
- AppHost is a platform privilege executor, not a provider feature.
- The current Steam Windows Hosts + DoH + AppHost smoke remains the provider-refactor regression baseline.

Post-v0.7 planning changed the order: DNSIntercept and Page Enhance are now planned before library extraction, but only with explicit, reversible behavior. TUN/VPN remains deferred and should use mature external libraries or separate integrations.

### v0.7.0 - Provider Architecture and Generic Site Skeleton

**Status:** Development and automated validation completed; real Windows smoke regression still recommended.

Completed the provider refactor. Steam-specific rules, outbound profiles, probes, and smoke targets now live behind the built-in Steam provider. GitHub is available as an explicit skeleton provider marked experimental. Core reverse proxy, resolver, and upstream packages consume generic matcher/profile data instead of Steam default data.

Validated automatically with `git diff --check`, `go test ./...`, `go vet ./...`, race tests for the core Windows/AppHost-facing packages, Windows binary build, and `--version`. Remaining manual smoke should verify the default Steam Hosts + DoH + AppHost path and an explicit `[steam, github]` provider config.

### v0.7.1 - DNSIntercept Decision and Local DNS Server

Plan a DNSIntercept foundation that does not modify system DNS by default. The first strategy is `manual`: run the DNS decision/server path only when explicitly enabled, detect port conflicts, forward non-target queries to explicit upstreams, expose stats/status, and leave no persistent system state after shutdown.

### v0.7.2 - Explicit Windows System DNS Takeover and Restore

Add `strategy: system` only after the manual DNS path is stable. System DNS changes must go through AppHost allowlisted commands, write rollback state before applying changes, restore on `stop` / `restore`, and avoid leaving the machine pointed at a dead local DNS server.

### v0.7.3 - Transparent Page Enhancement Pipeline

Add an opt-in reverse-proxy response transform pipeline. The library should provide mechanical transforms such as header edits, HTML injection, local asset serving, and replacements, but it should not hide developer choices behind black-box safety skips. Every applied, skipped, or failed transform must be observable.

### v0.8.0 - Library Extraction Readiness

Prepare the future library API draft and migration inventory after Provider, DNSIntercept, and Page Enhance boundaries are validated. Include:

- `Config`
- `Engine`
- `Provider`
- `Mode`
- `Status`
- `Start`
- `Stop`
- `Restore`
- DNSIntercept manual/system/external strategies
- Page Enhance transformer pipeline

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
- external DNS tool integrations
- mature-library TUN/VPN adapters, if ever needed
- additional provider enhancement packs
- macOS / Linux Hosts, certificate, and privilege loops

## Non-goals Before v1

- Do not promise real GitHub acceleration before provider validation.
- Do not implement TUN/VPN or cross-platform AppHost loops before the core boundary is stable.
- Do not make DNSIntercept modify system DNS by default; system takeover must be explicit and reversible.
- Do not add hidden Page Enhance safety skips; developer choices must be explicit and observable.
- Do not treat an upstream proxy as required for the default loop.
- Do not describe this repository as the final Go library repository.
- Do not copy, translate, or port SteamTools source code.
- Do not present AppHost as UAC bypass; it is a controlled privileged service installed with administrator authorization.
