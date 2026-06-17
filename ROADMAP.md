# steam-accelerator-core 中文 Roadmap

> 状态日期：2026-06-17  
> 主维护语言：中文  
> 当前阶段：仓库脚手架 / v0.1.0 开发准备  
> Go module：`github.com/gofurry/go-steam-core`

## 当前定位

steam-accelerator-core 是一个 Go 版 Steam 本地网络加速原子能力内核。它不是完整 Steam++ / Watt Toolkit 替代品，也不覆盖账号、库存、成就、挂卡、游戏下载、UI 面板或公共代理节点。

项目目标是沉淀可被 SteamScope、steam-go、Go/Wails 桌面工具或本地 sidecar 复用的底层能力：

- 本地 HTTP Proxy / HTTPS CONNECT。
- PAC 模式。
- System Proxy 模式。
- Hosts 模式。
- 本地 CA 与动态证书。
- HTTPS 本地反向代理。
- DNS / DoH / 缓存。
- Direct / HTTP Proxy / SOCKS5 上游出口。
- 可启动、停止、恢复的 Engine 生命周期。

## SteamTools 参考边界

Watt Toolkit / SteamTools 可以作为架构参考：它的公开 README 将网络加速描述为使用 YARP 进行本地反代，源码中的代理模式枚举包含 `DNSIntercept`、`Hosts`、`System`、`VPN`、`ProxyOnly`、`PAC`。

本项目采用 clean-room 边界：

- 可以学习公开文档、模式拆分、生命周期边界和安全恢复思路。
- 不复制、不翻译、不移植 SteamTools 的 C# 源码。
- 不把 JS 注入作为加速核心能力。
- 首期不做 `DNSIntercept`、VPN / TUN、驱动级流量接管。
- 代码以 Go 标准库、独立开源库和自研逻辑为主。

## 路线策略

优先级：先完成不会修改系统状态的代理核心，再引入 PAC 和系统代理，最后处理 Hosts、证书、HTTPS 反代等高权限高风险能力。

排序原则：

1. 先 ProxyOnly，后 PAC / System，再 Hosts 反代。
2. 先不解密 HTTPS，后提供高级模式。
3. 默认只监听 `127.0.0.1`。
4. 默认只代理 Steam 规则域名。
5. 所有系统修改都必须可恢复。
6. 所有敏感信息都不能进入日志。
7. `v1.0.0` 只用于稳定 API 与可集成发布。

## 版本计划

### v0.1.0 - ProxyOnly 加速内核

**状态：** 开发中  
**范围：** Stability / Developer-facing / Testing / Documentation  
**目标：** 完成最小可用的本地代理核心，不修改系统、不安装证书。

#### 重点

- 仓库基础结构。
- Engine 生命周期。
- Steam 域名规则。
- HTTP Proxy / HTTPS CONNECT。
- Direct upstream。
- CLI 最小命令。
- 基础测试与 CI。

#### 任务

- [x] 建立 Go module、README、CI、基础示例与文档结构。
- [x] 明确 SteamTools 参考边界和 clean-room 约束。
- [ ] 实现 Config 默认值、加载与校验。
- [ ] 实现 Steam 默认域名规则。
- [ ] 实现 `rules.Matcher`，支持精确、通配符和后缀匹配。
- [ ] 实现 HTTP Proxy 基础框架。
- [ ] 实现 HTTPS CONNECT 隧道。
- [ ] 实现 Direct upstream。
- [ ] 实现基础结构化日志。
- [ ] 实现 Engine `Start` / `Stop` / `Status`。
- [ ] 提供 CLI：`start --mode proxy-only`、`stop`、`status`。
- [ ] 添加 rules、proxy、engine 单元测试。

#### 非目标

- 不做 PAC。
- 不修改系统代理。
- 不修改 hosts。
- 不生成或安装本地 CA。
- 不做 HTTPS 反代。
- 不做 JS 注入。

#### 验收标准

- 默认监听 `127.0.0.1:26501`。
- 浏览器手动配置 HTTP Proxy 后，Steam 规则域名可通过本地代理转发。
- 非 Steam 域名默认不进入加速链路。
- `stop` 后端口释放。
- `go test ./...`、`go vet ./...`、`gofmt` 检查通过。

---

### v0.2.0 - Resolver / DoH / 上游代理

**状态：** 计划中  
**范围：** Stability / User-facing / Performance / Testing  
**目标：** 完成 DNS / DoH 与上游出口能力，让代理核心具备真实可配置性。

#### 重点

- Resolver 抽象。
- 系统 DNS、UDP DNS、TCP DNS、DoH。
- DNS 缓存与 fallback。
- IPv4 / IPv6 策略。
- HTTP / SOCKS5 上游代理。

#### 任务

- [ ] 实现 `Resolver` 接口。
- [ ] 实现 system / udp / tcp / doh resolver。
- [ ] 实现 DNS 缓存、超时和备用 DNS fallback。
- [ ] 实现 IPv4 / IPv6 偏好与禁用策略。
- [ ] 实现 Direct / HTTP Proxy / SOCKS5 upstream。
- [ ] 支持代理认证配置并避免日志泄露密码。
- [ ] 在 proxy Dial 链路中接入 resolver + upstream。
- [ ] 添加 resolver、upstream 单元测试。

#### 非目标

- 不做节点测速。
- 不做本机代理端口扫描。
- 不做 DNSIntercept。

#### 验收标准

- 可通过配置切换 system / udp / tcp / doh。
- DoH 或 DNS 失败时可 fallback。
- 支持通过用户配置的 HTTP / SOCKS5 上游转发 Steam 流量。
- 日志不输出代理密码、Cookie、Authorization 或完整 token。

---

### v0.3.0 - PAC 与 System Proxy

**状态：** 计划中  
**范围：** User-facing / Safety / Cross-platform / Testing  
**目标：** 支持通过 PAC 或系统代理接管 Steam 相关域名流量，并保证可恢复。

#### 重点

- PAC 生成器。
- 本地 PAC Server。
- Windows / macOS 系统 PAC 写入与恢复。
- Windows / macOS HTTP / HTTPS 系统代理写入与恢复。
- rollback 状态文件。
- `restore` 命令。

#### 任务

- [ ] 实现 PAC 生成器，规则来源统一来自 `rules` 模块。
- [ ] 实现 PAC Server，默认监听 `127.0.0.1`。
- [ ] 实现 `start --mode pac`。
- [ ] 实现 Windows 系统 PAC 写入与恢复。
- [ ] 实现 macOS 系统 PAC 写入与恢复。
- [ ] 实现 Windows 系统 HTTP / HTTPS 代理写入与恢复。
- [ ] 实现 macOS 系统 HTTP / HTTPS 代理写入与恢复。
- [ ] 实现 rollback 状态文件。
- [ ] 实现 `restore` 命令。
- [ ] 添加 PAC 与 System Proxy 集成测试。

#### 非目标

- Linux 桌面环境系统代理首期只提供文档，不强行统一。
- 不做 hosts。
- 不做本地 CA。
- 不做 DNSIntercept。

#### 验收标准

- PAC 文件只命中 Steam 规则域名。
- 开启 PAC 后系统 PAC 指向本地 PAC Server。
- 开启 System Proxy 后系统 HTTP / HTTPS 代理指向本地代理。
- 停止后恢复原系统代理配置。
- 模拟崩溃后执行 `restore` 可以恢复系统代理。

---

### v0.4.0 - Hosts + HTTPS Reverse Proxy

**状态：** 计划中  
**范围：** Security/Safety / Architecture / User-facing / Testing  
**目标：** 实现接近 Steam++ 体验的本地反代模式，同时把证书和 hosts 风险控制在显式启用边界内。

#### 重点

- hosts patcher。
- hosts 备份与恢复。
- Root CA 生成、安装、卸载。
- 动态站点证书签发。
- 本地 HTTP / HTTPS Server。
- HTTPS Reverse Proxy。
- 原始 Host 与 SNI 保留。
- WebSocket 支持。

#### 任务

- [ ] 实现 hosts patcher，只管理项目标记区块。
- [ ] 实现 hosts 备份、失败回滚与 `restore` 恢复。
- [ ] 实现 Root CA 生成。
- [ ] 实现 Root CA 安装与卸载。
- [ ] 实现动态站点证书签发与缓存。
- [ ] 实现本地 HTTP Server。
- [ ] 实现本地 HTTPS Server。
- [ ] 实现 HTTPS Reverse Proxy。
- [ ] 保留原始 Host 与 TLS SNI。
- [ ] 支持 WebSocket 升级。
- [ ] 实现 `start --mode hosts`。
- [ ] 实现 `cert install` / `cert uninstall`。
- [ ] 添加 hosts、cert、reverse 集成测试。

#### 非目标

- 不做 JS 注入。
- 不修改响应体。
- 不做 DNSIntercept。
- 不做 VPN / TUN。

#### 验收标准

- hosts 中只写入项目标记区块。
- 停止后完整移除项目标记区块。
- 本地 CA 安装后，规则域名证书链可被系统信任。
- 反代出口仍使用真实 Steam 域名作为 SNI。
- `restore` 可恢复 hosts 与证书相关状态。

---

### v0.5.0 - 稳定性、安全与跨平台打磨

**状态：** 计划中  
**范围：** Stability / Security/Safety / CI/Release / Documentation  
**目标：** 将 v0.1 到 v0.4 的能力打磨为可集成、可恢复、可诊断的 core。

#### 重点

- 错误码与错误信息。
- 结构化日志。
- 活跃连接统计与优雅关闭。
- 配置校验。
- 安全文档与恢复文档。
- 权限说明。
- CI 与 benchmark。

#### 任务

- [ ] 完善错误码和用户可理解错误信息。
- [ ] 完善结构化日志与敏感信息脱敏规则。
- [ ] 增加连接数统计。
- [ ] 增加 active connection graceful shutdown。
- [ ] 增加配置校验。
- [ ] 增加安全说明文档。
- [ ] 增加崩溃恢复文档。
- [ ] 增加 Windows 管理员权限说明。
- [ ] 增加 macOS 权限说明。
- [ ] 完善 CI。
- [ ] 评估 staticcheck 引入时机。
- [ ] 添加基础 benchmark。

#### 验收标准

- 常见错误有明确提示。
- 启动、停止、恢复流程稳定。
- 所有系统修改都有回滚路径。
- README 能指导用户理解每种模式的风险。
- 基础 benchmark 可用于后续性能回归比较。

---

### v1.0.0 - 稳定 API 与集成发布

**状态：** 计划中  
**范围：** API / Release / Documentation / Integration  
**目标：** 形成稳定 API，允许 SteamScope、steam-go、Wails 桌面端或本地 sidecar 集成。

#### 重点

- Engine API 稳定。
- Config 结构稳定。
- Mode 枚举稳定。
- Go package 示例。
- CLI 示例。
- Wails 集成建议。
- 安全边界说明。
- CHANGELOG 与 release notes。

#### 任务

- [ ] 稳定 Engine API。
- [ ] 稳定 Config 结构。
- [ ] 稳定 Mode 枚举。
- [ ] 提供 Go package 集成示例。
- [ ] 提供 CLI 使用示例。
- [ ] 提供 Wails 集成建议。
- [ ] 提供安全边界说明。
- [ ] 提供 CHANGELOG。
- [ ] 发布 `v1.0.0`。

#### 验收标准

- 可作为 Go library 使用。
- 可作为 CLI 使用。
- 可被 Wails UI 或本地桌面工具调用。
- 文档覆盖 ProxyOnly、PAC、System、Hosts 模式。
- 中文 Roadmap 与 CHANGELOG 完整。

## 短中长期方向

短期：

- 完成 v0.1.0 的 ProxyOnly MVP。
- 把 rules、proxy、engine 的测试先补起来。
- 保持 CI 和文档同步更新。

中期：

- 完成 resolver、DoH、upstream。
- 完成 PAC / System Proxy 与 rollback。
- 建立跨平台恢复策略。

长期：

- 在 v0.4 后再启用 Hosts + HTTPS 反代。
- 在 v0.5 进行安全、稳定性、权限和文档打磨。
- 进入 `v1.0.0-alpha.x` 前冻结公共 API 候选。

## 延后路线

以下内容不进入 v1.0：

- `DNSIntercept`：价值高，但 Windows 专项、权限和驱动风险高。
- VPN / TUN：接管能力强，但复杂度接近 VPN 客户端。
- JS 注入：与“加速原子能力”定位不一致，安全争议和维护成本高。
- 公共代理池或节点服务：超出本地 core 边界。

## 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| hosts 修改失败 | 用户网络异常 | 标记区块、备份、事务化写入、restore |
| 系统代理恢复失败 | 影响全局网络 | 保存原值、单独 `restore` 命令 |
| 本地 CA 引发用户不信任 | 安全信任问题 | 默认关闭、明确说明、可卸载 |
| 非 Steam 流量被代理 | 隐私风险 | 默认只代理规则域名 |
| 端口冲突 | 启动失败 | 明确报错，允许配置端口 |
| DNS / DoH 服务失效 | 加速不可用 | 多服务器 fallback |
| 复制 SteamTools 代码 | GPL 和维护边界风险 | clean-room 实现 |
| Steam 域名变化 | 加速覆盖不足 | 维护 rules，允许用户扩展 |

## 最小可用定义

MVP 是 ProxyOnly 模式，而不是 Hosts 反代：

- 本地启动 HTTP Proxy。
- 支持 HTTPS CONNECT。
- 只代理 Steam 规则域名。
- 支持 Direct upstream。
- 支持配置文件。
- 支持启动、停止、状态查询。
- 有基础测试。

完成 MVP 后，再进入 PAC、System Proxy、Hosts 反代。
