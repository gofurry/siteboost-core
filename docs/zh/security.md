# 安全说明

## 默认安全边界

steam-accelerator-core 面向本地运行，默认安全边界如下：

- 默认只监听 `127.0.0.1`。
- 默认只代理 Steam 规则域名。
- ProxyOnly 模式不解密 HTTPS。
- 默认不安装本地 Root CA，必须显式执行 `cert install`。
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

## 证书风险

安装本地 Root CA 意味着本项目可以为规则域名签发本地站点证书。该能力只应在 Hosts + HTTPS Reverse Proxy 模式中使用，并且默认关闭。

项目必须提供：

- `cert install`。
- `cert uninstall`。
- `restore`。
- 明确的风险提示。

`restore` 只恢复系统代理或删除 hosts 项目标记区块，不会自动卸载用户显式安装的 Root CA。

v0.4.0 的证书安装范围是 Windows 当前用户 Root store；macOS / Linux 会明确返回 unsupported。

## SteamTools 边界

本项目参考 SteamTools 的网络加速架构思想，但不复制、不翻译、不移植其源码。详见 [SteamTools 参考边界](./steamtools-reference.md)。
