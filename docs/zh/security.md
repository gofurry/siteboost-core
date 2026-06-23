# 安全说明

## 默认安全边界

steam-accelerator-core 面向本地运行，默认安全边界如下：

- 默认只监听 `127.0.0.1`。
- 默认只代理 Steam 规则域名。
- ProxyOnly 模式不解密 HTTPS。
- 只在用户显式启动 Hosts 模式或执行 `cert install` 时安装本地 Root CA。
- 只在用户显式启动 Hosts 模式、执行 `restore` 或执行证书命令且确实需要系统写入时请求 Windows UAC。
- `start --mode hosts` 在 `cert.auto_install` 为 true 时会把 Root CA 检查/安装纳入启动流程，并限制在配置的 Windows Root store。默认 `cert.store_scope: machine`；普通 Windows 进程需要系统写入时会通过已安装的 AppHost named pipe 完成，`user` 作为兼容退路。核心不会绕过 UAC、企业策略，也不会接受任意系统修改命令。
- `cert install` 和自动安装流程都会先检查本项目 Root CA 是否已存在，避免重复执行安装动作。
- 默认不修改 hosts，必须显式启动 `mode: hosts`。
- 默认不暴露公网代理入口。
- 日志不记录 Cookie、Authorization、代理密码、token 或完整敏感 URL。

## 高风险模式

以下模式会修改系统状态或信任状态，必须显式启用：

- PAC。
- System Proxy。
- Hosts。
- 本地 Root CA 安装。
- HTTPS Reverse Proxy。

这些模式必须满足：

- 修改前记录 rollback 状态。
- 停止时尝试恢复。
- 支持独立 `restore`。
- 文档写明手动恢复方式。
- 只修改项目拥有的配置项或 hosts 标记区块。
- AppHost 必须保持窄命令面。v0.7.0-dev AppHost 仍只接受 `prepare-hosts-start`、`trust-root-ca`、`restore-hosts`、`untrust-root-ca` 和 health 请求，通过 Windows named pipe 传输，带 DACL，平台支持时启用 `PIPE_REJECT_REMOTE_CLIENTS`，随机 token 非空校验、pipe client PID 与请求父进程 PID 绑定、客户端二进制路径校验，限制默认 hosts 路径与项目 runtime / cert 目录，并对请求设置超时。

## 证书风险

安装本地 Root CA 意味着本项目可以为规则域名签发本地站点证书。该能力只应在 Hosts + HTTPS Reverse Proxy 模式中使用，并且必须由用户显式启动 Hosts 模式或执行证书命令触发。

项目必须提供：

- `cert install`。
- `cert uninstall`。
- `restore`。
- 明确的风险提示。

`restore` 只恢复系统代理或删除 hosts 项目标记区块，不会自动卸载用户显式安装的 Root CA。

v0.6.4 的证书安装默认范围是 Windows `LocalMachine\Root`；普通 PowerShell 通过已安装的 AppHost Service 完成机器级写入，管理员 PowerShell 走直接路径；`cert.store_scope: user` 可切回当前用户 Root store。macOS / Linux 会明确返回 unsupported。

## SteamTools 边界

本项目参考 SteamTools 的网络加速架构思想，但不复制、不翻译、不移植其源码。详见 [SteamTools 参考边界](./steamtools-reference.md)。
