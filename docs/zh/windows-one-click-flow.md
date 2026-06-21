# Windows 一键系统修改流程

本文记录 v0.6.1 Windows Hosts 模式的系统修改边界。

## 边界

核心负责确定性、受限的系统动作：

- 检查并信任当前用户 Root store 中的本项目 Root CA。
- 检查 hosts 可读写前置条件。
- 启动本地 HTTP / HTTPS reverse proxy 监听。
- 写入项目拥有的 hosts 标记区块。
- 记录 rollback 状态，并通过 `system_change` 暴露诊断信息。
- 通过 `stop` 或 `restore` 恢复项目拥有的 hosts 修改。
- 只通过显式 `cert uninstall` 卸载本项目 Root CA。

核心不会绕过 UAC、企业策略或文件系统权限。桌面壳、提权 wrapper 或后续 privileged helper 应负责用户交互和进程提权。提权侧只应暴露很窄的命令，例如 `trust-root-ca`、`apply-hosts`、`restore-hosts`、`untrust-root-ca`。

## v0.6.1 行为

`cert.auto_install` 默认是 `true`。Hosts 模式下：

1. `start --mode hosts` 检查本项目 Root CA 是否已经受信。
2. 如果已受信，启动流程跳过证书安装。
3. 如果未受信且 `cert.auto_install` 为 true，启动流程通过 Windows 证书库 API 安装。
4. 如果 `cert.auto_install` 为 false，启动会停止并提示先执行 `cert install`。
5. hosts preflight、reverse proxy 监听、hosts 写入和 rollback 状态都在同一个启动流程中处理。

`status` 会打印 `system_change:` 行，调用方可以看到哪些系统动作已执行：

```text
system_change: component=root_ca action=install status=ok detail=installed
system_change: component=hosts action=preflight status=ok
system_change: component=reverse_proxy action=listen status=ok
system_change: component=hosts action=apply status=ok detail=entries=13
```

## 后续 Helper 契约

引入独立 helper 时，契约应保持很窄：

| 命令 | 输入 | 输出 | 说明 |
|---|---|---|---|
| `trust-root-ca` | 证书路径 / thumbprint | 信任结果 | 幂等 |
| `apply-hosts` | marker block entries 和 rollback path | 写入结果 | 只允许项目标记区块 |
| `restore-hosts` | rollback path | 恢复结果 | 只允许项目标记区块 |
| `untrust-root-ca` | thumbprint | 卸载结果 | 必须是显式用户动作 |

helper 不能接受任意 shell 命令、任意文件写入、代理凭据、Cookie 或用户秘密信息。
