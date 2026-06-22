# Windows 一键系统修改流程

本文记录 v0.6.3 Windows Hosts 模式的系统修改边界。

## 边界

核心负责确定性、受限的系统动作：

- 检查并信任配置的 Windows Root store 中的本项目 Root CA。
- 检查 hosts 可读写前置条件。
- 启动本地 HTTP / HTTPS reverse proxy 监听。
- 写入项目拥有的 hosts 标记区块。
- 记录 rollback 状态，并通过 `system_change` 暴露诊断信息。
- 通过 `stop` 或 `restore` 恢复项目拥有的 hosts 修改。
- 只通过显式 `cert uninstall` 卸载本项目 Root CA。

核心不会绕过 UAC、企业策略或文件系统权限。普通 PowerShell 下，主进程会通过 Windows `ShellExecute/runas` 拉起同一可执行文件的隐藏 `__helper` 入口，请求一次显式 UAC 授权。提权侧只暴露很窄的白名单命令，不接受任意 shell、任意文件写入、代理凭据、Cookie 或用户秘密。

## v0.6.3 行为

`cert.auto_install` 默认是 `true`，`cert.store_scope` 默认是 `machine`。Hosts 模式下：

1. `start --mode hosts` 先启动本地 HTTP / HTTPS reverse proxy。
2. 管理员进程直接检查并写入 Root CA / hosts。
3. 普通进程通过 helper 执行 `prepare-hosts-start`，在同一次 UAC 授权内完成 Root CA 信任检查/安装、hosts preflight 和 hosts 写入。
4. 如果 `cert.auto_install` 为 false，启动会停止并提示先执行 `cert install`。
5. `stop` / `restore` 恢复 hosts，以及 `cert install` / `cert uninstall` 写机器级证书库时，也会在普通进程下通过 helper 请求 UAC。

默认 `machine` 会写入 `LocalMachine\Root`，这是管理员运行 Hosts 模式时的低打扰路径，也可以避开 `CurrentUser\Root` 常见的首次确认框。普通 PowerShell 仍需要用户确认 UAC；这是显式系统授权，不是静默绕过。只有明确需要当前用户证书库时才配置 `cert.store_scope: user`。

`status` 会打印 `system_change:` 行，调用方可以看到哪些系统动作已执行：

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed
system_change: component=hosts action=preflight status=ok
system_change: component=reverse_proxy action=listen status=ok
system_change: component=hosts action=apply status=ok detail=entries=13
```

普通 PowerShell 通过 helper 成功时，Root CA 或 hosts 行会带上 `helper=elevated`：

```text
system_change: component=root_ca action=install status=ok detail=store=machine,installed,helper=elevated
system_change: component=hosts action=apply status=ok detail=entries=13,helper=elevated
```

## Helper 契约

当前 helper 契约保持很窄：

| 命令 | 输入 | 输出 | 说明 |
|---|---|---|---|
| `prepare-hosts-start` | 证书配置、hosts entries、rollback path、auto_install | 证书信任结果和 hosts 写入结果 | 同一次 UAC 内完成启动前系统修改 |
| `trust-root-ca` | 证书目录、store scope | 信任结果 | 幂等 |
| `restore-hosts` | rollback path | 恢复结果 | 只允许项目标记区块 |
| `untrust-root-ca` | 证书目录、store scope | 卸载结果 | 必须是显式用户动作 |

请求通过临时 JSON request / response 文件传递，并校验：

- helper request version。
- 随机 token。
- 父进程 PID。
- 命令白名单。
- 默认 Windows hosts 路径。
- 默认项目 runtime / cert 目录下的 rollback 与证书路径。
- 超时。

因此，非管理员 helper 不支持任意 `hosts.path`、`runtime.rollback_path` 或 `cert.dir`。需要自定义路径时，请使用管理员进程或后续受控桌面集成。
