# SiteBoost Core 中文 Roadmap

> 状态日期：2026-06-23
> 当前仓库：`gofurry/siteboost-core`
> 当前 Go module：`github.com/gofurry/go-steam-core`
> 当前 CLI / ProjectName：`steam-accelerator` / `steam-accelerator-core`
> 当前代码阶段：`version.go` 已进入 `v0.6.4-dev`，主干包含 Windows AppHost Service 自动启动与 named pipe IPC 能力

## 当前定位

这个仓库是本地站点加速核心的实验性验证仓库。它当前以 Steam 为唯一真实落地目标，用来验证 Steam++ / Watt Toolkit 式本地加速闭环：规则接管、本地反代、Root CA、DoH、出站 profile、权限编排和恢复。后续会在本仓库内继续实验多站点 provider 形态，例如先用 GitHub skeleton 验证扩展边界，但本仓库不计划直接改造成正式 Go 开源库。

正式的通用 Go 加速库会在功能和架构验证后另起新仓库维护。届时可以直接借鉴、拆分或复用本仓库中已经验证过的核心内容，包括 resolver、reverse proxy、certstore、privilege/AppHost、provider 规则模型和 diagnostics。本仓库的主要价值是作为实验场、行为样板、smoke 记录和迁移来源，而不是最终稳定库本身。

项目参考 Steam++ / Watt Toolkit 的本地加速架构思想：本地反代、Hosts / PAC / System Proxy 接管、本地 Root CA、DNS / DoH、可选上游代理、提权 Root/AppHost 进程和恢复机制。项目保持 clean-room 边界：只参考架构思想和公开行为，不复制、不翻译、不移植 SteamTools 源码。

默认加速路线不是“必须配置上游代理”。默认主线是：

```text
站点 provider 规则
        ↓
接管模式：Hosts / PAC / System Proxy / 后续 DNSIntercept 或 TUN
        ↓
本地 HTTP / HTTPS Reverse Proxy
        ↓
本地 Root CA 与动态站点证书
        ↓
DoH / DNS 解析真实目标
        ↓
provider outbound profile 选择 ForwardHost / TLS SNI / candidate IP
        ↓
stop / restore 恢复系统修改
```

## 当前已实现能力

### 运行模式

- ProxyOnly：本地 HTTP proxy 与 HTTPS CONNECT。
- PAC：本地 PAC server 与 PAC 文件生成。
- System Proxy：Windows / macOS 系统代理写入与恢复。
- Hosts：Windows-first hosts 写入、本地 HTTP / HTTPS 反代、Root CA、动态站点证书、WebSocket 转发。

### Steam 当前落地状态

- 默认 Steam rules：store、community、api、chat、static、cdn 分组。
- 默认 Steam outbound profiles：
  - `steamcommunity.com` / `*.steamcommunity.com` 优先走 `steamcommunity-a.akamaihd.net`。
  - store / checkout / help / login / media 优先走 `cdn-a.akamaihd.net`。
  - `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 覆盖静态资源 / CDN 场景。
- Hosts + Direct 模式默认使用 DoH，避免 hosts 自绕回到 `127.0.0.1`。
- 反代保持原始 HTTP Host，同时按 profile 使用可达 CDN 的 TLS SNI。
- 出站失败诊断可以区分 DoH 解析、TCP 连接、TLS 握手和 HTTP smoke 阶段。
- Windows 中国网络下已手动验证过 Steam 商店、社区、帮助、聊天 / 登录和常见静态资源访问路径。

### Windows 权限与一键体验

- Windows Root CA 默认写入 `LocalMachine\Root`，保留 `cert.store_scope: user` 退路。
- `start --mode hosts` 会将 Root CA 检查 / 安装、hosts preflight、反代监听、hosts 写入和 rollback 纳入同一启动编排。
- 已实现同一二进制隐藏 `__helper`，但默认路线已从短 helper 迁移到 AppHost Service。
- 已实现 Windows `SiteBoostCoreAppHost` 服务：
  - `apphost install|start|stop|status|uninstall|run`。
  - 服务通过 Windows named pipe `\\.\pipe\SiteBoostCoreAppHost` 接收请求。
  - named pipe 使用 DACL 限制本机交互用户接入，并拒绝远程客户端。
  - AppHost 请求会校验 pipe 客户端 PID 必须等于请求中的 `parent_pid`，并校验客户端进程路径必须等于当前 AppHost 二进制。
  - 系统修改请求走 AppHost named pipe RPC。
  - 安装为 `StartAutomatic` + `DelayedAutoStart`。
  - 重启电脑后应由 Windows Service Control Manager 自动拉起。
  - 普通 PowerShell 后续应能无管理员执行 `start --mode hosts`。

### 当前限制

- `version.go`、Go module、CLI、配置字段和大量包名仍带 Steam 专用命名。
- 运行时基本仍在 `internal/`，公共 Go API 尚未稳定。
- AppHost Service 自动启动代码已实现并构建通过，但仍需要在真实 Windows 机器上完成 `install -> reboot -> no-admin start` 验收。
- AppHost RPC 已迁移到 Windows named pipe，并增加 DACL、本机连接限制、pipe client PID 校验与客户端二进制路径校验；后续仍需继续评估用户会话绑定、审计日志和按需启动。
- GitHub 还没有真实 provider，下一阶段只做骨架占位和架构验证。
- hosts 只能覆盖 exact 域名，wildcard 完整覆盖需要 DNSIntercept 或更高级接管模式。
- macOS / Linux Hosts 与证书安装未落地。
- 桌面 installer、服务升级、日志位置、卸载清理和发布包还没有产品化。

## 架构重构目标

下一阶段的核心不是继续堆 Steam 规则，而是在这个实验仓库内把已经验证过的能力整理成高可用、高可维护、可迁移的架构形态。这里的重构目标是降低 Steam 硬编码、明确模块边界、沉淀可复制经验，为未来独立 Go library 仓库做准备，而不是把当前仓库直接发布成正式库。

目标包边界：

- `provider`：站点定义，包含 rule pack、outbound profile、smoke targets、文档元数据。
- `rules`：通用规则编译、匹配、版本信息，不再默认绑定 Steam。
- `resolver`：system / udp / tcp / doh、缓存、IPv4 / IPv6 策略、超时和 fallback。
- `upstream`：direct / HTTP / SOCKS5 / provider profile / candidate dialing。
- `takeover`：ProxyOnly / PAC / System Proxy / Hosts / 后续 DNSIntercept / TUN。
- `reverse`：通用本地 HTTP / HTTPS 反代、证书、WebSocket、错误诊断。
- `certstore`：Root CA、动态证书和平台信任存储。
- `privilege`：Windows AppHost、后续 macOS/Linux 权限边界、系统修改白名单。
- `runtime`：状态、控制接口、生命周期、restore。
- `diagnostics`：脱敏日志、错误摘要、smoke 结果、支持包。

未来独立 Go library 的抽取目标：

- 新仓库提供稳定 Go API，用于启动、停止、查询状态、恢复系统修改。
- 新仓库支持注册或选择 provider，例如 Steam、GitHub。
- 本实验仓库需要把核心逻辑和 CLI 入口解耦到可迁移边界，便于未来复制到新仓库。
- `internal/` 中稳定的能力先在本仓库内整理、测试和文档化；真正 public package 的冻结放到未来新仓库完成。

## 版本计划

### v0.6.4 - Windows AppHost Service 闭环验收

**状态：** 代码已完成，等待真实机器验收
**范围：** Windows / User-facing / Security-Safety / Testing
**目标：** 把 Steam++ 式 Root/AppHost 权限边界变成可重复的一键初始化与无管理员日常启动体验。

#### Focus

- 一次管理员安装 AppHost。
- 重启后自动启动 AppHost。
- 普通 PowerShell 无管理员启动 Hosts 模式。
- AppHost 安全边界和故障恢复。

#### Tasks

- [x] 新增 `SiteBoostCoreAppHost` Windows Service。
- [x] 新增 `apphost install|start|stop|status|uninstall|run` CLI。
- [x] 将 AppHost 配置为 `StartAutomatic` + `DelayedAutoStart`。
- [x] 将 Windows 系统修改默认改为走 AppHost RPC，而不是短生命周期 `runas` helper。
- [x] `apphost install` 对旧 Manual 服务执行配置升级并重启服务。
- [x] 将 AppHost IPC 从早期本地 HTTP 原型迁移到 Windows named pipe。
- [x] 为 AppHost named pipe 增加 DACL、本机连接限制、pipe client PID 校验和客户端二进制路径校验。
- [ ] 在真实 Windows 管理员 PowerShell 中执行 `apphost install` 并记录输出。
- [ ] 验证 `apphost status` 输出 `start_type=automatic delayed_auto_start=true`。
- [ ] 重启 Windows 后验证服务自动运行。
- [ ] 普通 PowerShell 执行 `start --mode hosts`，验证不再要求管理员终端。
- [ ] 验证 `stop` / `restore` 在普通 PowerShell 下通过 AppHost 恢复 hosts。
- [ ] 记录失败场景：服务未安装、服务未运行、named pipe 无法连接、AppHost 请求失败。
- [ ] 设计下一版 AppHost IPC 加固方案：用户会话绑定、请求审计、按需启动。

#### Acceptance Criteria

- 首次安装只需要一次管理员授权。
- 重启后 AppHost 自动运行。
- 普通用户可以执行 `start --mode hosts` 和 `stop`。
- AppHost 失败时错误信息能指导用户安装、启动或恢复。
- AppHost 不接受任意 shell、任意路径写入或敏感凭据。

---

### v0.7.0 - Provider 架构与通用站点骨架

**状态：** 计划中
**范围：** Architecture / Maintainability / Developer-facing / Testing
**目标：** 将 Steam 从核心硬编码中抽离成内置 provider，并以 GitHub 骨架 provider 验证扩展模型。

#### Focus

- 通用 provider / rule pack / outbound profile 模型。
- Steam provider 保持现有行为。
- GitHub provider 先做骨架占位，不承诺真实加速效果。
- 配置命名从 Steam 专用迁移到站点通用。

#### Tasks

- [ ] 审计 Steam 专用命名：`DefaultSteamRules`、`DefaultSteamProfiles`、`NonSteamBehavior`、错误文案、CLI help、docs。
- [ ] 设计 `Provider` 接口：ID、名称、规则、出站 profile、hosts exact 列表、startup probes、smoke targets。
- [ ] 将 Steam rules/profile/smoke targets 收敛到 `provider/steam` 或等价包。
- [ ] 增加 `provider/github` 骨架：GitHub 域名分组、规则版本、空或实验 outbound profile、明确 `experimental` 状态。
- [ ] 配置层支持选择 providers，例如 `providers.enabled: [steam]`，并保留旧 `enable_default_steam_rules` / `enable_default_steam_profiles` 兼容读取。
- [ ] 将 `NonSteamBehavior` 重命名规划为 `non_target_behavior`，保留迁移兼容。
- [ ] 让 reverse/proxy/resolver/upstream 只依赖通用 matcher 和 profile，不依赖 Steam 语义。
- [ ] 增加 provider 单元测试：Steam 行为不变，GitHub 骨架可编译、可匹配、不会影响 Steam。
- [ ] 更新 smoke 文档，区分 Steam stable provider 和 GitHub skeleton provider。

#### Acceptance Criteria

- 新增 GitHub provider 骨架不需要修改 reverse / resolver / upstream 核心逻辑。
- Steam 当前 Windows Hosts + DoH 闭环在重构后保持通过。
- 旧配置仍可运行，迁移提示清晰。
- GitHub 被标记为骨架 / experimental，不夸大为已可用真实加速。

---

### v0.8.0 - 独立 Go Library 抽取准备

**状态：** 计划中
**范围：** API / Architecture / Developer-facing / Documentation
**目标：** 标记、整理和验证可迁移核心能力，为未来新建独立 Go library 仓库做准备。

#### Focus

- 未来 public API 草案。
- Engine 生命周期边界。
- Provider 注册与选择边界。
- 系统修改权限边界。
- 可迁移清单与迁移文档。

#### Tasks

- [ ] 设计未来新仓库 API 草案：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
- [ ] 明确本仓库哪些 `internal/` 包适合迁移、哪些只作为实验实现保留。
- [ ] 提供迁移样例：从本仓库抽取 Steam provider 一键启动、状态查询、停止恢复能力。
- [ ] 提供 provider 开发样例：最小 GitHub skeleton provider。
- [ ] 将 CLI 与核心拼装边界整理清楚，避免未来迁移时把命令行细节带进新库。
- [ ] 设计配置 schema 版本和从实验仓库到正式库的迁移策略。
- [ ] 更新 README：明确本仓库是实验验证仓库，未来正式 Go library 会另起仓库。

#### Acceptance Criteria

- 未来新仓库可以按迁移清单复用本仓库核心能力。
- CLI 与核心能力的耦合点被识别并可拆分。
- API 草案明确只是未来新仓库输入，不在本仓库承诺稳定。
- Steam provider 的真实 smoke 仍可作为迁移回归标准。

---

### v0.9.0 - 高可用性、可恢复性与发布工程

**状态：** 计划中
**范围：** Reliability / Security-Safety / CI-Release / Documentation
**目标：** 将核心从“可跑”提升为“可长期维护、可诊断、可升级、可恢复”。

#### Focus

- AppHost 安全与升级。
- 诊断支持包。
- 服务健康检查。
- rollback 版本化。
- CI 与发布包。

#### Tasks

- [x] 将 AppHost IPC 迁移到 Windows named pipe + DACL + pipe client PID + 客户端二进制路径校验。
- [ ] 为 AppHost 请求增加用户会话绑定和审计日志，继续降低任意本地进程滥用系统修改接口的风险。
- [ ] 增加 AppHost install/upgrade/uninstall 集成 smoke 脚本。
- [ ] 增加 rollback state schema version 和迁移测试。
- [ ] 增加诊断命令：端口占用、hosts 区块、证书 thumbprint、AppHost health、resolver health。
- [ ] 增加 GitHub Actions matrix：Windows / Linux / macOS 的可运行测试边界。
- [ ] 补充 release checklist、签名 / 安装器规划和安全说明。

#### Acceptance Criteria

- AppHost 可升级、可卸载、可诊断。
- 异常退出后系统状态可恢复。
- CI 能覆盖核心单元测试和平台边界。
- 发布前有明确手动验收清单。

---

### v1.0.0-alpha.1 - 实验架构冻结候选

**状态：** 计划中
**范围：** API / Architecture / Testing / Documentation
**目标：** 冻结本实验仓库的可迁移架构边界，为未来独立 Go library 仓库提供稳定输入。

#### Tasks

- [ ] 冻结未来 Go library API 草案。
- [ ] 冻结 provider / rule pack / outbound profile schema。
- [ ] 冻结实验 CLI 主命令和配置迁移策略。
- [ ] 完成 Steam provider Windows smoke 回归。
- [ ] 明确 GitHub provider 状态：skeleton / experimental / stable 之一。
- [ ] 完成安全边界文档：Root CA、hosts、AppHost、DoH、日志脱敏。

#### Acceptance Criteria

- 破坏性改动需要明确记录。
- 未来新仓库可以按冻结边界启动 library 实现。
- Steam provider 仍是已验证主线。

---

### v1.0.0 - 实验验证基线

**状态：** 计划中
**范围：** Release / API / Stability / Documentation
**目标：** 发布本实验仓库的稳定验证基线，作为未来独立 Go library 仓库的主要迁移来源。

#### Tasks

- [ ] 发布本实验仓库的稳定验证版本。
- [ ] 保证 Steam provider 的 Windows Hosts + DoH + AppHost 一键闭环。
- [ ] 保留 ProxyOnly / PAC / System Proxy / Hosts 作为稳定接管模式。
- [ ] 文档覆盖安装、启动、停止、恢复、卸载、证书和 AppHost。
- [ ] 输出给未来新仓库使用的迁移清单、CHANGELOG、release notes 和版本标签。

#### Acceptance Criteria

- 用户可以按文档完成一次管理员初始化和后续无管理员日常启动。
- 默认不需要外部上游代理。
- 高风险系统修改都有恢复路径和安全说明。
- 未来独立 Go library 仓库可以基于本基线复用核心实现和经验。

---

### v1.1+ - 高级能力路线

**状态：** 计划中
**范围：** Advanced / Cross-platform / Security-Safety
**目标：** 在 v1 稳定主线之后逐步加入更强接管能力和更多真实 provider。

#### Candidate Milestones

- `v1.1.0`：GitHub provider 从 skeleton 进入真实网络验证，明确哪些 GitHub 域名和资源可被本地加速。
- `v1.2.0`：DNSIntercept，用于覆盖 hosts 无法表达的 wildcard 和非浏览器 DNS 路径。
- `v1.3.0`：VPN / TUN，用于更强流量接管，但必须有清晰权限、路由和恢复边界。
- `v1.4.0`：JS 注入 / 页面增强，默认关闭，只作为显式高级能力。
- `v1.x`：macOS / Linux Hosts、证书、权限和 AppHost 等价能力。

## 短中长期方向

短期：

- 完成 `v0.6.4` Windows AppHost Service 真实验收。
- 继续把内部 `helper` 命名逐步收敛为更清晰的 AppHost / privileged request 语义。
- 进入 `v0.7.0` provider 架构重构。

中期：

- 完成 Steam provider 抽离。
- 加入 GitHub skeleton provider。
- 整理未来独立 Go library 的 API 草案和迁移清单。
- 将 CLI 与核心能力的边界整理到可迁移状态。

长期：

- `v1.0.0` 作为本实验仓库的稳定验证基线，而不是正式通用库发布。
- 新建独立 Go library 仓库，复用本仓库验证过的核心能力和经验。
- Steam 是稳定 provider；GitHub 先从 skeleton / experimental 逐步进入真实验证。
- DNSIntercept、TUN/VPN、JS 注入只在主线稳定后逐步进入。

## 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| AppHost named pipe 仍缺少用户会话绑定 | 同一交互用户下的本地恶意进程仍可能尝试请求受限系统修改 | 已有 DACL、远程拒绝、pipe client PID、客户端二进制路径校验和命令白名单；后续补用户会话绑定、审计日志和按需启动 |
| AppHost 服务安装 / 自动启动未实测 | 普通用户无管理员启动闭环可能失败 | v0.6.4 必须完成 `install -> reboot -> no-admin start` 验收 |
| Steam 专用命名太深 | 通用 provider 重构成本上升 | v0.7 先做命名审计和兼容迁移 |
| GitHub 过早承诺真实加速 | 误导用户并扩大维护面 | v0.7 只做 skeleton，占位和架构验证 |
| hosts 无法覆盖 wildcard | 部分域名无法接管 | v1.x DNSIntercept / TUN 作为高级能力 |
| Root CA 信任风险 | 用户安全顾虑 | 显式安装 / 卸载、清晰文档、最小命令面、日志脱敏 |
| 80 / 443 端口占用 | Hosts 模式启动失败 | 诊断命令、错误提示、高端口 smoke |
| 复制 SteamTools 源码 | 许可证和维护风险 | 坚持 clean-room，只参考架构思想 |
| 误把实验仓库当正式库 | 路线图和用户预期偏移 | 明确正式 Go library 未来另起仓库，本仓库只做验证和迁移来源 |
| Go API 过早冻结 | 未来新仓库设计受阻 | 本仓库只冻结 API 草案和迁移边界，正式兼容承诺放到新仓库 |
