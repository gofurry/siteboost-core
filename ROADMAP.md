# SiteBoost Core 中文 Roadmap

> 状态日期：2026-06-23
> 当前仓库：`gofurry/siteboost-core`
> 当前 Go module：`github.com/gofurry/go-steam-core`
> 当前 CLI / ProjectName：`steam-accelerator` / `steam-accelerator-core`
> 当前代码阶段：`version.go` 已进入 `v0.7.4-dev`，主干包含 Windows AppHost Service、named pipe IPC、provider registry、Steam stable provider、GitHub experimental skeleton provider、DNSIntercept manual 本地 DNS server、显式 Windows system DNS 接管 / rollback / restore、默认关闭的 Page Enhance 响应转换 pipeline，以及 Steam 官方 Web API outbound profile；本机已完成 AppHost health、named pipe RPC、普通用户 `start/stop/restore`、真实中国网络 Steam 访问、卸载主流程和 Windows system DNS 接管 / 恢复 smoke 记录，真实 Windows 重启后的 AppHost 自动拉起仍建议补充单独记录

## 当前定位

这个仓库是本地站点加速核心的实验性验证仓库。它当前以 Steam 为唯一真实落地目标，用来验证 Steam++ / Watt Toolkit 式本地加速闭环：规则接管、本地反代、Root CA、DoH、出站 profile、权限编排和恢复。v0.7 已加入 provider registry：Steam 是默认 stable provider，GitHub 是需要显式启用的 experimental skeleton provider，只用于验证扩展边界，不承诺真实加速。本仓库不计划直接改造成正式 Go 开源库。

正式的通用 Go 加速库会在 [gofurry/web-boost](https://github.com/gofurry/web-boost) 维护。届时只抽取本仓库中已经验证过的核心能力，包括 resolver、reverse proxy、certstore 抽象、provider 规则模型、DNSIntercept、Page Enhance、rollback 和 diagnostics；不能把当前实验仓库的 Steam 历史命名、CLI 形状、AppHost service 安装流程或 Windows smoke 脚手架耦合进新库。本仓库的主要价值是作为实验场、行为样板、smoke 记录和迁移来源，而不是最终稳定库本身。

项目参考 Steam++ / Watt Toolkit 的本地加速架构思想：本地反代、Hosts / PAC / System Proxy 接管、本地 Root CA、DNS / DoH、可选上游代理、提权 Root/AppHost 进程和恢复机制。项目保持 clean-room 边界：只参考架构思想和公开行为，不复制、不翻译、不移植 SteamTools 源码。

默认加速路线不是“必须配置上游代理”。默认主线是：

```text
站点 provider 规则
        ↓
接管模式：Hosts / PAC / System Proxy / DNSIntercept；TUN/VPN 暂不进入本仓库实现
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
- DNSIntercept：manual 策略下启动本地 UDP/TCP DNS server 与本地 HTTP / HTTPS 反代，不自动修改系统 DNS、hosts 或证书信任；system 策略可在 Windows 上显式接管指定网卡 DNS，并通过 rollback / AppHost 恢复。

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
  - named pipe 使用 DACL 限制交互用户接入，并在平台支持时启用远程客户端拒绝。
  - AppHost 请求会校验 pipe 客户端 PID 必须等于请求中的 `parent_pid`，并校验客户端进程路径必须等于当前 AppHost 二进制。
  - 系统修改请求走 AppHost named pipe RPC。
  - 安装为 `StartAutomatic` + `DelayedAutoStart`。
  - 重启电脑后应由 Windows Service Control Manager 自动拉起。
  - 普通 PowerShell 后续应能无管理员执行 `start --mode hosts`。
  - `apphost status` 已支持 `health=ok` 健康检查。
  - AppHost IPC 已从早期 `127.0.0.1:26505` 本地 HTTP 原型迁移到 Windows named pipe，不再暴露 loopback HTTP 控制端口。
  - `stop` / `restore` 只关闭加速状态和恢复系统修改，不停止 AppHost；AppHost 常驻是为了后续普通用户一键启动，彻底移除需要管理员执行 `apphost uninstall`。
  - AppHost 服务绑定安装时的固定二进制路径；移动 exe、换路径或重建发布位置后，需要管理员重新执行 `apphost install` 更新服务路径。

### 当前限制

- `version.go` 已进入 `v0.7.4-dev`，但 Go module、CLI、配置字段和部分包名仍带 Steam 专用命名。
- 运行时基本仍在 `internal/`，公共 Go API 尚未稳定。
- AppHost Service 已在本机完成管理员安装、`health=ok`、named pipe RPC、普通用户 Hosts 闭环、真实中国网络 Steam 访问、`stop` / `restore` 和卸载主流程验证；仍建议补充 `install -> reboot -> no-admin start` 的单独重启 smoke 记录。
- AppHost RPC 已迁移到 Windows named pipe，并增加 DACL、本机连接限制、pipe client PID 校验与客户端二进制路径校验；后续仍需继续评估用户会话绑定、审计日志和按需启动。
- GitHub 已有 experimental skeleton provider，可通过 `providers.enabled` 显式启用；它不包含默认 outbound profile，也不承诺真实加速。
- hosts 只能覆盖 exact 域名；DNSIntercept manual 已可覆盖 provider wildcard 与自定义规则，但只在 `mode: dns` 下显式启动，不自动改系统 DNS。`system` 策略已支持 Windows 显式网卡 DNS 接管，要求 53 端口、loopback DNS server、显式 `dns_intercept.interfaces` 和 rollback；本机已验证 WLAN DNS 可切到 `127.0.0.1` 并通过 `stop` / `restore` 回到原 DNS。`external` 策略仍是后续计划，不能默认改坏开发者环境。
- Page Enhance 已提供默认关闭的 response transform pipeline，支持 provider / host / path / content-type / status 匹配、header 修改、HTML 注入、本地 asset、replace 和 Go transformer 扩展点；它不写系统 DNS、hosts、证书、浏览器配置或开发者环境。
- macOS / Linux Hosts 与证书安装未落地。
- 桌面 installer、服务升级、日志位置、卸载清理和发布包还没有产品化。

## 架构重构目标

下一阶段的核心不是继续堆 Steam 规则，而是在这个实验仓库内把已经验证过的能力整理成高可用、高可维护、可迁移的架构形态。这里的重构目标是降低 Steam 硬编码、明确模块边界、沉淀可复制经验，为未来独立 Go library 仓库做准备，而不是把当前仓库直接发布成正式库。

目标包边界：

- `provider`：站点定义，包含 rule pack、outbound profile、smoke targets、文档元数据。
- `rules`：通用规则编译、匹配、版本信息，不再默认绑定 Steam。
- `resolver`：system / udp / tcp / doh、缓存、IPv4 / IPv6 策略、超时和 fallback。
- `upstream`：direct / HTTP / SOCKS5 / provider profile / candidate dialing。
- `takeover`：ProxyOnly / PAC / System Proxy / Hosts / DNSIntercept；TUN/VPN 不进入当前实验主线。
- `reverse`：通用本地 HTTP / HTTPS 反代、证书、WebSocket、错误诊断。
- `certstore`：Root CA、动态证书和平台信任存储。
- `privilege`：Windows AppHost、后续 macOS/Linux 权限边界、系统修改白名单。
- `runtime`：状态、控制接口、生命周期、restore。
- `diagnostics`：脱敏日志、错误摘要、smoke 结果、支持包。

未来 `gofurry/web-boost` Go library 的抽取目标：

- 新仓库提供稳定 Go API，用于启动、停止、查询状态、恢复系统修改。
- 新仓库支持注册或选择 provider，例如 Steam、GitHub。
- 本实验仓库需要把核心逻辑和 CLI 入口解耦到可迁移边界，便于未来复制到新仓库。
- `internal/` 中稳定的能力先在本仓库内整理、测试和文档化；真正 public package 的冻结放到 `gofurry/web-boost` 完成。
- 新仓库目录必须分层清楚，根目录只放 `package webboost` 的入口 API；provider、rules、network、takeover、reverse、pageenhance、certstore、rollback、diagnostics 和 adapters 分包维护，避免各种能力散在仓库根目录。

## 版本计划

### v0.6.4 - Windows AppHost Service 闭环验收

**状态：** 主体验闭环已记录，真实重启自动拉起 smoke 建议继续补充
**范围：** Windows / User-facing / Security-Safety / Testing
**目标：** 把 Steam++ 式 Root/AppHost 权限边界变成可重复的一键初始化与无管理员日常启动体验。

#### Focus

- 一次管理员安装 AppHost。
- AppHost 作为常驻提权底座，通过 Windows named pipe 接收白名单系统修改请求。
- 普通用户启动、停止和恢复 Hosts 模式。
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
- [x] 修复 Windows 服务删除后 `marked for deletion` 场景的等待与提示。
- [x] 修复 named pipe 响应写入后客户端偶发 `No process is on the other end of the pipe`。
- [x] 在真实 Windows 管理员 PowerShell 中执行 `apphost install` 并验证服务可运行。
- [x] 验证 `apphost status` 输出 `start_type=automatic delayed_auto_start=true ... health=ok`。
- [x] 普通 PowerShell 执行 `start --mode hosts`，验证不再要求管理员终端。
- [x] 验证 `stop` / `restore` 在普通 PowerShell 下通过 AppHost 恢复 hosts；AppHost 保持 `running health=ok` 是预期行为。
- [x] 将本机输出整理进 smoke 文档：安装、健康检查、普通用户 Hosts 闭环、stop、restore、uninstall 和服务不存在错误。
- [ ] 补充真实 Windows 重启后的服务自动拉起记录：`install -> reboot -> apphost status health=ok -> no-admin start --mode hosts`。
- [ ] 继续补充失败场景文档：服务未安装、服务未运行、named pipe 无法连接、AppHost 请求失败、二进制路径变更。
- [ ] 在 v0.6.5 / v0.9 中继续设计 AppHost IPC 加固方案：用户会话绑定、请求审计、按需启动。

#### Acceptance Criteria

- 首次安装只需要一次管理员授权。
- AppHost 常驻运行但不表示加速正在运行；加速状态由 `start/stop/restore/status` 管理。
- 普通用户可以执行 `start --mode hosts`、`stop` 和 `restore`。
- 服务已配置为 `StartAutomatic` + `DelayedAutoStart`，真实重启后自动运行需要保留单独 smoke 记录。
- AppHost 失败时错误信息能指导用户安装、启动或恢复。
- AppHost 不接受任意 shell、任意路径写入或敏感凭据。

---

### v0.6.5 - 能力边界冻结与 v0.7 预检

**状态：** 已完成
**范围：** Architecture / Security-Safety / Documentation / Developer-facing
**目标：** 在 provider 架构重构前冻结核心能力边界，避免 v1.1+ 高级能力提前污染 v0.7 的开源库抽取准备。

#### Focus

- 明确 provider、takeover mode、privilege/AppHost、certificate、resolver/upstream 和 diagnostics 的责任边界。
- 将 GitHub 真实加速、DNSIntercept、TUN/VPN、JS 注入和跨平台 AppHost 等高级能力保留为 v1.1+ 候选，不在 v0.7 前实现。
- 为 v0.7 Provider 抽象提供可执行的 non-goals 和 extension points。
- 保留 Steam 当前 Windows Hosts + DoH + AppHost 闭环作为后续重构的回归基线。

#### Tasks

- [x] 记录 v0.6.4 AppHost + Steam 真实中国网络主流程 smoke。
- [x] 新增 v0.6.5 能力边界文档，说明 provider / takeover / privilege / cert / diagnostics 的归属。
- [x] 明确 v1.1+ 高级能力不提前实现，只作为 future extension points。
- [x] 审计 Steam 专用命名和核心包依赖，形成 v0.7 重构清单。
- [x] 为 v0.7 Provider 接口草案列出必需字段和禁止职责。
- [x] 更新 smoke 文档，明确 v0.7 后 Steam provider 必须保持当前 Windows 闭环回归。

#### Acceptance Criteria

- Provider 不拥有 hosts 写入、Root CA 安装、AppHost 调用、TUN/DNSIntercept 控制或任意系统修改权限。
- Takeover mode 负责流量接管方式，Provider 只提供站点规则、outbound profile 和 smoke targets。
- AppHost 只执行白名单系统修改请求，不暴露任意 shell、任意路径写入或 provider 私有命令。
- DNSIntercept、TUN/VPN、JS 注入、GitHub 真实加速和跨平台权限闭环在 v0.7 前只记录边界，不进入实现。
- v0.7 可以在不修改 reverse / resolver / upstream 核心语义的前提下加入 Steam provider 和 GitHub skeleton provider。

#### Notes

详细边界维护在 [docs/zh/capability-boundary.md](docs/zh/capability-boundary.md)。

v0.7 provider registry 落地后，DNSIntercept 和 Page Enhance 已调整为抽库前的主能力验证；它们必须显式启用、可观察、可还原。TUN/VPN 继续延期，优先使用成熟外部库或独立集成。

---

### v0.7.0 - Provider 架构与通用站点骨架

**状态：** 开发与自动化验证已完成，等待真实 Windows smoke 回归
**范围：** Architecture / Maintainability / Developer-facing / Testing
**目标：** 将 Steam 从核心硬编码中抽离成内置 provider，并以 GitHub 骨架 provider 验证扩展模型。

#### Focus

- 通用 provider / rule pack / outbound profile 模型。
- Steam provider 保持现有行为。
- GitHub provider 先做骨架占位，不承诺真实加速效果。
- 配置命名从 Steam 专用迁移到站点通用。

#### Tasks

- [x] 审计 Steam 专用命名：`DefaultSteamRules`、`DefaultSteamProfiles`、`NonSteamBehavior`、错误文案、CLI help、docs。
- [x] 设计 provider 数据结构：ID、名称、状态、规则、出站 profile、startup probes。
- [x] 将 Steam rules/profile/smoke targets 收敛到 `internal/provider`。
- [x] 增加 `provider/github` 骨架：GitHub 域名分组、规则版本、空 outbound profile、明确 `experimental` 状态。
- [x] 配置层支持选择 providers，例如 `providers.enabled: [steam]`；旧 Steam 专用 key 返回迁移错误。
- [x] 将 `NonSteamBehavior` 重命名为 `non_target_behavior`，CLI 改为 `--non-target`，旧 `--non-steam` 返回迁移错误。
- [x] 让 reverse/proxy/resolver/upstream 只依赖通用 matcher 和 profile，不依赖 Steam 默认数据。
- [x] 增加 provider 单元测试：Steam 行为不变，GitHub 骨架可编译、可匹配、不会影响 Steam。
- [x] 完成自动化验证：`git diff --check`、`go test ./...`、`go vet ./...`、race 子集、Windows 二进制 build、`--version`。
- [x] 更新 smoke 文档，区分 Steam stable provider 和 GitHub skeleton provider。

#### Acceptance Criteria

- [x] 新增 GitHub provider 骨架不需要修改 reverse / resolver / upstream 核心逻辑。
- [x] Steam 默认 provider 的规则、outbound profile、startup probes 和 status 兼容输出保持可测。
- [x] 旧配置不再静默兼容，迁移提示清晰。
- [x] GitHub 被标记为骨架 / experimental，不夸大为已可用真实加速。
- [ ] Steam 当前 Windows Hosts + DoH + AppHost 真实环境回归 smoke 在重构后保持通过。
- [ ] `providers.enabled: [steam, github]` 真实启动 smoke 显示两个 provider，GitHub 为 `experimental`，且不要求 GitHub live 可达。

---

### v0.7.1 - DNSIntercept 决策与本地 DNS Server 基础

**状态：** 代码与自动化验证已完成，等待真实手动 smoke
**范围：** Network / Takeover / Reliability / Testing
**目标：** 在不默认修改系统 DNS 的前提下，为 provider wildcard 和非 hosts 场景提供可验证的 DNS 接管基础。

#### Focus

- DNS 查询决策：target provider 域名、本地映射、非目标转发。
- 可选本地 DNS server，默认只在显式 `manual` 策略下启动，不自动改系统 DNS。
- 避免把 `127.0.0.1:53` 占用和系统 DNS 接管做成隐藏默认行为。
- 为后续 `system` 和 `external` 策略预留边界。

#### Tasks

- [x] 新增 `mode: dns` 与 `dns_intercept.strategy: manual|system|external` 配置草案；v0.7.1 只允许 `manual`，默认不启用，不修改系统 DNS。
- [x] 实现 DNS decision 层：provider rules / custom domains / hosts extra domains 命中返回本地映射，非目标查询转发到 DoH 或显式上游。
- [x] 实现可选本地 DNS server，支持 UDP/TCP 查询、响应缓存、超时、并发保护、命中/转发/cache/error 统计。
- [x] 对 `A` / `AAAA` / `HTTPS` / `SVCB` 等记录提供显式策略：目标 `A` / `AAAA` 映射到本地，目标 `HTTPS` / `SVCB` 默认返回 NODATA，其他目标类型默认不转发。
- [x] 启动前检测 listen 地址和端口占用，冲突时返回清晰错误，不强占端口。
- [x] `status` 输出 DNSIntercept 策略、监听地址、是否接管系统 DNS、命中数、转发数、cache 命中数、错误数。
- [x] CLI 支持 `start --mode dns --dns-listen 127.0.0.1:15353`，并在 `start/status` 输出 `dns_intercept:` 摘要。
- [x] 单元测试覆盖 target 命中、非目标转发、上游失败、缓存、UDP/TCP server、端口冲突、engine DNS 模式和 CLI 输出。

#### Acceptance Criteria

- [x] `manual` 策略下不会修改系统 DNS、hosts、证书信任或任何持久化系统设置。
- [x] 本地 DNS server 停止后不留下系统状态变化。
- [x] 非目标 DNS 默认使用 DoH 或显式上游，不会自绕回到本机 DNS server。
- [x] 端口冲突、上游失败和不支持记录类型都有可诊断错误或 status。
- [x] 所有行为都可通过配置关闭、停止进程或不启用来恢复原状；系统 DNS 接管留到 v0.7.2。

---

### v0.7.2 - Windows System DNS 显式接管与恢复

**状态：** 代码、自动化验证与真实 Windows system DNS smoke 已完成
**范围：** Windows / Reliability / Security-Safety / Testing
**目标：** 在用户显式选择 `dns_intercept.strategy: system` 时，通过 AppHost 受控修改系统 DNS，并保证 stop / restore 可还原。

#### Focus

- Windows 网卡 DNS preflight。
- AppHost 白名单 DNS apply / restore 请求。
- rollback 记录原始 DNS 状态。
- 崩溃恢复和重复 restore 幂等。

#### Tasks

- [x] 设计 `system_dns` rollback schema，记录每个受影响 interface 的原始 DNS 设置、DHCP/static 状态和应用时间。
- [x] AppHost 增加窄命令：`apply-system-dns`、`restore-system-dns`、`preflight-system-dns`，不接受任意命令或任意脚本。
- [x] `start --mode dns` 或等价配置在 `strategy: system` 下先完成 preflight，再启动 DNS server，再应用系统 DNS。
- [x] `stop` / `restore` 使用 rollback 恢复原 DNS；失败时 rollback 保留，可再次执行 `restore`。
- [x] `status` 和 `system_change:` 输出 DNS preflight / apply 的 interface 数、目标 DNS 和 helper 信息。
- [x] 单元测试覆盖 config、systemdns rollback、Windows PowerShell 后端脚本、AppHost 请求校验、Engine 启停顺序和 CLI restore 分发。
- [x] 真实 Windows smoke 覆盖 apply、stop、restore 和系统 DNS 恢复；AppHost 缺失、端口占用、崩溃后 restore 仍可作为后续负向 smoke 扩展。

#### Acceptance Criteria

- [x] 不选择 `strategy: system` 时，库不会改系统 DNS。
- [x] 任何系统 DNS 修改前都必须已有 rollback 记录或可生成 rollback 记录。
- [x] `stop` / `restore` 会先恢复系统 DNS，再关闭本地 DNS server。
- [x] AppHost 缺失、权限不足、接口枚举失败或 DNS server 未启动时，不应写入半成品系统 DNS。
- [x] 失败路径会优先尝试恢复，且保留 rollback 供用户再次执行 `restore`。
- [x] 真实 Windows smoke 确认系统 DNS 回到启动前状态，并记录手动恢复命令。

---

### v0.7.3 - JS 注入与页面增强透明 Pipeline

**状态：** 代码、自动化验证与真实浏览器 Page Enhance smoke 已完成
**范围：** Reverse Proxy / Developer-facing / Diagnostics / Testing
**目标：** 在本地 reverse proxy 中提供可观察、可关闭、无隐藏安全魔法的响应转换能力，让开发者显式决定如何注入或增强页面。

#### Focus

- 有序 response transform pipeline。
- YAML 声明式 transform 和 Go 自定义 transformer。
- 机械能力：header 修改、HTML 注入、本地 asset serving、字符串替换。
- 可观察性：每次应用、跳过和错误都能看到原因。

#### Tasks

- [x] 设计 `page_enhance.enabled`、`transforms`、`assets`、`on_error`、`max_body_size` 等配置；默认关闭。
- [x] 实现 `ResponseMeta` 与 `Transformer` 接口，支持 provider、host、path、content-type、status code 等显式匹配。
- [x] 实现内置机械 transform：header remove/set、HTML head/body 注入、本地 asset mount、简单 replace。
- [x] 开发者可选择 `on_error: pass_through|fail_closed`；库不得因内置黑盒规则偷偷跳过 login、checkout 或任意路径。
- [x] 对压缩、body size、unsupported content encoding、transform error 等跳过或失败原因输出 `page_enhance_*` log 和 status 计数。
- [x] Provider 只能声明推荐 enhancement pack 或元数据；是否启用、启用哪些 transform 由开发者或上层应用决定。
- [x] 单元测试覆盖 header 修正、Content-Length / ETag 处理、asset serving、错误策略、body size skip、custom transformer 和 reverse / engine / CLI status 接入。

#### Acceptance Criteria

- [x] 默认不启用页面增强，不修改任何响应内容。
- [x] 启用后，所有修改都来自显式配置或显式注册的 transformer。
- [x] 库不内置不可见的“安全跳过”规则；任何跳过都必须有明确原因。
- [x] 页面增强不会写系统 DNS、hosts、证书、浏览器配置或开发者环境。
- [x] 开发者可以通过关闭 `page_enhance.enabled` 或移除 transform 完全恢复原始响应行为。

---

### v0.7.4 - Steam 官方 Web API outbound profile

**状态：** 代码与自动化验证已完成，等待真实 Go API smoke 记录
**范围：** Provider / Upstream Profile / Diagnostics / Smoke
**目标：** 修复 `api.steampowered.com` 在中国网络下虽被 hosts 接管但反代上游仍走原始直连导致 Go 程序请求超时的问题。

#### Focus

- 借鉴 Steam++ / Watt Toolkit 公开远端加速项目的行为层经验：`api.steampowered.com` 随 Steam 商店项目走 `steamstore.rmbgame.net` 前置域。
- 只补 API provider profile，不整体替换已通过 smoke 的 store/community profile。
- 保留原始 HTTP Host；TLS 仍验证证书链，但允许该前置域场景下的 hostname mismatch。
- 把 `api.steampowered.com` 加入默认 startup probes，让 API 链路断点在启动时可见。

#### Acceptance Criteria

- [x] 默认 Steam provider 输出 `profiles=5 probes=7`。
- [x] `api.steampowered.com` 匹配 Steam provider API rule，并优先连接 `steamstore.rmbgame.net`。
- [x] `partner.steam-api.com` 仍保留规则捕获，但未把未经验证的 profile 强加到该域名。
- [x] 文档记录 API smoke 命令和 profile 语义。

---

### v0.8.0 - 独立 Go Library 抽取准备

**状态：** 计划中
**范围：** API / Architecture / Developer-facing / Documentation
**目标：** 在 Provider、DNSIntercept 和 Page Enhance 主能力边界验证后，标记、整理和验证可迁移核心能力，为 `gofurry/web-boost` 独立 Go library 做准备。

#### Focus

- 未来 public API 草案。
- Engine 生命周期边界。
- Provider 注册与选择边界。
- 系统修改权限边界，尤其是 DNS / hosts / cert / AppHost 的可还原约束。
- DNSIntercept 与 Page Enhance 的 public API 草案。
- `web-boost` 目录层级和 package 边界。
- 可迁移清单与迁移文档。

#### Tasks

- [ ] 将 [docs/zh/web-boost-library-plan.md](docs/zh/web-boost-library-plan.md) 固化为 v0.8.0 抽取约束，明确目标仓库为 `gofurry/web-boost`。
- [ ] 设计 `web-boost` API 草案：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
- [ ] 明确本仓库哪些 `internal/` 包适合迁移、哪些只作为实验实现保留。
- [ ] 提供迁移样例：从本仓库抽取 Steam provider 一键启动、状态查询、停止恢复能力。
- [ ] 提供 provider 开发样例：最小 GitHub skeleton provider。
- [ ] 将 CLI 与核心拼装边界整理清楚，避免未来迁移时把命令行细节带进新库。
- [ ] 设计配置 schema 版本和从实验仓库到正式库的迁移策略。
- [ ] 明确哪些能力会进入 core library，哪些作为可选 adapter 或外部集成保留。
- [ ] 规划 `web-boost` 目录层级：根包入口、provider、rules、network、takeover、reverse、pageenhance、certstore、rollback、diagnostics、adapters、internal、examples 和 docs。
- [ ] 更新 README：明确本仓库是实验验证仓库，正式 Go library 是 `gofurry/web-boost`。

#### Acceptance Criteria

- `gofurry/web-boost` 可以按迁移清单复用本仓库核心能力。
- CLI 与核心能力的耦合点被识别并可拆分。
- API 草案明确只是 `web-boost` 输入，不在本仓库承诺稳定。
- Steam provider 的真实 smoke 仍可作为迁移回归标准。
- DNSIntercept 的 `manual` 策略和 Page Enhance 的默认关闭行为不会修改开发者系统环境。
- 新库不继承当前实验仓库的 Steam 命名、CLI/AppHost 包袱或平铺式目录结构。

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
- [x] 明确 GitHub provider 状态：skeleton / experimental / stable 之一。
- [ ] 完成安全边界文档：Root CA、hosts、AppHost、DoH、日志脱敏。

#### Acceptance Criteria

- 破坏性改动需要明确记录。
- `web-boost` 可以按冻结边界启动 library 实现。
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
- [ ] 输出给 `web-boost` 使用的迁移清单、CHANGELOG、release notes 和版本标签。

#### Acceptance Criteria

- 用户可以按文档完成一次管理员初始化和后续无管理员日常启动。
- 默认不需要外部上游代理。
- 高风险系统修改都有恢复路径和安全说明。
- 未来独立 Go library 仓库可以基于本基线复用核心实现和经验。

---

### v1.1+ - 高级能力路线

**状态：** 计划中
**范围：** Advanced / Cross-platform / Security-Safety
**目标：** 在主线能力稳定之后逐步加入更多真实 provider、外部 DNS 集成和跨平台体验；TUN/VPN 暂不进入本仓库实现。

#### Candidate Milestones

- `v1.1.0`：GitHub provider 从 skeleton 进入真实网络验证，明确哪些 GitHub 域名和资源可被本地加速。
- `v1.2.0+`：外部 DNS 工具集成，例如导出规则给 AdGuardHome、dnsmasq、sing-box 或 Clash Meta。
- `v1.x`：更多 provider 的真实网络验证和 enhancement pack。
- `v1.x`：macOS / Linux Hosts、证书、权限和 AppHost 等价能力。
- `deferred`：VPN / TUN 可使用成熟外部库或独立项目集成，不作为当前 core library 前置目标。

## 短中长期方向

短期：

- 补齐 `v0.6.4` Windows AppHost Service 单独重启自动拉起 smoke 记录。
- 保留 `v0.7.2` DNSIntercept manual 高端口 smoke、Windows system DNS 显式接管 smoke 和恢复命令记录。
- 完成 provider 架构真实 Windows smoke 回归。
- 保留 `v0.7.3` Page Enhance 真实浏览器 smoke 记录，确认显式配置后能注入、可观察、移除配置即可恢复。
- 继续把内部 `helper` 命名逐步收敛为更清晰的 AppHost / privileged request 语义。
- 将 `v0.8.0` 独立 Go Library 抽取准备指向 `gofurry/web-boost`，先冻结目录层级、API 草案和迁移清单。

中期：

- 整理未来独立 Go library 的 API 草案和迁移清单。
- 将 CLI 与核心能力的边界整理到可迁移状态。
- 设计外部 DNS 工具导出和 enhancement pack 的 adapter 边界。

长期：

- `v1.0.0` 作为本实验仓库的稳定验证基线，而不是正式通用库发布。
- 在 `gofurry/web-boost` 中维护正式 Go library，复用本仓库验证过的核心能力和经验。
- Steam 是稳定 provider；GitHub 先从 skeleton / experimental 逐步进入真实验证。
- DNSIntercept 和 JS / Page Enhance 作为抽库前主能力验证；TUN/VPN 使用成熟外部库或独立项目，不进入当前 core 前置目标。

## 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| AppHost named pipe 仍缺少用户会话绑定 | 同一交互用户下的本地恶意进程仍可能尝试请求受限系统修改 | 已有 DACL、远程拒绝、pipe client PID、客户端二进制路径校验和命令白名单；后续补用户会话绑定、审计日志和按需启动 |
| AppHost 重启后自动拉起 smoke 未记录 | 电脑重启后的无管理员启动闭环可能仍有遗漏 | 已完成安装、health、named pipe、普通用户 Hosts 闭环、stop/restore、uninstall 主流程记录；下一步补 `reboot -> apphost status -> no-admin start` |
| DNSIntercept 默认接管系统 DNS | 占用 53 端口或把开发者系统 DNS 指向不可用服务，导致全局网络异常 | 默认只做 `manual` 策略，不改系统 DNS；`system` 策略必须显式启用、preflight、rollback、stop/restore 幂等 |
| DNS server 占用 53 端口 | 与 AdGuard、Clash、dnscrypt-proxy、Docker 等本地服务冲突 | 启动前检测端口，冲突时报错；支持 `manual` 非系统接管和 `external` 规则导出路线 |
| Page Enhance 隐藏规则过多 | 开发者配置了注入却被库内置规则跳过，难以排查 | 不内置不可见的安全跳过；跳过和错误必须输出 reason；是否注入 login/checkout 等页面由开发者显式决定 |
| v0.7.x 能力扩展污染 Provider 抽象 | Provider 同时承担系统修改、DNS 接管或页面改写执行职责，未来 Go library 边界变重 | Provider 只声明 rules/profile/probes/可选 enhancement metadata；DNS 接管属于 takeover，页面增强属于 reverse transform pipeline |
| 新库继承实验仓库包袱 | `web-boost` 变成 `siteboost-core` 的改名版本，目录、命名和职责不适合开源长期维护 | v0.8.0 先冻结抽取约束和目录层级；只迁移核心能力，CLI、Steam 历史命名、AppHost installer、smoke 脚手架不进入核心库 |
| Steam 专用命名太深 | 通用 provider 重构成本上升 | v0.7 已完成 provider registry、配置迁移错误和 CLI 新参数；Go module / CLI 名称保留为实验仓库历史包袱 |
| GitHub 过早承诺真实加速 | 误导用户并扩大维护面 | v0.7 只做 skeleton，占位和架构验证 |
| hosts 无法覆盖 wildcard | 部分域名无法接管 | 已实现 DNSIntercept manual/server 和显式 Windows system DNS 接管；仍需真实 system smoke 验证恢复 |
| Root CA 信任风险 | 用户安全顾虑 | 显式安装 / 卸载、清晰文档、最小命令面、日志脱敏 |
| 80 / 443 端口占用 | Hosts 模式启动失败 | 诊断命令、错误提示、高端口 smoke |
| 复制 SteamTools 源码 | 许可证和维护风险 | 坚持 clean-room，只参考架构思想 |
| 误把实验仓库当正式库 | 路线图和用户预期偏移 | 明确正式 Go library 是 `gofurry/web-boost`，本仓库只做验证和迁移来源 |
| Go API 过早冻结 | `web-boost` 设计受阻 | 本仓库只冻结 API 草案和迁移边界，正式兼容承诺放到 `web-boost` |
