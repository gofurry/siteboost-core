# Internal Package Plan

This directory is reserved for implementation packages that should not become
part of the public Go API before the project reaches a stable release.

Planned package boundaries:

- `engine`: lifecycle, status, start, stop, and restore orchestration.
- `rules`: Steam domain rules and host matching.
- `proxy`: HTTP proxy, HTTPS CONNECT, and PAC server.
- `resolver`: DNS, DoH, caching, and IP preference policy.
- `upstream`: direct, HTTP proxy, and SOCKS5 outbound dialing.
- `patcher`: hosts, PAC, system proxy, and rollback state.
- `reverse`: hosts-mode HTTP/HTTPS reverse proxy.
- `cert`: local root CA and dynamic site certificates.
- `config`: defaults, validation, and config loading.
- `log`: structured logging helpers.
