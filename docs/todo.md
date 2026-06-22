# Todo

## Short-Term Tasks

- Move into the v0.7.0 general acceleration core refactor and rename preparation.
- Audit Steam-specific naming and hardcoded assumptions across code, config, CLI, status output, and docs.
- Design neutral provider, rule-pack, outbound-profile, takeover-mode, and restore-state concepts.
- Add one non-Steam example provider to prove the core can support other accelerated targets.
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
- Normal Windows PowerShell can use an explicit UAC helper for default Hosts / root CA / restore system changes; custom hosts, cert, or rollback paths still require an elevated process or a future controlled desktop integration.
- Hosts files cannot express wildcard domains; current Hosts mode writes exact domains only.
- Linux desktop system proxy handling is deferred.
- DNSIntercept, VPN / TUN, and JS injection are out of v1.0 scope but are part of the staged v1.x advanced roadmap.
- Public API is not stable before `v1.0.0`.
