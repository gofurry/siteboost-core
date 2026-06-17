# Internal Package Plan

This directory contains implementation packages that should not become part of
the public Go API before the project reaches a stable release.

Planned package boundaries:

- `config`: YAML loading, defaults, validation, and CLI override support.
- `rules`: Steam domain rules and host matching.
- `resolver`: system DNS, UDP DNS, TCP DNS, DoH, DNS cache, and IP preference policy.
- `upstream`: direct, HTTP CONNECT upstream, and SOCKS5 outbound dialing.
- `proxy`: HTTP proxy and HTTPS CONNECT for ProxyOnly mode.
- `engine`: lifecycle, status, start, and stop orchestration.
- `runtime`: local state file and loopback control server.

Planned packages:

- `patcher`: hosts, PAC, system proxy, and rollback state.
- `reverse`: hosts-mode HTTP/HTTPS reverse proxy.
- `cert`: local root CA and dynamic site certificates.
- `log`: structured logging helpers if the standard library handler becomes insufficient.
