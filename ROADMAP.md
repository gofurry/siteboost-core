# steam-accelerator-core 中文 Roadmap

> 状态日期：2026-06-21
> 主维护语言：中文
> 当前阶段：`v0.5.1-dev` 一键 Hosts + DoH 默认闭环已落地，已补出站失败诊断，后续进入真实 Steam profile 与验收
> Go module：`github.com/gofurry/go-steam-core`

## 当前定位

`steam-accelerator-core` 是一个 Go 版 Steam 本地网络加速原子能力内核。它不是完整 Steam++ / Watt Toolkit 替代品，也不覆盖账号、库存、成就、挂卡、游戏下载、UI 面板或公共代理节点。

项目最终目标是沉淀可被 SteamScope、steam-go、Go/Wails 桌面工具或本地 sidecar 复用的底层能力，并逐步朝 Steam++ 的“一键可用”本地加速体验靠齐。

当前 `v0.5.1-dev` 已经具备核心零件，并完成第一版 Hosts + DoH 默认闭环和出站失败诊断：

- 本地 HTTP Proxy / HTTPS CONNECT。
- PAC 模式与 System Proxy 模式。
- Windows-first Hosts 模式。
- 本地 Root CA 与动态站点证书。
- 本地 HTTP / HTTPS Reverse Proxy。
- DNS / DoH / 缓存。
- Direct / HTTP Proxy / SOCKS5 upstream。
- Engine 生命周期、rollback 状态与 loopback 控制接口。
- Hosts + Direct 模式默认使用内置 DoH 解析真实 Steam IP，避免读取本机 hosts 造成自绕回。
- `start --mode hosts` 已串联 Root CA、hosts 可读写、rollback 目录可写、反代监听和 hosts 写入失败恢复。
- `status` / `start` 输出会显示运行时 resolver 模式和 DoH servers。
- Reverse Proxy / Proxy 的 502 会显示裁剪后的出站失败摘要，Direct 出口可区分 DoH 解析、TCP 连接和 TLS 握手阶段。

但当前还不能称为完整 Steam++ 式“一键可用”体验。主要差距是：

- 真实 Steam 域名、Steam 客户端内置浏览器、网页登录、社区、商店、聊天、静态资源和 WebSocket 场景缺少系统化冒烟记录。
- hosts 文件仍只能覆盖 exact 域名，wildcard 规则需要后续 DNSIntercept 等高级模式。
- macOS / Linux Hosts 与证书安装仍未支持。
- 当前运行时实现仍主要位于 `internal/`，公共 Go API 尚未稳定。

## Steam++ 参考边界

Watt Toolkit / Steam++ 可以作为架构参考：它的本地网络加速思路包括本地反代、Hosts / PAC / System / DNSIntercept / VPN 等接管方式、本地证书、DNS / DoH、可选二级代理和脚本注入。

本项目采用 clean-room 边界：

- 可以学习公开文档、模式拆分、生命周期边界和安全恢复思路。
- 不复制、不翻译、不移植 SteamTools 的 C# 源码。
- 默认加速主线不依赖外部代理节点；HTTP / SOCKS5 upstream 只是可选增强。
- `v1.0.0` 先稳定 Hosts + DoH 一键闭环；DNSIntercept、VPN / TUN、JS 注入进入 `v1.x` 高级能力路线。

## 路线策略

项目后续目标从“核心模块实现完成”调整为“默认一键闭环可用”。

默认一键闭环定义：

```text
Steam 域名规则
        ↓
Hosts 指向本地
        ↓
本地 HTTPS Reverse Proxy
        ↓
本地 Root CA 动态证书
        ↓
DoH / 指定 DNS 解析真实 Steam IP
        ↓
Direct 直连真实 IP，保留原始 Host / SNI
        ↓
stop / restore 可恢复系统修改
```

排序原则：

1. 已先修正 Hosts 模式默认闭环和 DNS 自绕回风险。
2. 接下来用真实 Steam 场景验证规则、证书、反代和恢复路径。
3. 之后稳定跨平台能力和 Go 集成 API。
4. `v1.0.0` 以一键 Hosts + DoH 闭环作为稳定主线。
5. DNSIntercept、VPN / TUN、JS 注入进入 `v1.x` 高级能力路线，但不阻塞 `v1.0.0`。

## 已实现摘要

### v0.1.0 - ProxyOnly 加速内核

**状态：** 已完成
**范围：** Stability / Developer-facing / Testing / Documentation
**目标：** 完成最小可用本地代理核心，不修改系统、不安装证书。

已完成 Config、Steam 域名规则、HTTP Proxy、HTTPS CONNECT、Direct upstream、Engine 生命周期、CLI 基础命令和基础单元测试。

### v0.2.0 - Resolver / DoH / 上游出口

**状态：** 已完成
**范围：** Stability / User-facing / Performance / Testing
**目标：** 完成 DNS / DoH 与上游出口能力。

已完成 system / udp / tcp / doh resolver、DNS 缓存、超时、fallback、IPv4 / IPv6 策略、Direct / HTTP / SOCKS5 upstream 和相关测试。

### v0.3.0 - PAC 与 System Proxy

**状态：** 已完成
**范围：** User-facing / Safety / Cross-platform / Testing
**目标：** 支持 PAC 与系统代理接管，并保证可恢复。

已完成 PAC 生成器、本地 PAC Server、Windows / macOS PAC 写入与恢复、Windows / macOS HTTP / HTTPS 系统代理写入与恢复、rollback 状态文件与 `restore`。

### v0.4.0 - Hosts + HTTPS Reverse Proxy

**状态：** 已完成核心零件，等待闭环打磨
**范围：** Security-Safety / Architecture / User-facing / Testing
**目标：** 实现 Windows-first Hosts 模式下的本地 HTTPS 反代。

已完成 hosts patcher、备份、回滚、Root CA 生成和显式安装/卸载、动态站点证书、本地 HTTP / HTTPS Reverse Proxy、原始 Host / SNI 保留、WebSocket 支持、`start --mode hosts`。

已知限制：

- v0.4.0 为 Windows-first；macOS / Linux Hosts 与证书安装暂不支持。
- hosts 文件只写入 exact 域名，通配符完整覆盖留给后续 DNSIntercept。
- `restore` 只删除 hosts 项目标记区块，Root CA 由 `cert uninstall` 显式卸载。
- Hosts 模式默认闭环还没有强制避开 system resolver。

## 后续版本计划

### v0.5.0 - 一键 Hosts + DoH 默认闭环

**状态：** 已完成默认闭环代码，等待真实 Steam 场景验收
**范围：** User-facing / Stability / Security-Safety / Testing
**目标：** 把现有 Hosts 反代零件串成接近 Steam++ 的默认一键加速闭环。

#### 重点

- Hosts 模式出站解析默认避开 system resolver。
- DoH / 指定 DNS 成为 Hosts 模式默认真实域名解析路径。
- 一键启动前置检查和失败恢复。
- 高风险能力的用户提示和可恢复性。

#### 任务

- [x] 为 Hosts + Direct 模式增加专用解析策略，默认使用 DoH 解析真实 Steam IP，避免读取本机 hosts 导致自绕回。
- [x] 提供内置 DoH 默认服务器列表和 fallback 顺序，允许用户覆盖但不要求用户配置上游代理。
- [x] 将 `start --mode hosts` 的前置检查串联起来：Root CA 状态、hosts 可读写、rollback 目录可写、80 / 443 监听失败提示和 hosts 写入失败恢复。
- [x] 明确外部 HTTP / SOCKS5 upstream 是可选增强，不作为默认加速前提。
- [x] 改进 Hosts 模式启动失败时的错误信息和恢复建议。
- [x] 增加 Hosts + DoH 闭环单元测试，覆盖默认 resolver、状态输出和 hosts preflight。
- [x] 更新使用文档和冒烟文档，说明“一键加速”实际做了哪些本地系统修改。

#### 验收标准

- 默认 Hosts 模式不会因为本机 hosts 写入而把出站连接解析回 `127.0.0.1`。
- 未配置上游代理时，仍可通过本地 DoH + 直连真实 IP 完成 Steam 规则域名反代。
- 启动前置检查失败时有明确中文提示，不留下不可恢复的系统状态。
- `stop` / `restore` 可以恢复 hosts 与代理相关状态，Root CA 仍由 `cert uninstall` 显式卸载。
- `go test ./...` 通过，并覆盖 Hosts 解析闭环关键路径。

---

### v0.5.1 - 出站失败诊断补丁

**状态：** 已完成
**范围：** User-facing / Stability / Testing / Diagnostics
**目标：** 把 `upstream request failed` 从黑盒错误改成可定位的出站失败链路，为 v0.6 真实 Steam profile 做基础。

#### 重点

- 让用户能区分 DoH 解析失败、TCP 连接失败和 TLS 握手失败。
- 让日志包含目标 host、候选 IP、逐次失败原因和安全裁剪后的错误摘要。
- HTTPS 反代在 Direct 出口下按候选 IP 执行 TCP + TLS 尝试，避免“第一个 IP TCP 连上但 TLS 失败后不再尝试”的行为。
- 仍不把外部 HTTP / SOCKS5 upstream 写成默认前提。

#### 任务

- [x] 为 Direct upstream 增加结构化出站错误，包含目标 host、port、解析错误、候选 IP 和逐次失败原因。
- [x] 为 HTTPS Reverse Proxy 接入 TLS-aware direct dial，对每个候选 IP 执行 TCP + TLS 握手并记录失败阶段。
- [x] 在 Reverse Proxy / Proxy 502 响应和日志中输出安全裁剪后的出站错误摘要。
- [x] 增加 direct dial、reverse 502 和 proxy 502 诊断测试。
- [x] 将版本推进到 `v0.5.1-dev` 并同步 smoke/usage 文档中的诊断说明。

#### 验收标准

- 用户再次看到 `upstream request failed` 时，可以在响应体或日志中看到具体失败原因。
- Direct 出口失败能显示 DoH 解析失败或每个候选 IP 的 TCP / TLS 失败阶段。
- HTTPS 反代不会因为单个候选 IP TLS 失败就直接丢失后续 IP 尝试机会。
- `go test ./...` 和 `go vet ./...` 通过。

---

### v0.6.0 - 真实 Steam 访问验收与规则完善

**状态：** 计划中
**范围：** User-facing / Testing / Documentation / Stability
**目标：** 用真实 Steam 访问场景验证一键闭环，补齐 Steam 默认出站 profile、域名规则和手动验收记录。

#### 重点

- Steam 商店、社区、登录、聊天、静态资源和 WebSocket 场景。
- 规则覆盖率与 hosts exact 域名清单。
- Steam 默认出站 profile：候选 IP、ForwardDestination、TLS / SNI 策略和失败 fallback。
- 真实环境 smoke test。
- 基于 v0.5.1 诊断输出定位真实网络失败。

#### 任务

- [ ] 建立真实 Steam 域名兼容性清单，覆盖 store、community、login、api、chat、static、cdn 等分组。
- [ ] 为 Hosts 模式维护 exact 域名写入清单，明确 wildcard 规则无法通过 hosts 覆盖的范围。
- [ ] 为核心 Steam 域名设计默认出站 profile，支持候选 IP、ForwardDestination、TLS / SNI pattern、证书名不匹配策略和 fallback 顺序。
- [ ] 增加启动前真实探测：DoH 解析、TCP 443 连通、TLS 握手和轻量 HTTP smoke。
- [ ] 增加真实 Steam 访问手动 smoke 文档，记录 Windows 管理员终端、证书安装、启动、访问、停止、恢复全过程。
- [ ] 增加常见失败诊断文档：DNS 失败、证书不受信、端口占用、hosts 被安全软件拦截、WebSocket 失败。
- [ ] 增加代理请求、反代请求和 resolver 失败的脱敏日志字段扩展，确保不泄露 Cookie / Authorization / URL secret。
- [ ] 评估是否需要内置规则版本号和规则更新时间。

#### 验收标准

- 至少完成一轮 Windows 真实 Steam 商店 / 社区 / 登录 / 静态资源 / WebSocket 手动验收记录。
- 默认规则能覆盖最常见的 Steam Web 访问链路。
- 未覆盖域名和失败原因可以从日志或 status 输出中定位。
- 文档明确区分“本地反代加速”和“外部代理节点加速”。

---

### v0.7.0 - 跨平台闭环与集成 API 候选

**状态：** 计划中
**范围：** Cross-platform / Developer-facing / API / Documentation
**目标：** 将一键闭环从 Windows-first 扩展为可评估的跨平台能力，并形成 Go 集成 API 候选。

#### 重点

- macOS / Linux Hosts 与证书安装评估。
- 权限提升和恢复策略。
- Engine / Config / Mode API 候选。
- Wails / sidecar 集成建议。

#### 任务

- [ ] 评估并实现 macOS Hosts 写入、Root CA 安装/卸载、系统代理恢复路径。
- [ ] 评估 Linux 桌面环境 hosts、证书信任、权限提升和发行版差异，明确支持边界。
- [ ] 设计公共 Go API 候选，稳定 Engine、Config、Mode、Status、Restore 的调用边界。
- [ ] 增加 package 集成示例和 Wails / sidecar 调用建议。
- [ ] 为系统修改状态设计兼容的 rollback 文件版本策略。
- [ ] 增加跨平台 smoke 文档和 unsupported 场景提示。

#### 验收标准

- Windows 一键闭环保持稳定。
- macOS / Linux 支持范围、限制和失败提示明确。
- 公共 API 候选可被上层工具调用，不要求直接引用 `internal/`。
- 文档能指导桌面端集成启动、状态、停止、恢复和证书管理。

---

### v1.0.0 - 一键闭环稳定发布

**状态：** 计划中
**范围：** Release / API / Stability / Documentation
**目标：** 发布第一个稳定版本，以 Hosts + DoH 一键本地加速闭环作为主线能力。

#### 重点

- API freeze。
- CLI 和 Go package 双入口稳定。
- release notes 与安全边界。
- 可重复的一键验收流程。

#### 任务

- [ ] 冻结 Engine API、Config 结构、Mode 枚举和 Status 输出。
- [ ] 完成 CLI 使用示例、Go package 示例和 Wails 集成建议。
- [ ] 完成安全边界说明：Root CA、hosts、系统代理、DoH、日志脱敏、restore。
- [ ] 完成 CHANGELOG、release notes、版本标签和发布检查清单。
- [ ] 完成 Windows 一键 Hosts + DoH 闭环最终验收。
- [ ] 保留 ProxyOnly、PAC、System Proxy 作为可选模式和调试模式。

#### 验收标准

- 用户可以按文档完成一键启动、访问 Steam、停止和恢复。
- 默认不需要配置外部上游代理。
- 公共 API、CLI 行为和配置格式在 v1 期间保持兼容。
- 没有已知会破坏系统代理、hosts 或证书恢复的阻塞问题。

---

### v1.1.0 - DNSIntercept 高级模式

**状态：** 计划中
**范围：** Windows / Advanced / Security-Safety / Architecture
**目标：** 在 v1 稳定主线之后评估并实现 Windows DNSIntercept，补齐 hosts 无法覆盖 wildcard 的接管能力。

#### 任务

- [ ] 设计 DNSIntercept 架构和权限模型。
- [ ] 评估 WinDivert / WFP 等方案的许可证、分发、安全和维护成本。
- [ ] 增加 DNSIntercept 与 Hosts 模式的切换和恢复策略。
- [ ] 增加 Windows 专项测试和故障恢复文档。

#### 验收标准

- DNSIntercept 不影响 v1.0 Hosts + DoH 稳定主线。
- 启停失败可恢复，不破坏系统 DNS 状态。
- wildcard 域名覆盖能力比 Hosts 模式更完整。

---

### v1.2.0 - VPN / TUN 高级接管模式

**状态：** 计划中
**范围：** Advanced / Cross-platform / Architecture / Security-Safety
**目标：** 评估并实现更强的流量接管模式，用于 Hosts、PAC、System Proxy 无法覆盖的场景。

#### 任务

- [ ] 设计 TUN / VPN 模式的最小可用边界和非目标。
- [ ] 评估路由、DNS、虚拟网卡、权限、退出恢复和跨平台分发成本。
- [ ] 增加与现有 resolver / upstream / rules 的复用方案。
- [ ] 增加单独的安全提示、诊断和恢复命令。

#### 验收标准

- VPN / TUN 模式作为显式高级模式启用，不改变默认一键 Hosts + DoH 主线。
- 异常退出后有可执行的恢复路径。
- 文档明确风险和平台支持范围。

---

### v1.3.0 - JS 注入与页面增强能力

**状态：** 计划中
**范围：** Advanced / User-facing / Security-Safety
**目标：** 在用户显式开启的前提下，评估类似 Steam++ 的脚本注入和页面增强能力。

#### 任务

- [ ] 设计 JS 注入的安全边界、启用开关、脚本来源和匹配规则。
- [ ] 明确默认关闭，不进入核心加速必需链路。
- [ ] 增加响应体修改、压缩、编码、CSP、Content-Length、缓存的测试。
- [ ] 增加脚本管理和安全提示文档。

#### 验收标准

- 默认一键加速不依赖 JS 注入。
- 用户能明确知道 HTTPS 内容会被本地修改。
- 注入失败不影响基础反代访问。

## 短中长期方向

短期：

- 聚焦 `v0.6.0`，用真实 Steam 场景验证 Hosts + DoH 默认闭环，补齐规则、诊断和冒烟记录。

中期：

- 用 `v0.6.0` 和 `v0.7.0` 完成真实 Steam 验收、规则维护、跨平台评估和 Go 集成 API 候选。

长期：

- `v1.0.0` 稳定发布后，在 `v1.x` 中逐步加入 DNSIntercept、VPN / TUN、JS 注入等高级 Steam++ 能力。

## 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| Hosts 模式 system resolver 自绕回 | 反代连接回本机导致访问失败 | v0.5 强制 Hosts 出站解析避开 system resolver |
| Root CA 用户不信任 | 安全信任问题 | 默认明确提示、显式安装、显式卸载、文档说明 |
| hosts 写入失败 | 用户网络异常 | 标记区块、rollback、restore、管理员权限检查 |
| 80 / 443 端口占用 | 一键启动失败 | 启动前检查并提示占用进程或替代测试端口 |
| Steam 域名变化 | 覆盖不足 | 规则分组、手动 smoke、规则版本化评估 |
| DNS / DoH 服务失效 | 加速不可用 | 多服务器 fallback、用户可覆盖配置 |
| 高级模式复杂度过高 | v1.0 被拖慢 | DNSIntercept / VPN / JS 注入放入 v1.x，不阻塞 v1.0 |
| 复制 SteamTools 代码 | GPL 和维护边界风险 | 坚持 clean-room，只参考架构思想 |
