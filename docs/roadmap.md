# Roadmap

## Current Position

The v0.6.3 line has the first Hosts + DoH default loop, outbound failure diagnostics, the first default Steam outbound profile, startup probes, a Windows China-network smoke record, Windows one-click certificate/hosts orchestration, machine-scope certificate install by default, and a narrow Windows elevated helper. It includes ProxyOnly, PAC, System Proxy, and Windows-first Hosts reverse proxy modes, YAML configuration, Steam domain matching, local HTTP proxying, HTTPS CONNECT tunneling, configurable resolver modes, DNS cache, IPv4/IPv6 policy, direct/HTTP/SOCKS5 upstream dialing, local root CA generation, dynamic site certificates, rollback state, a token-protected loopback control server, and `start` / `status` / `stop` / `restore` / `cert install` / `cert uninstall` CLI commands.

In Hosts + Direct mode, runtime outbound resolution now uses built-in DoH defaults instead of the system resolver, preventing the local hosts marker block from resolving Steam domains back to `127.0.0.1`. Hosts mode also performs preflight checks for the root CA, hosts readability/writability, rollback directory writability, reverse-proxy listen errors, and hosts write rollback. Reverse Proxy / Proxy 502 responses now include a trimmed outbound error summary, and Direct outbound errors distinguish DoH resolve, TCP connect, and TLS handshake stages. The default Steam outbound profile now lets community domains prefer `steamcommunity-a.akamaihd.net`, store / checkout / help / login / media domains prefer `cdn-a.akamaihd.net`, and common static/CDN hosts such as `community.steamstatic.com` and `steamcdn-a.akamaihd.net` are covered, while preserving the original HTTP Host and using profile-specific TLS SNI before falling back to the original domain. Hosts + Direct startup also runs non-fatal probes and exposes rule metadata plus `startup_probes` in `start` / `status`.

Starting in v0.6.1, `cert.auto_install` defaults to true. In v0.6.2, the default Windows store scope is `machine`, so administrator-run Hosts mode installs into `LocalMachine\Root` through the Windows certificate-store API to avoid the first-run confirmation commonly seen with CurrentUser root installs. In v0.6.3, a normal PowerShell can request one Windows UAC prompt through a same-binary hidden helper launched with `ShellExecute/runas` when `LocalMachine\Root` or Windows hosts writes are needed. `cert.store_scope: user` remains available as a compatibility fallback. `start --mode hosts` skips installation when already trusted and reports root CA / hosts / listener actions through `system_change` output.

This is closer to a Steam++-style one-click path but is not a full Steam++ replacement. The project now has a compatibility checklist, a Windows store/community/login/chat/static/WebSocket pass record, core certificate/hosts orchestration, and an explicit UAC-triggered helper for normal startup paths. Hosts still cannot cover wildcard domains, custom helper paths are intentionally constrained, macOS/Linux Hosts support, DNSIntercept/VPN/TUN modes, desktop packaging, and a stable public Go API remain future work.

The runtime remains internal. A stable public Go integration API is deferred until the project approaches v1. After the Steam flow is validated, the project is expected to evolve into a more general local acceleration core. A future repository/module rename is possible; Steam should become a built-in rule/profile provider rather than the only core target.

## Roadmap Strategy

Priority: keep the safe proxy foundation stable while making the default Hosts + DoH loop usable without a user-configured upstream proxy. The Windows CLI can now request one explicit UAC prompt when system writes are needed; next, refactor toward a provider-based general acceleration core and prepare rename/module migration. HTTP and SOCKS5 upstreams remain optional enhancements, not the default acceleration prerequisite.

## Version Plan

### v0.1.0 - ProxyOnly Core

**Status:** Completed  
**Scope:** Stability / Developer-facing / Testing / Documentation  
**Goal:** Build the minimal local proxy core without modifying system state or installing certificates.

#### Completed

- [x] Add Go module, README files, CI, basic example, and docs layout
- [x] Document SteamTools reference boundary and clean-room rule
- [x] Add config defaults, YAML loading, and validation
- [x] Add default Steam domain rules
- [x] Add rules matcher with exact, wildcard, port stripping, lowercase, and IDNA handling
- [x] Add HTTP proxy framework
- [x] Add HTTPS CONNECT tunnel
- [x] Add direct upstream
- [x] Add engine start, stop, and status
- [x] Add CLI commands for proxy-only start, stop, and status
- [x] Add unit tests for config, rules, proxy, engine, and runtime control

#### Acceptance Criteria

- The proxy listens on `127.0.0.1:26501` by default
- Steam rule domains can be forwarded through manual browser proxy setup
- Non-Steam domains are rejected by default and can be configured as direct
- Stop releases the listening port
- `gofmt`, `go vet`, and `go test ./...` pass

---

### v0.2.0 - Resolver, DoH, and Upstream

**Status:** Completed
**Scope:** Stability / User-facing / Performance / Testing  
**Goal:** Add configurable DNS resolution and outbound proxy support.

#### Tasks

- [x] Add resolver interface
- [x] Add system, UDP, TCP, and DoH resolvers
- [x] Add DNS cache, timeout, and fallback behavior
- [x] Add IPv4 and IPv6 selection policy
- [x] Add HTTP proxy and SOCKS5 upstreams
- [x] Keep proxy credentials out of logs
- [x] Wire resolver and upstream into proxy dialing
- [x] Add resolver and upstream tests

---

### v0.3.0 - PAC and System Proxy

**Status:** Completed
**Scope:** User-facing / Safety / Cross-platform / Testing  
**Goal:** Add PAC and system proxy takeover while preserving rollback behavior.

#### Tasks

- [x] Generate PAC from the rules module
- [x] Add local PAC server
- [x] Add `start --mode pac`
- [x] Add Windows and macOS PAC setup and restore
- [x] Add Windows and macOS HTTP/HTTPS proxy setup and restore
- [x] Add rollback state file
- [x] Add `restore`
- [x] Add PAC and system proxy integration tests

---

### v0.4.0 - Hosts and HTTPS Reverse Proxy

**Status:** Completed
**Scope:** Security/Safety / Architecture / User-facing / Testing  
**Goal:** Add hosts-mode reverse proxy with explicit certificate and rollback boundaries.

#### Tasks

- [x] Add hosts patcher with a project-owned marker block
- [x] Add hosts backup, rollback, and restore
- [x] Add local root CA generation and install/uninstall
- [x] Add dynamic certificate issuance and cache
- [x] Add local HTTP and HTTPS servers
- [x] Add HTTPS reverse proxy
- [x] Preserve original Host and TLS SNI
- [x] Support WebSocket upgrade
- [x] Add hosts, cert, and reverse proxy integration tests

#### Notes

- v0.4.0 is Windows-first; macOS/Linux Hosts and certificate-store setup are unsupported.
- Hosts files cannot express wildcard domains, so v0.4.0 writes exact domains only.
- `restore` removes the hosts marker block; root CA uninstall remains an explicit `cert uninstall` action.

---

### v0.5.0 - One-click Hosts + DoH Default Loop

**Status:** Default-loop code completed; real Steam smoke validation deferred to v0.6.0
**Scope:** User-facing / Stability / Security-Safety / Testing
**Goal:** Connect the Hosts reverse-proxy pieces into a Steam++-style default local loop without requiring an upstream proxy.

#### Completed

- [x] Add a Hosts + Direct resolver policy that uses DoH by default and avoids local hosts loopback.
- [x] Add built-in DoH servers with fallback order and user override support.
- [x] Add Hosts-mode preflight for root CA state, hosts read/write access, rollback directory writability, listen failures, and hosts write rollback.
- [x] Keep HTTP/SOCKS5 upstreams as optional enhancements.
- [x] Expose runtime resolver mode and servers in `start` / `status` output.
- [x] Add unit tests for the default resolver policy, status reporting, and hosts preflight.
- [x] Update usage and smoke-test documentation.

#### Acceptance Criteria

- Default Hosts mode does not resolve Steam domains back to `127.0.0.1` after writing hosts.
- With no upstream proxy configured, Hosts mode can still use DoH plus direct IP dialing for Steam rule domains.
- Startup failures produce clear errors and do not leave unrecoverable hosts state.
- `stop` / `restore` can restore hosts-related state; root CA uninstall remains explicit.
- `go test ./...` passes and covers the key Hosts resolver loop path.

---

### v0.5.1 - Outbound Failure Diagnostics Patch

**Status:** Completed
**Scope:** User-facing / Stability / Testing / Diagnostics
**Goal:** Turn `upstream request failed` from a black-box error into an actionable outbound failure chain before real Steam profile work.

#### Completed

- [x] Add structured Direct upstream errors with target host, port, resolve error, candidate IPs, and per-attempt failure reasons.
- [x] Add TLS-aware Direct dialing for HTTPS reverse proxy so each candidate IP can run TCP plus TLS before falling back.
- [x] Include a safely trimmed outbound error summary in Reverse Proxy / Proxy 502 responses and logs.
- [x] Add tests for direct dial diagnostics, reverse 502 diagnostics, and proxy 502 diagnostics.
- [x] Move the development version to `v0.5.1-dev` and document the new diagnostic behavior.

#### Acceptance Criteria

- When users see `upstream request failed`, the response body or logs include the concrete failure reason.
- Direct outbound failures show DoH resolve failures or candidate IP TCP / TLS failure stages.
- HTTPS reverse proxy does not give up all candidates after a single candidate IP fails TLS.
- `go test ./...` and `go vet ./...` pass.

---

### v0.6.0 - Real Steam Smoke Tests and Rule Coverage

**Status:** Completed
**Scope:** User-facing / Testing / Documentation / Stability
**Goal:** Validate the one-click loop against real Steam pages and add the default Steam outbound profile needed for Steam++-style behavior.

#### Tasks

- [x] Maintain a real Steam domain compatibility checklist for store, community, login, API, chat, static assets, and CDN domains.
- [x] Maintain the exact-domain Hosts write list and document wildcard gaps.
- [x] Design and implement the default outbound profile skeleton for core Steam domains with candidate IPs, ForwardDestination, TLS SNI, certificate-name-mismatch policy, and fallback order.
- [x] Add default `steamcommunity-a.akamaihd.net` fallback for `steamcommunity.com` / `*.steamcommunity.com`; add default `cdn-a.akamaihd.net` fallback for store / checkout / help / login / media; cover `community.steamstatic.com` and `steamcdn-a.akamaihd.net` in the default rules and profiles.
- [x] Add YAML custom outbound profile configuration and validation for `match_domains`, `candidate_ips`, `forward_host`, `tls_server_name`, and `ignore_tls_name_mismatch`.
- [x] Add startup probes for DoH resolution, TCP 443 connectivity, TLS handshake, and light HTTP smoke checks.
- [x] Add a manual Windows smoke-test template for install, start, browse, stop, restore, and uninstall.
- [x] Document common failure cases for DNS failures, untrusted certificates, port conflicts, hosts write blocks, and WebSocket failures.
- [x] Complete at least one real Windows smoke record for Steam store, community, login, static assets, and WebSocket behavior.
- [x] Extend sanitized resolver and reverse-proxy failure log fields without leaking Cookie / Authorization / URL secrets.
- [x] Add built-in rule-set version and update metadata for status output and future provider/rule-pack evolution.

---

### v0.6.1 - Windows Privilege and Certificate One-click Flow

**Status:** Completed
**Scope:** User-facing / Security-Safety / Windows / Architecture
**Goal:** Package Windows root CA writes, hosts writes, startup checks, and rollback into a low-friction core flow, while documenting the boundary for a future elevated helper.

#### Tasks

- [x] Design a Windows privileged helper / IPC boundary where the main process owns user interaction and the elevated process only performs limited system changes. See [Windows one-click system flow](windows-one-click-flow.md).
- [x] Evaluate and implement a Windows certificate-store API backend for thumbprint lookup, install, and removal of this project's root CA.
- [x] Extend `start --mode hosts` so it can check certificate trust, install during the explicit start flow when needed, and skip install work when the root CA is already trusted.
- [x] Coordinate root CA writes, hosts writes, listener startup, and rollback state in one startup workflow.
- [x] Add `status` / diagnostics for certificate trust, hosts write state, and last system-change result.
- [x] Document that the core must not accept arbitrary system changes; future desktop/elevated helpers should expose only a narrow command surface.

#### Acceptance Criteria

- A fresh Windows Hosts-mode start can trust the CA, write hosts, and start the reverse proxy in one `start` run when the process already has permission to write hosts.
- Repeated starts skip certificate installation when the project root CA is already installed.
- Failed system changes are recoverable with clear diagnostics and `restore`.
- The future helper contract exposes only the minimum system-modification surface.

---

### v0.6.2 - Windows Machine-scope Certificate Default

**Status:** Completed
**Scope:** User-facing / Security-Safety / Windows
**Goal:** Install the local root CA into `LocalMachine\Root` by default to avoid the first-run confirmation commonly seen with `CurrentUser\Root`, while preserving a compatibility fallback.

#### Tasks

- [x] Add `cert.store_scope` with `machine` and `user`.
- [x] Make `machine` the default Windows root-store scope for administrator-run Hosts mode.
- [x] Keep `cert.store_scope: user` as a `CurrentUser\Root` compatibility path.
- [x] Include the root CA store scope in `system_change`, for example `detail=store=machine,installed`.
- [x] Update usage, security, smoke, and one-click-flow documentation.

#### Acceptance Criteria

- A first administrator-run `start --mode hosts` can silently trust the root CA.
- Repeated starts skip root CA installation when the project CA is already trusted.
- Non-admin machine-store failures explain how to rerun as Administrator or switch to `cert.store_scope: user`.

---

### v0.6.3 - Windows Elevated Helper One-click Start

**Status:** Completed
**Scope:** User-facing / Security-Safety / Windows / Architecture / Testing
**Goal:** Let a normal startup path request one explicit Windows UAC elevation and launch a narrow elevated helper / AppHost for root CA, hosts, and restore actions instead of requiring users to manually open an Administrator terminal.

#### Focus

- A same-binary hidden `__helper` entrypoint launched by the main process through Windows `ShellExecute/runas`; a separate `siteboost-helper.exe` / AppHost and manifest package is deferred to future desktop or release packaging.
- Non-admin main process detection plus Windows `ShellExecute` / `runas` helper launch.
- A narrow helper command surface with no arbitrary shell execution, arbitrary file writes, or access to proxy credentials.
- A narrow temporary JSON request/response IPC boundary with token, parent-process checks, command whitelisting, path constraints, and timeouts.
- A reusable privilege boundary for CLI, future desktop shells, and Go integrations.

#### Tasks

- [x] Add a Windows privilege package for admin detection, `runas` launch, helper path lookup, and helper response waiting.
- [x] Add a hidden helper entrypoint that only accepts `prepare-hosts-start`, `trust-root-ca`, `restore-hosts`, and `untrust-root-ca`.
- [x] Use a same-binary hidden helper plus `ShellExecute/runas` as the current AppHost-equivalent path; separate helper manifest / installer packaging moves to future desktop integration.
- [x] Make `start --mode hosts` request the helper when the main process is not elevated; keep the current direct path when already elevated.
- [x] Route hosts writes, root CA writes, restore, and cert uninstall through the helper while preserving rollback and `system_change` output.
- [x] Add token, parent PID, command whitelist, path constraints, and timeout protection for helper IPC.
- [x] Add privilege boundary tests and an engine helper-branch test; real UAC, cancellation, and normal PowerShell behavior remain manual smoke checks.
- [x] Update security docs to state that this is explicit UAC elevation, not UAC bypass; reference Watt Toolkit only as an architectural boundary, not as copied code.

#### Acceptance Criteria

- Running `start --mode hosts` from a normal PowerShell can trigger one Windows UAC prompt and complete root CA trust, hosts writes, and reverse-proxy startup after authorization.
- UAC cancellation returns a clear error, leaves hosts unchanged, and does not leave partial rollback state.
- The helper cannot execute arbitrary shell commands, cannot write outside approved project-owned system changes, and cannot receive cookies, proxy passwords, or user secrets.
- `stop` / `restore` can use the helper to recover project-owned hosts changes from the normal startup path.
- Administrator PowerShell still uses the v0.6.2 silent machine-store certificate path.
- The non-admin helper only accepts the default Windows hosts path and rollback/cert files under the default project runtime/config directories; custom system paths require an elevated process or a future controlled desktop integration.

#### Notes

- This is implementable, but it must not bypass UAC. The goal is to replace "manually run an Administrator terminal" with "the program requests one system authorization when needed."
- Watt Toolkit / Steam++ publicly shows the same broad pattern through `requireAdministrator` manifests, IPCRoot, and `runas`; this project keeps a clean-room implementation.

---

### v0.7.0 - General Acceleration Core Refactor and Rename Preparation

**Status:** Planned
**Scope:** Architecture / Developer-facing / Maintainability / Documentation
**Goal:** After the Steam flow is validated, evolve the project from a Steam-specific core into a maintainable, extensible local acceleration core.

#### Tasks

- [ ] Audit Steam-specific naming and hardcoded assumptions across code, config, CLI, status output, and docs.
- [ ] Design neutral concepts for rule packs, provider profiles, target groups, outbound profiles, takeover modes, and restore state.
- [ ] Move Steam defaults into a built-in Steam provider while keeping core logic provider-neutral.
- [ ] Draft the repository rename, Go module migration, CLI migration, and config migration plan before v1.
- [ ] Clarify package boundaries for resolver, upstream, reverse proxy, certstore, privilege, and runtime responsibilities.
- [ ] Add one minimal non-Steam provider example to prove the core can support other accelerated targets.
- [ ] Keep Steam smoke tests as regression coverage during the refactor.

#### Acceptance Criteria

- Core packages can run without Steam-specific names or assumptions.
- Adding another provider does not require changes to reverse / resolver / upstream core logic.
- Migration docs explain old repository, module, CLI, and config transitions.
- The validated Steam one-click flow still passes smoke tests after the refactor.

---

### v0.8.0 - Cross-platform Loop and Integration API Candidate

**Status:** Planned
**Scope:** Cross-platform / Developer-facing / API / Documentation
**Goal:** Extend the one-click loop beyond Windows-first support and prepare a Go integration API candidate after the general core boundary is clear.

#### Tasks

- [ ] Evaluate macOS Hosts writes, root CA install/uninstall, system proxy restore, and permission paths.
- [ ] Evaluate Linux desktop hosts, trust-store, privilege, and distro differences.
- [ ] Design public Go API candidates for Engine, Config, Mode, Status, Restore, provider registration, and rule packs.
- [ ] Add package examples and Wails / sidecar integration notes.
- [ ] Version rollback state files for future compatibility.
- [ ] Document unsupported platforms and manual recovery paths.

---

### v1.0.0 - Stable API and Integration Release

**Status:** Planned  
**Scope:** API / Release / Documentation / Integration  
**Goal:** Publish a stable API suitable for provider-based local acceleration cores, including SteamScope, steam-go, Wails, or local sidecar integration.

#### Tasks

- [ ] Freeze Engine API
- [ ] Freeze Config structure
- [ ] Freeze Mode enum
- [ ] Freeze provider, rule-pack, and outbound-profile extension interfaces
- [ ] Add Go package integration example
- [ ] Add CLI usage example
- [ ] Add Wails integration notes
- [ ] Complete security boundary documentation
- [ ] Complete changelog and release notes
- [ ] Publish `v1.0.0`
