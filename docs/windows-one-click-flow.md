# Windows One-Click System Flow

This document records the v0.6.1 Windows Hosts-mode system-change boundary.

## Boundary

The core owns deterministic, limited system actions:

- check and trust the project root CA in the current-user Root store;
- check hosts read/write preflight;
- start local HTTP / HTTPS reverse-proxy listeners;
- write the project-owned hosts marker block;
- record rollback state and expose `system_change` diagnostics;
- restore project-owned hosts changes with `stop` or `restore`;
- uninstall the project root CA only through explicit `cert uninstall`.

The core does not bypass UAC, enterprise policy, or file-system permissions. A desktop app, elevated wrapper, or future privileged helper should own user interaction and process elevation. The elevated side should expose only narrow commands such as `trust-root-ca`, `apply-hosts`, `restore-hosts`, and `untrust-root-ca`.

## v0.6.1 Behavior

`cert.auto_install` defaults to `true`. In Hosts mode:

1. `start --mode hosts` checks whether the project root CA is already trusted.
2. If it is trusted, startup skips certificate installation.
3. If it is not trusted and `cert.auto_install` is true, startup installs it through the Windows certificate-store API.
4. If `cert.auto_install` is false, startup stops with guidance to run `cert install`.
5. Hosts preflight, reverse-proxy listeners, hosts write, and rollback state are handled in the same startup flow.

`status` prints `system_change:` lines so a caller can see which system actions ran:

```text
system_change: component=root_ca action=install status=ok detail=installed
system_change: component=hosts action=preflight status=ok
system_change: component=reverse_proxy action=listen status=ok
system_change: component=hosts action=apply status=ok detail=entries=13
```

## Future Helper Contract

When a separate helper is introduced, keep the contract narrow:

| Command | Input | Output | Notes |
|---|---|---|---|
| `trust-root-ca` | certificate path / thumbprint | trust result | idempotent |
| `apply-hosts` | marker block entries and rollback path | apply result | project marker block only |
| `restore-hosts` | rollback path | restore result | project marker block only |
| `untrust-root-ca` | thumbprint | uninstall result | explicit user action |

The helper must not accept arbitrary shell commands, arbitrary file writes, proxy credentials, cookies, or user secrets.
