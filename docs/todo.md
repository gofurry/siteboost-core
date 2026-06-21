# Todo

## Short-Term Tasks

- Move into v0.6.0 real Steam smoke validation for store, community, login, chat, static assets, and WebSocket flows.
- Validate the implemented default Steam outbound profile against real access, then continue filling chat, static, API, and CDN groups.
- Complete at least one real Windows smoke record using the Steam compatibility matrix.
- Add more proxy edge-case tests for malformed requests, dial failures, and upstream failures.
- Add a short config sample file once the next docs pass starts.
- Keep runtime implementation internal until public API design is ready.

## Medium-Term Tasks

- Evaluate macOS/Linux Hosts and certificate-store support.
- Review whether status output should support JSON format in a patch release.

## Long-Term Ideas

- Add more WebSocket reverse proxy edge-case coverage.
- Add API freeze review before `v1.0.0-alpha.1`.
- Add release automation after the public API settles.
- Evaluate DNSIntercept, VPN / TUN, and JS injection as staged `v1.x` advanced Steam++-style capabilities.

## Known Limitations

- ProxyOnly, PAC, System Proxy, Windows Hosts, certificate management, and reverse proxy are implemented.
- Hosts files cannot express wildcard domains; current Hosts mode writes exact domains only.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope but are part of the staged v1.x advanced roadmap.
- Public API is not stable before `v1.0.0`.
