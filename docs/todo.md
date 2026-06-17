# Todo

## Short-Term Tasks

- Finish `v0.1.0` package boundaries.
- Add config defaults and validation.
- Add default Steam domain rules.
- Add rules matcher tests.
- Add HTTP proxy and CONNECT skeleton.
- Add Engine start, stop, and status.
- Add CLI commands for proxy-only mode.

## Medium-Term Tasks

- Add resolver and DoH implementation.
- Add DNS cache and fallback.
- Add upstream HTTP and SOCKS5 proxy support.
- Add PAC generator and PAC server.
- Add rollback state model.
- Add Windows and macOS system proxy integrations.

## Long-Term Ideas

- Add hosts-mode reverse proxy.
- Add local root CA management.
- Add dynamic certificate issuance.
- Add WebSocket reverse proxy coverage.
- Add API freeze review before `v1.0.0-alpha.1`.
- Add release automation after the public API settles.

## Known Limitations

- Runtime acceleration is not implemented yet.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope.
- Public API is not stable before `v1.0.0`.
