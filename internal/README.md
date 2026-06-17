# Internal Package Plan

This directory contains implementation packages that should not become part of
the public Go API before the project reaches a stable release.

Planned package boundaries:

- `config`: YAML loading, defaults, validation, and CLI override support.
- `rules`: Steam domain rules and host matching.
- `upstream`: direct outbound dialing.
- `proxy`: HTTP proxy and HTTPS CONNECT for ProxyOnly mode.
- `engine`: lifecycle, status, start, and stop orchestration.
- `runtime`: local state file and loopback control server.

Planned packages:

- `resolver`: DNS, DoH, caching, and IP preference policy.
- `patcher`: hosts, PAC, system proxy, and rollback state.
- `reverse`: hosts-mode HTTP/HTTPS reverse proxy.
- `cert`: local root CA and dynamic site certificates.
- `log`: structured logging helpers if the standard library handler becomes insufficient.
