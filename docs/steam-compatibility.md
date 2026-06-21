# Steam Compatibility Matrix

This document tracks the v0.6.0 default Steam web-acceleration coverage. It is a clean-room compatibility checklist, not a copy of Steam++ / Watt Toolkit data.

## Startup Probes

Hosts + Direct mode runs non-fatal startup probes through the same DoH and outbound profile path used by the reverse proxy. The probes check resolution, TCP connect, TLS handshake, and a lightweight HTTPS `HEAD /` request.

`start` and `status` print:

```text
startup_probes: ok=6 failed=0
startup_probe_failed: host=store.steampowered.com target=cdn-a.akamaihd.net stage=tcp error=tcp ...
```

Default probe targets:

| Host | Expected profile target | Purpose |
|---|---|---|
| `steamcommunity.com` | `steamcommunity-a.akamaihd.net` | Community entry point |
| `store.steampowered.com` | `cdn-a.akamaihd.net` | Store entry point |
| `help.steampowered.com` | `cdn-a.akamaihd.net` | Support entry point |
| `media.steampowered.com` | `cdn-a.akamaihd.net` | Media/static asset host |
| `community.steamstatic.com` | `community.steamstatic.com` | Community static assets |
| `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | CDN asset host |

Probe failures do not block startup. They are diagnostics for deciding whether the next failure is DNS/DoH, TCP reachability, TLS/SNI behavior, or missing rule/profile coverage.

## Hosts Exact List

Hosts mode can only write exact domains. The default exact entries are:

```text
api.steampowered.com
akamai.steamstatic.com
checkout.steampowered.com
community.steamstatic.com
help.steampowered.com
login.steampowered.com
media.steampowered.com
partner.steam-api.com
steam-chat.com
steamcdn-a.akamaihd.net
steamcommunity.com
steamstatic.com
store.steampowered.com
```

Wildcard rules are matched by ProxyOnly/PAC/System modes but cannot be expressed in the hosts file:

```text
*.akamai.steamstatic.com
*.steam-chat.com
*.steamcommunity.com
*.steamstatic.com
```

For a wildcard subdomain that must be tested in Hosts mode before DNSIntercept exists, add that specific hostname to `hosts.extra_domains` or `rules.custom_domains`.

## Coverage Table

| Group | Hosts / rules | Default outbound profile | Startup probe | Manual smoke status |
|---|---|---|---|---|
| Store | `store.steampowered.com`, `checkout.steampowered.com`, `help.steampowered.com`, `login.steampowered.com`, `media.steampowered.com` | `cdn-a.akamaihd.net` | store/help/media | Windows China-network smoke passed |
| Community | `steamcommunity.com`, plus wildcard `*.steamcommunity.com` outside Hosts exact coverage | `steamcommunity-a.akamaihd.net` | `steamcommunity.com` | Windows China-network smoke passed |
| API | `api.steampowered.com`, `partner.steam-api.com` | Original host direct fallback | Not probed by default | API smoke pending |
| Chat | `steam-chat.com`, plus wildcard `*.steam-chat.com` outside Hosts exact coverage | Original host direct fallback | Not probed by default | `steamcommunity.com/chat/` smoke passed |
| Static | `community.steamstatic.com`, `steamstatic.com`, `akamai.steamstatic.com`, plus static wildcards outside Hosts exact coverage | `community.steamstatic.com` has explicit profile; other static hosts use original host fallback | `community.steamstatic.com` | Windows China-network smoke passed |
| CDN | `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | Windows China-network smoke passed |

## Manual Smoke Record Template

Use this table after a real Windows Hosts-mode test. Do not mark a row as passed unless it was tested on the machine and network described below.

Environment:

```text
Date:
OS:
Network:
Command:
Root CA state:
Ports:
Browser / Steam client:
```

| Scenario | URL / action | Expected result | Actual result | Pass |
|---|---|---|---|---|
| Community homepage | `https://steamcommunity.com/` | Page content loads |  |  |
| Store homepage | `https://store.steampowered.com/` | Page content loads |  |  |
| Help homepage | `https://help.steampowered.com/` | Page content loads |  |  |
| Login page | Open Steam login flow | Login page assets load |  |  |
| Static asset | `https://community.steamstatic.com/` | HTTP response is returned |  |  |
| CDN asset | `https://steamcdn-a.akamaihd.net/` | HTTP response is returned |  |  |
| Chat / WebSocket | Steam web chat or client embedded browser | WebSocket connects or failure is diagnosable |  |  |

## Recorded Smoke: Windows Hosts / China Network / 2026-06-22

Environment:

```text
Date: 2026-06-22
OS: Windows
Network: China network
Mode: Hosts + Direct + default DoH + default Steam outbound profiles
Root CA state: installed and trusted
Ports: 127.0.0.1:80 / 127.0.0.1:443
Browser / Steam client: browser login and https://steamcommunity.com/chat/ worked normally
```

| Scenario | URL / action | Result | Pass |
|---|---|---|---|
| Hosts takeover | `Test-NetConnection steamcommunity.com -Port 443` | `RemoteAddress=127.0.0.1`, TCP succeeded | Yes |
| Hosts takeover | `Test-NetConnection store.steampowered.com -Port 443` | `RemoteAddress=127.0.0.1`, TCP succeeded | Yes |
| Hosts takeover | `Test-NetConnection help.steampowered.com -Port 443` | `RemoteAddress=127.0.0.1`, TCP succeeded | Yes |
| Community homepage | `curl.exe --ssl-no-revoke -I https://steamcommunity.com/` | `HTTP/1.1 200 OK` | Yes |
| Store homepage | `curl.exe --ssl-no-revoke -I https://store.steampowered.com/` | `HTTP/1.1 200 OK` | Yes |
| Help homepage | `curl.exe --ssl-no-revoke -I https://help.steampowered.com/` | `HTTP/1.1 302 Found` to `/en/` | Yes |
| Static asset | `curl.exe --ssl-no-revoke -I https://community.steamstatic.com/` | `HTTP/1.1 403 Forbidden`, proving TLS and upstream HTTP response path worked | Yes |
| Media asset | `curl.exe --ssl-no-revoke -I https://media.steampowered.com/` | `HTTP/1.1 200 OK` | Yes |
| CDN asset | `curl.exe --ssl-no-revoke -I https://steamcdn-a.akamaihd.net/` | `HTTP/1.1 200 OK` | Yes |
| Login flow | Steam login page | Page and interaction worked normally | Yes |
| Chat / WebSocket | `https://steamcommunity.com/chat/` | Page and chat behavior worked normally | Yes |

## Common Failure Mapping

| Symptom | Likely layer | Next check |
|---|---|---|
| `startup_probe_failed ... stage=resolve` | DoH / DNS | Check `resolver_servers`, firewall, DNS interception |
| `stage=tcp` | Direct reachability | Check candidate IP reachability and profile target |
| `stage=tls` | TLS / SNI / certificate chain | Check `tls_server_name`, certificate validation, local clock |
| Browser certificate warning | Local root CA trust | Run `cert install`, check current-user Root store |
| Browser works, `curl.exe` reports `CRYPT_E_NO_REVOCATION_CHECK` | Windows Schannel revocation check | Use `curl.exe --ssl-no-revoke` for command-line validation |
| Main page works, subresource fails | Rule/profile gap | Inspect failed host in browser devtools and add exact host for Hosts mode |
| WebSocket fails | Rule/profile or upgrade path | Check logs and verify the websocket host is covered |
