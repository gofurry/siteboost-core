# Internal Package Plan

This directory contains implementation packages that should not become part of
the public Go API before the project reaches a stable release.

Planned package boundaries:

- `config`: YAML loading, defaults, validation, and CLI override support.
- `provider`: built-in provider registry, provider metadata, rule packs, outbound profiles, and startup probe targets.
- `rules`: generic domain rules and host matching.
- `resolver`: system DNS, UDP DNS, TCP DNS, DoH, DNS cache, and IP preference policy.
- `upstream`: direct, HTTP CONNECT upstream, SOCKS5 outbound dialing, and generic outbound profiles.
- `proxy`: HTTP proxy and HTTPS CONNECT for ProxyOnly mode.
- `pac`: PAC generation and local PAC server.
- `systemproxy`: Windows/macOS system proxy setup, rollback, and restore.
- `hosts`: Windows hosts marker block patching and restore.
- `certstore`: local root CA, Windows certificate store install/uninstall, and dynamic site certificates.
- `reverse`: hosts-mode HTTP/HTTPS reverse proxy.
- `engine`: lifecycle, status, start, and stop orchestration.
- `runtime`: local state file and loopback control server.

Planned packages:

- `log`: structured logging helpers if the standard library handler becomes insufficient.
