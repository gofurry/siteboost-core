# Capability Boundary Freeze

> Target stage: `v0.6.5`
> Purpose: define capability boundaries before the `v0.7.0` provider refactor, so future `v1.1+` advanced features do not leak into the initial reusable library shape.

## Decision

Do not implement real GitHub acceleration, DNSIntercept, TUN/VPN, JS injection, or cross-platform AppHost equivalents before `v0.7.0`. Record them only as future extension points and non-goals.

The boundary to freeze now is:

- Providers describe sites; they do not modify the system.
- Takeover modes describe how traffic is captured; they do not contain site-specific logic.
- AppHost executes allowlisted system changes; it does not expose arbitrary shell commands or provider-specific commands.
- Certificate and Root CA behavior belongs to runtime/takeover layers, not providers.
- Resolver and upstream packages consume generic rules and profiles; they should not depend on Steam-specific semantics.
- Diagnostics explain state and failure layers; they do not silently change routing.

## Provider Boundary

A provider may define:

- `id`, `name`, and `status`, such as `stable`, `skeleton`, or `experimental`.
- Rule packs for domains, paths, or groups.
- Outbound profiles, including ForwardHost, TLS SNI, candidate targets, direct routing, or optional upstream strategy.
- Exact host lists that Hosts mode can write.
- Startup probes and smoke targets.
- Documentation metadata, known limits, and validation links.

A provider must not:

- Write the hosts file.
- Install or uninstall a Root CA.
- Call AppHost or perform privileged system changes.
- Create DNSIntercept, TUN/VPN, System Proxy, or PAC state directly.
- Execute shell commands, write arbitrary files, delete arbitrary paths, or read sensitive credentials.
- Push site-specific logic into core reverse / resolver / upstream packages.

## Takeover Mode Boundary

A takeover mode describes how traffic is captured, not which site is captured.

Existing or validated modes:

- `proxy_only`: local HTTP proxy and HTTPS CONNECT.
- `pac`: PAC server and PAC file.
- `system_proxy`: current-user system proxy settings.
- `hosts`: exact hosts writes, local HTTP / HTTPS reverse proxy, Root CA, and dynamic certificates.

Future extension points:

- `dns_intercept`: for wildcard coverage and non-browser DNS paths that hosts cannot represent.
- `tun` / `vpn`: for stronger traffic capture.

Before `v0.7.0`, `dns_intercept` and `tun` should remain names and boundaries only. Do not implement routing, drivers, virtual adapters, DNS hijacking, or system network stack changes.

## Privilege / AppHost Boundary

AppHost is a platform privilege executor, not a provider feature.

AppHost requests must:

- Stay within an allowlisted command surface, such as Root CA, hosts write, and restore.
- Come from a controlled client using the same binary path.
- Avoid arbitrary shell execution, arbitrary path writes, and sensitive credentials.
- Return actionable diagnostics for install, start, restore, or uninstall paths.

AppHost does not own:

- Site rule selection.
- Provider-specific commands.
- TUN/VPN driver management.
- UAC bypass.
- Long-lived sensitive user configuration.

## Certificate Boundary

Certificate behavior belongs to runtime/takeover:

- Root CA generation, installation, lookup, and uninstall are owned by certstore / privilege.
- Dynamic site certificates are owned by reverse / certificate runtime.
- Providers declare HTTPS takeover hosts; they do not issue certificates directly.
- Root CA default behavior, trust scope, and uninstall paths must remain visible in documentation.

## Resolver / Upstream Boundary

Resolver and upstream are generic network capabilities:

- Resolver may support system / udp / tcp / doh, plus future cache, fallback, and IPv4/IPv6 policy.
- Upstream may support direct, optional HTTP / SOCKS5 upstreams, provider outbound profiles, and candidate dialing.
- Providers may supply ForwardHost, TLS SNI, and candidates, but core packages should not contain Steam or GitHub-specific checks.
- HTTP / SOCKS5 upstreams are optional enhancements, not the default loop prerequisite.

## Runtime / Status / Diagnostics Boundary

The future library and current CLI should be able to express:

- `unsupported`: platform or mode is unsupported.
- `experimental`: capability exists but is not stable.
- `requires_admin_init`: one administrator initialization is required.
- `running`: active.
- `restored`: system changes are restored.
- `partial_failure`: part of the flow failed with a clear layer and recovery hint.

Diagnostics should identify the failing layer:

- AppHost missing, stopped, or unhealthy.
- Root CA check or install failure.
- Hosts preflight or write failure.
- Listener port conflict.
- Resolver / DoH failure.
- TCP / TLS / HTTP smoke failure.

Diagnostics should not silently perform high-risk system changes unless the user explicitly runs the corresponding command or start flow.

## v0.7.0 Prerequisites

Before the provider refactor:

- Steam is the only stable provider.
- GitHub may only be a skeleton / experimental provider, with no real acceleration promise.
- Provider interfaces must express status and limitations.
- Provider interfaces must exclude system modification responsibilities.
- `reverse`, `resolver`, and `upstream` should depend only on generic matchers and profiles.
- Advanced takeover capabilities may appear only as future modes, not default behavior.

## Deferred To v1.1+

These do not enter `v0.7.0` without a new safety design and dedicated smoke:

- Real GitHub provider network validation.
- DNSIntercept.
- VPN / TUN.
- JS injection or page enhancement.
- macOS / Linux Hosts, certificate, and privilege loops.
- Full AppHost user-session binding and audit-log productization.

## v0.6.5 Acceptance

- Documentation makes clear that v1.1+ advanced capabilities do not move before v0.7.
- Provider allowed and forbidden responsibilities are explicit.
- Takeover modes are separated from Providers.
- AppHost is defined as a platform privilege executor, not part of a site provider.
- The current Steam Windows Hosts + DoH + AppHost smoke remains the v0.7 regression baseline.
