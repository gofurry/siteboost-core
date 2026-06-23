# Roadmap

The canonical roadmap is maintained in [../ROADMAP.md](../ROADMAP.md). This file is an English summary for contributors who do not read the Chinese roadmap.

## Current Position

The repository has moved from a Steam-only local acceleration core toward `gofurry/siteboost-core`, an experimental local site acceleration core inspired by the architecture ideas behind Steam++ / Watt Toolkit.

This repository is not intended to become the final public Go library itself. It is the experimental proving ground. The formal reusable Go library will live in [gofurry/web-boost](https://github.com/gofurry/web-boost), reusing or porting only the proven core pieces from this repository.

Current facts:

- The remote repository is `gofurry/siteboost-core`.
- The Go module is still `github.com/gofurry/go-steam-core`.
- The CLI is still `steam-accelerator`.
- `version.go` reports `v0.7.4-dev`.
- The main branch contains Windows AppHost Service and named pipe IPC work.
- Local real-machine validation has passed for AppHost health, named pipe RPC, the normal-user Hosts loop, China-network Steam access, `stop`, `restore`, uninstall behavior, and explicit Windows system DNS takeover/restore. A dedicated reboot auto-start smoke is still recommended.

Steam is currently the only real provider. The Windows Hosts + DoH + HTTPS Reverse Proxy path has been manually validated in a China-network environment for Steam store, community, help, chat/login, static assets, and common CDN hosts. GitHub is available as an explicit experimental skeleton provider for architecture validation only.

## Direction

The long-term goal is to produce enough validated implementation, smoke records, and architecture boundaries here so `gofurry/web-boost` can be built with less guesswork and without inheriting this repository's Steam naming, CLI shape, AppHost service installer, or experimental layout.

The core should be split around these concepts:

- provider and rule packs
- resolver and DoH
- upstream and outbound profiles
- takeover modes: ProxyOnly, PAC, System Proxy, Hosts, DNSIntercept manual, and explicit Windows DNSIntercept system takeover; TUN/VPN is deferred to external libraries or separate integrations
- local reverse proxy and opt-in Page Enhance response transforms
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

**Status:** Code and automated validation completed; manual smoke still recommended.

Implemented a DNSIntercept foundation that does not modify system DNS by default. The first strategy is `manual`: `mode: dns` starts the local UDP/TCP DNS server and local HTTP/HTTPS reverse proxy, maps target provider/custom domains to local addresses, returns NODATA for target HTTPS/SVCB records by default, forwards non-target queries to explicit resolver upstreams or loop-safe DoH defaults, exposes cache/stats/status, detects listen conflicts, and leaves no persistent system state after shutdown.

### v0.7.2 - Explicit Windows System DNS Takeover and Restore

**Status:** Code, automated validation, and real Windows system-DNS smoke completed.

Implemented explicit `strategy: system` for Windows DNSIntercept. It requires `mode: dns`, a loopback `:53` listener, and explicit `dns_intercept.interfaces`. System DNS changes go through AppHost allowlisted commands, write `system_dns` rollback state before applying changes, restore on `stop` / `restore`, and start/stop in an order that avoids leaving the machine pointed at a dead local DNS server. Real Windows smoke has verified that the selected adapter DNS can be switched to `127.0.0.1`, target names resolve locally, non-target names still resolve upstream, and restore returns the adapter to its prior DNS servers.

### v0.7.3 - Transparent Page Enhancement Pipeline

**Status:** Code, automated validation, and real browser Page Enhance smoke completed.

Implemented an opt-in reverse-proxy response transform pipeline. It provides mechanical transforms such as provider/host/path/content-type/status matching, header edits, HTML injection, local asset serving, replacements, and custom transformer hooks. It does not hide developer choices behind black-box safety skips. Every applied, skipped, or failed transform is observable through logs and `page_enhance` status counters.

### v0.7.4 - Steam Official Web API Outbound Profile

**Status:** Code and automated validation completed; real Go API smoke is still recommended.

Added a focused Steam provider profile for `api.steampowered.com`, using the public Steam++ / Watt Toolkit behavior-level reference that routes the Steam store/API project through `steamstore.rmbgame.net`. The profile preserves the original HTTP Host, keeps certificate-chain validation, tolerates hostname mismatch for this fronting case, and adds the official API endpoint to startup probes. `partner.steam-api.com` remains rule-captured but still uses original-host fallback until separately validated.

### v0.8.0 - Library Extraction Readiness

Prepare the `gofurry/web-boost` library API draft, directory plan, and migration inventory after Provider, DNSIntercept, and Page Enhance boundaries are validated. Include:

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
- package boundaries for `provider`, `rules`, `network`, `takeover`, `reverse`, `pageenhance`, `certstore`, `rollback`, `diagnostics`, and optional `adapters`

The current CLI should be decoupled from core assembly enough that extraction does not carry CLI-specific details into `web-boost`. See [web-boost-library-plan.md](web-boost-library-plan.md).

### v0.9.0 - Reliability, Recovery, and Release Engineering

Harden AppHost IPC, add diagnostics, version rollback state, expand CI, and prepare installer / upgrade / uninstall / signing documentation.

### v1.0.0-alpha.1 - Experimental Architecture Freeze Candidate

Freeze this repository's migration-ready architecture boundary, provider schema, configuration migration path, and security boundary documentation.

### v1.0.0 - Experimental Validation Baseline

Ship a stable validation baseline for this experimental repository. Steam remains the stable provider. Windows Hosts + DoH + AppHost is the stable one-click path. The release should include migration notes for `gofurry/web-boost`; it is not the final library release.

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
