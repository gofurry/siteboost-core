# 中文 Roadmap

中文主路线图维护在仓库根目录的 [ROADMAP.md](../../ROADMAP.md)。本文件作为 `docs/zh/` 目录下的同步入口，保留当前阶段、策略和版本摘要，便于从文档目录导航。

## 当前阶段

当前 `v0.4.0` 本地加速内核已实现：支持 ProxyOnly、PAC、System Proxy、Windows Hosts 反代、YAML 配置、Steam 域名规则、HTTP Proxy、HTTPS CONNECT、可配置 resolver、DNS 缓存、IPv4 / IPv6 策略、Direct / HTTP / SOCKS5 upstream、Root CA 生成与显式安装、动态站点证书、rollback 状态、带 token 的 loopback 控制接口，以及 `start` / `status` / `stop` / `restore` / `cert install` / `cert uninstall` CLI。

## 路线策略

先完成不修改系统状态的 ProxyOnly 核心，再进入 PAC / System Proxy，最后处理 Hosts、Root CA、动态证书和 HTTPS Reverse Proxy。默认只监听 `127.0.0.1`，默认只代理 Steam 规则域名，所有系统修改都必须可恢复。

## 版本摘要

### v0.1.0 - ProxyOnly 加速内核

**状态：** 已完成  
**目标：** 完成最小可用的本地代理核心，不修改系统、不安装证书。

已完成：

- [x] 建立仓库基础结构。
- [x] 实现 Config、rules、HTTP Proxy、HTTPS CONNECT、Direct upstream。
- [x] 实现 Engine `Start` / `Stop` / `Status`。
- [x] 提供 `start --mode proxy-only`、`stop`、`status`。
- [x] 添加 config、rules、proxy、engine、runtime/control 单元测试。

验收标准：

- 默认监听 `127.0.0.1:26501`。
- Steam 规则域名可通过手动代理访问。
- 非 Steam 域名默认拒绝，可配置为 direct。
- `gofmt`、`go vet`、`go test ./...` 通过。

### v0.2.0 - Resolver / DoH / 上游代理

**状态：** 已完成
**目标：** 完成 DNS / DoH 与上游出口能力。

任务摘要：

- [x] 支持 system / udp / tcp / doh resolver。
- [x] 支持 DNS 缓存、超时、fallback。
- [x] 支持 IPv4 / IPv6 策略。
- [x] 支持 HTTP Proxy / SOCKS5 upstream。
- [x] 添加 resolver、upstream 测试。

验收标准：

- 可通过配置切换 resolver。
- DNS 失败可 fallback。
- 代理密码和敏感头不会进入日志。

### v0.3.0 - PAC 与 System Proxy

**状态：** 已完成
**目标：** 支持 PAC 与系统代理接管，并保证可恢复。

任务摘要：

- [x] PAC 从 rules 模块生成。
- [x] 本地 PAC Server。
- [x] Windows / macOS PAC 写入与恢复。
- [x] Windows / macOS HTTP / HTTPS 系统代理写入与恢复。
- [x] rollback 状态文件与 `restore`。

### v0.4.0 - Hosts + HTTPS Reverse Proxy

**状态：** 已完成
**目标：** 实现 Hosts 模式下的本地 HTTPS 反代。

任务摘要：

- [x] hosts patcher、备份、回滚。
- [x] Root CA 生成、安装、卸载。
- [x] 动态站点证书签发。
- [x] HTTP / HTTPS Reverse Proxy。
- [x] 保留原始 Host 与 SNI。
- [x] 支持 WebSocket。

说明：

- v0.4.0 为 Windows-first；macOS / Linux Hosts 与证书安装暂不支持。
- hosts 文件只写入 exact 域名，通配符完整覆盖留给后续 DNSIntercept。
- `restore` 只删除 hosts 项目标记区块，Root CA 由 `cert uninstall` 显式卸载。

### v0.5.0 - 稳定性、安全与跨平台打磨

**状态：** 计划中  
**目标：** 将前序能力打磨为可集成、可恢复、可诊断的 core。

任务摘要：

- [ ] 错误码与错误信息。
- [ ] 结构化日志和敏感信息脱敏。
- [ ] 优雅关闭与连接统计。
- [ ] 安全文档、恢复文档、权限说明。
- [ ] CI 与基础 benchmark。

### v1.0.0 - 稳定 API 与集成发布

**状态：** 计划中  
**目标：** 形成稳定 API，允许 SteamScope、steam-go、Wails 或本地 sidecar 集成。

任务摘要：

- [ ] 稳定 Engine API、Config、Mode。
- [ ] 提供 Go package、CLI、Wails 集成示例。
- [ ] 完成安全边界、CHANGELOG 与 release notes。
- [ ] 发布 `v1.0.0`。

## 延后路线

- `DNSIntercept`：v1.0 后再评估。
- VPN / TUN：除非项目成熟且需求明确，否则不做。
- JS 注入：不进入加速核心。
- 公共代理池：超出本地 core 边界。
