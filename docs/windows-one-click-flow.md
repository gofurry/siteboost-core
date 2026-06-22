# Windows One-Click System Flow

This document records the v0.6.3 Windows Hosts-mode system-change boundary.

## Boundary

The core owns deterministic, limited system actions:

- check and trust the project root CA in the configured Windows Root store;
- check hosts read/write preflight;
- start local HTTP / HTTPS reverse-proxy listeners;
- write the project-owned hosts marker block;
- record rollback state and expose `system_change` diagnostics;
- restore project-owned hosts changes with `stop` or `restore`;
- uninstall the project root CA only through explicit `cert uninstall`.

The core does not bypass UAC, enterprise policy, or file-system permissions. From a normal PowerShell, the main process launches the same executable's hidden `__helper` entrypoint with Windows `ShellExecute/runas`, requesting one explicit UAC authorization. The elevated side exposes only narrow whitelisted commands and accepts no arbitrary shell execution, arbitrary file writes, proxy credentials, cookies, or user secrets.

## v0.6.3 Behavior

`cert.auto_install` defaults to `true`, and `cert.store_scope` defaults to `machine`. In Hosts mode:

1. `start --mode hosts` starts the local HTTP / HTTPS reverse-proxy listeners first.
2. An elevated process checks and writes root CA / hosts state directly.
3. A normal process uses the helper command `prepare-hosts-start` to perform root CA trust check/install, hosts preflight, and hosts write under the same UAC authorization.
4. If `cert.auto_install` is false, startup stops with guidance to run `cert install`.
5. `stop` / `restore` hosts recovery, plus machine-scope `cert install` / `cert uninstall`, also request UAC through the helper when the process is not elevated.

The default `machine` scope writes to `LocalMachine\Root`, which is the low-friction path for administrator-run Hosts mode and avoids the first-run confirmation commonly seen with `CurrentUser\Root`. Normal PowerShell still needs user-approved UAC; this is explicit system authorization, not a silent bypass. Use `cert.store_scope: user` only when a current-user trust store is required.

`status` prints `system_change:` lines so a caller can see which system actions ran:

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed
system_change: component=hosts action=preflight status=ok
system_change: component=reverse_proxy action=listen status=ok
system_change: component=hosts action=apply status=ok detail=entries=13
```

When the normal PowerShell helper path succeeds, root CA or hosts rows include `helper=elevated`:

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed,helper=elevated
system_change: component=hosts action=apply status=ok detail=entries=13,helper=elevated
```

## Helper Contract

The current helper contract is intentionally narrow:

| Command | Input | Output | Notes |
|---|---|---|---|
| `prepare-hosts-start` | certificate config, hosts entries, rollback path, auto_install | certificate trust and hosts apply result | startup system changes under one UAC authorization |
| `trust-root-ca` | certificate directory, store scope | trust result | idempotent |
| `restore-hosts` | rollback path | restore result | project marker block only |
| `untrust-root-ca` | certificate directory, store scope | uninstall result | explicit user action |

Requests are passed through temporary JSON request/response files and validate:

- helper request version;
- random token;
- parent process PID;
- command whitelist;
- default Windows hosts path;
- rollback and certificate paths under the default project runtime/config directories;
- timeout.

Because of that boundary, the non-admin helper does not support arbitrary `hosts.path`, `runtime.rollback_path`, or `cert.dir` values. Use an elevated process or future controlled desktop integration for custom paths.
