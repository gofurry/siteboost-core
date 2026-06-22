# Security Policy

## Supported Versions

The project has not published a stable runtime release yet. Security reports should target the default branch until the first tagged release exists.

## Security Boundary

steam-accelerator-core is intended to run as a local tool. The safe defaults are:

- listen on `127.0.0.1` only;
- proxy Steam rule domains only;
- do not decrypt HTTPS in ProxyOnly mode;
- install a local root CA only as part of an explicit Hosts-mode or `cert install` action;
- request Windows UAC only for explicit Hosts-mode, restore, or certificate actions that need system writes;
- do not modify hosts by default;
- do not expose a public proxy endpoint;
- do not log Cookie, Authorization, proxy passwords, tokens, or full sensitive URLs.

## High-Risk Modes

PAC, System Proxy, Hosts, certificate installation, and HTTPS reverse proxy modes modify system or trust state. These modes must:

- be explicit user actions;
- record rollback state before changes;
- provide `restore` behavior;
- document manual recovery steps;
- keep all changes scoped to project-owned settings or marker blocks.
- check for an existing project root CA before running the certificate install action again.
- when `cert.auto_install` is enabled, keep root CA trust scoped to the explicit `start --mode hosts` flow and the configured Windows Root store. The default is `cert.store_scope: machine`; normal Windows processes use an explicit UAC-triggered helper when system writes are required, and `user` remains available as a compatibility fallback. The core must not bypass UAC, enterprise policy, or accept arbitrary system-change commands.
- keep elevated helper commands narrow. The v0.6.3 helper accepts only project-owned `prepare-hosts-start`, `trust-root-ca`, `restore-hosts`, and `untrust-root-ca` requests, validates token and parent PID values, restricts paths to the default hosts file and project runtime/config directories, and times out stalled requests.

In v0.6.3, Hosts and certificate-store setup are still Windows-first. `restore` removes project-owned hosts or proxy rollback state, while root CA removal remains an explicit `cert uninstall` action.

## Reporting

For now, report issues through the repository issue tracker. Do not include secrets, Steam account tokens, cookies, private certificates, or real proxy credentials in reports.

## Clean-Room Rule

This project references Watt Toolkit / SteamTools architecture ideas only. Do not submit copied, translated, or mechanically ported SteamTools source code.
