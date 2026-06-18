# Todo

## Short-Term Tasks

- Harden the v0.3.0 PAC/System Proxy smoke and restore paths.
- Add more proxy edge-case tests for malformed requests, dial failures, and upstream failures.
- Add a short config sample file once the next docs pass starts.
- Review whether status output should support JSON format in a patch release.
- Keep runtime implementation internal until public API design is ready.

## Medium-Term Tasks

- Add hosts-mode reverse proxy.
- Add local root CA management.
- Add dynamic certificate issuance.

## Long-Term Ideas

- Add WebSocket reverse proxy coverage.
- Add API freeze review before `v1.0.0-alpha.1`.
- Add release automation after the public API settles.

## Known Limitations

- ProxyOnly, PAC, and System Proxy are implemented; Hosts, certificates, and reverse proxy are deferred.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope.
- Public API is not stable before `v1.0.0`.
