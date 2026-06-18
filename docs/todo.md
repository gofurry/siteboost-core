# Todo

## Short-Term Tasks

- Harden the v0.4.0 Hosts/HTTPS reverse proxy smoke and restore paths.
- Add more proxy edge-case tests for malformed requests, dial failures, and upstream failures.
- Add a short config sample file once the next docs pass starts.
- Review whether status output should support JSON format in a patch release.
- Keep runtime implementation internal until public API design is ready.

## Medium-Term Tasks

- Evaluate macOS/Linux Hosts and certificate-store support.
- Design a future DNSIntercept mode.
- Add manual compatibility notes for real Steam domains.

## Long-Term Ideas

- Add more WebSocket reverse proxy edge-case coverage.
- Add API freeze review before `v1.0.0-alpha.1`.
- Add release automation after the public API settles.

## Known Limitations

- ProxyOnly, PAC, System Proxy, Windows Hosts, certificate management, and reverse proxy are implemented.
- Hosts files cannot express wildcard domains; v0.4.0 writes exact domains only.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope.
- Public API is not stable before `v1.0.0`.
