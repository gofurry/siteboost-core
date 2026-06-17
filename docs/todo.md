# Todo

## Short-Term Tasks

- Harden the v0.2.0 resolver, upstream, and ProxyOnly CLI smoke paths.
- Add more proxy edge-case tests for malformed requests, dial failures, and upstream failures.
- Add a short config sample file once the next docs pass starts.
- Review whether status output should support JSON format in a patch release.
- Keep runtime implementation internal until public API design is ready.

## Medium-Term Tasks

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

- Only ProxyOnly runtime is implemented; PAC, System Proxy, Hosts, certificates, and reverse proxy are deferred.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope.
- Public API is not stable before `v1.0.0`.
