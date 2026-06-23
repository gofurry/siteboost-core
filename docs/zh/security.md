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
- 默认不修改系统 DNS；只有显式 `mode: dns` 且 `dns_intercept.strategy: system` 时才会接管指定 Windows 网卡 DNS。
- 默认不修改响应内容；只有显式配置 `page_enhance.enabled: true` 时才会执行页面增强 transform。
- 默认不暴露公网代理入口。
- 日志不记录 Cookie、Authorization、代理密码、token 或完整敏感 URL。

## 高风险模式

以下模式会修改系统状态或信任状态，必须显式启用：

- PAC。
- System Proxy。
- Hosts。
- DNSIntercept system。
- 本地 Root CA 安装。
- HTTPS Reverse Proxy。

这些模式必须满足：

- 修改前记录 rollback 状态。
- 停止时尝试恢复。
- 支持独立 `restore`。
- 文档写明手动恢复方式。
- 只修改项目拥有的配置项或 hosts 标记区块。
- AppHost 必须保持窄命令面。v0.7.3-dev AppHost 只接受 `prepare-hosts-start`、`trust-root-ca`、`restore-hosts`、`untrust-root-ca`、`preflight-system-dns`、`apply-system-dns`、`restore-system-dns` 和 health 请求，通过 Windows named pipe 传输，带 DACL，平台支持时启用 `PIPE_REJECT_REMOTE_CLIENTS`，随机 token 非空校验、pipe client PID 与请求父进程 PID 绑定、客户端二进制路径校验，限制默认 hosts 路径、loopback system DNS server、显式 interface selector 与项目 runtime / cert 目录，并对请求设置超时。

## Page Enhance 风险

Page Enhance 不属于系统修改能力，但会修改 reverse proxy 返回给浏览器的响应内容，因此必须显式启用并保持可观察：

- 默认 `page_enhance.enabled: false`。
- 只执行显式 YAML transform 或显式注册的 Go transformer。
- 不内置隐藏的 login、checkout 或敏感路径跳过规则；是否注入由开发者配置决定。
- 对 body 过大、不支持的 `Content-Encoding`、缺少注入锚点、replace 未命中或 transform 错误输出明确 reason。
- 关闭 `page_enhance.enabled` 或移除 transform 后恢复原始响应行为。
- Page Enhance 不写系统 DNS、hosts、证书信任、浏览器配置或开发者环境。

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
