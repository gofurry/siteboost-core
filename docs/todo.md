# Todo

## Short-Term Tasks

- Keep the DNSIntercept manual high-port and Windows system DNS smoke records current for the v0.7.3-dev path.
- Finish Page Enhance manual smoke with high-port DNSIntercept and a local asset.
- Finish real Windows smoke for the provider registry and Hosts/AppHost path.
- Prepare the v0.8.0 public-library extraction plan and package boundary draft after DNSIntercept/Page Enhance boundaries are validated.
- Add provider developer documentation using the GitHub skeleton provider as the minimal example.
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
- Prepare the v0.8.0 extraction readiness plan after Page Enhance smoke; keep VPN/TUN deferred to mature external adapters.

## Known Limitations

- ProxyOnly, PAC, System Proxy, Windows Hosts, certificate management, and reverse proxy are implemented.
- Steam is the default stable provider; GitHub is an explicit experimental skeleton provider and does not promise real acceleration.
- Normal Windows PowerShell can use the installed AppHost named pipe for default Hosts / root CA / restore system changes; custom hosts, cert, or rollback paths still require an elevated process or a future controlled desktop integration.
- Hosts files cannot express wildcard domains; current Hosts mode writes exact domains only. DNSIntercept manual can cover wildcard rules when explicitly enabled.
- Linux desktop system proxy handling is deferred.
- DNSIntercept manual, explicit Windows system DNS takeover, and opt-in JS injection/page enhancement are implemented. VPN/TUN is deferred to external adapters.
- Public API is not stable before `v1.0.0`.
