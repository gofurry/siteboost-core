# Windows One-Click System Flow

This document records the v0.6.4-dev Windows Hosts-mode system-change boundary.

## Boundary

The core owns deterministic, limited system actions:

- check and trust the project root CA in the configured Windows Root store;
- check hosts read/write preflight;
- start local HTTP / HTTPS reverse-proxy listeners;
- write the project-owned hosts marker block;
- record rollback state and expose `system_change` diagnostics;
- restore project-owned hosts changes with `stop` or `restore`;
- uninstall the project root CA only through explicit `cert uninstall`.

The core does not bypass UAC, enterprise policy, or file-system permissions. The recommended default path is to run `apphost install` once from an Administrator PowerShell, installing the `SiteBoostCoreAppHost` Windows Service. The service runs in a controlled privileged context. Later normal PowerShell runs send restricted system-change requests through the local Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`.

The privileged side exposes only narrow whitelisted commands and accepts no arbitrary shell execution, arbitrary file writes, proxy credentials, cookies, or user secrets.

## v0.6.4-dev Behavior

`cert.auto_install` defaults to `true`, and `cert.store_scope` defaults to `machine`. In Hosts mode:

1. `start --mode hosts` starts the local HTTP / HTTPS reverse-proxy listeners first.
2. An administrator process checks and writes root CA / hosts state directly.
3. A normal process uses AppHost named pipe RPC to run `prepare-hosts-start`, performing root CA trust check/install, hosts preflight, and hosts write through the installed service.
4. If `cert.auto_install` is false, startup stops with guidance to run `cert install`.
5. `stop` / `restore` hosts recovery, plus machine-scope `cert install` / `cert uninstall`, also use AppHost from a normal process.

The default `machine` scope writes to `LocalMachine\Root`, which is the low-friction path for administrator-run Hosts mode and avoids the first-run confirmation commonly seen with `CurrentUser\Root`. Initial AppHost service installation still needs user-approved administrator authorization; this is explicit system authorization, not a silent bypass. Use `cert.store_scope: user` only when a current-user trust store is required.

`status` prints `system_change:` lines so a caller can see which system actions ran:

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed
system_change: component=hosts action=preflight status=ok
system_change: component=reverse_proxy action=listen status=ok
system_change: component=hosts action=apply status=ok detail=entries=13
```

When the normal PowerShell AppHost path succeeds, root CA or hosts rows currently include the compatibility detail `helper=elevated`:

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed,helper=elevated
system_change: component=hosts action=apply status=ok detail=entries=13,helper=elevated
```

## AppHost Contract

The current AppHost contract is intentionally narrow:

| Command | Input | Output | Notes |
|---|---|---|---|
| `prepare-hosts-start` | certificate config, hosts entries, rollback path, auto_install | certificate trust and hosts apply result | startup system changes through AppHost |
| `trust-root-ca` | certificate directory, store scope | trust result | idempotent |
| `restore-hosts` | rollback path | restore result | project marker block only |
| `untrust-root-ca` | certificate directory, store scope | uninstall result | explicit user action |
| `apphost-health` | none | health result | pipe health check |

Requests are passed through the Windows named pipe `\\.\pipe\SiteBoostCoreAppHost` and validate:

- request version;
- random token is non-empty;
- parent process PID;
- pipe client PID equals the request parent PID;
- pipe client executable path equals the installed AppHost executable path;
- command whitelist;
- default Windows hosts path;
- rollback and certificate paths under the default project runtime/config directories;
- timeout.

The pipe is created with a DACL for local access control and enables `PIPE_REJECT_REMOTE_CLIENTS` when the platform supports it. Because of that boundary, AppHost does not support arbitrary `hosts.path`, `runtime.rollback_path`, or `cert.dir` values from a normal process. Use an elevated process or future controlled desktop integration for custom paths.
