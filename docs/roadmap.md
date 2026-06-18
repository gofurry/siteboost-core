# Roadmap

## Current Position

The v0.3.0 local acceleration core is implemented. It includes ProxyOnly, PAC, and System Proxy modes, YAML configuration, Steam domain matching, local HTTP proxying, HTTPS CONNECT tunneling, configurable resolver modes, DNS cache, IPv4/IPv6 policy, direct/HTTP/SOCKS5 upstream dialing, rollback state, a token-protected loopback control server, and `start` / `status` / `stop` / `restore` CLI commands.

The runtime remains internal. A stable public Go integration API is deferred until the project approaches v1.

## Roadmap Strategy

Priority: keep the safe proxy foundation stable before adding modes that modify system state. PAC and System Proxy come after ProxyOnly. Hosts, certificates, and HTTPS reverse proxy come later because they require elevated trust and stronger rollback behavior.

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

**Status:** Planned  
**Scope:** Security/Safety / Architecture / User-facing / Testing  
**Goal:** Add hosts-mode reverse proxy with explicit certificate and rollback boundaries.

#### Tasks

- [ ] Add hosts patcher with a project-owned marker block
- [ ] Add hosts backup, rollback, and restore
- [ ] Add local root CA generation and install/uninstall
- [ ] Add dynamic certificate issuance and cache
- [ ] Add local HTTP and HTTPS servers
- [ ] Add HTTPS reverse proxy
- [ ] Preserve original Host and TLS SNI
- [ ] Support WebSocket upgrade
- [ ] Add hosts, cert, and reverse proxy integration tests

---

### v1.0.0 - Stable API and Integration Release

**Status:** Planned  
**Scope:** API / Release / Documentation / Integration  
**Goal:** Publish a stable API suitable for SteamScope, steam-go, Wails, or local sidecar integration.

#### Tasks

- [ ] Freeze Engine API
- [ ] Freeze Config structure
- [ ] Freeze Mode enum
- [ ] Add Go package integration example
- [ ] Add CLI usage example
- [ ] Add Wails integration notes
- [ ] Complete security boundary documentation
- [ ] Complete changelog and release notes
- [ ] Publish `v1.0.0`
