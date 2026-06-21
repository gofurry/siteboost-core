# Security Policy

## Supported Versions

The project has not published a stable runtime release yet. Security reports should target the default branch until the first tagged release exists.

## Security Boundary

steam-accelerator-core is intended to run as a local tool. The safe defaults are:

- listen on `127.0.0.1` only;
- proxy Steam rule domains only;
- do not decrypt HTTPS in ProxyOnly mode;
- install a local root CA only as part of an explicit Hosts-mode or `cert install` action;
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
- when `cert.auto_install` is enabled, keep root CA trust scoped to the explicit `start --mode hosts` flow and the current-user Root store; the core must not bypass UAC, enterprise policy, or accept arbitrary system-change commands.

In v0.6.1, Hosts and certificate-store setup are still Windows-first. `restore` removes project-owned hosts or proxy rollback state, while root CA removal remains an explicit `cert uninstall` action.

## Reporting

For now, report issues through the repository issue tracker. Do not include secrets, Steam account tokens, cookies, private certificates, or real proxy credentials in reports.

## Clean-Room Rule

This project references Watt Toolkit / SteamTools architecture ideas only. Do not submit copied, translated, or mechanically ported SteamTools source code.
