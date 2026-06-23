# 中文 Roadmap

主路线图维护在仓库根目录：[ROADMAP.md](../../ROADMAP.md)。

本文件作为 `docs/zh/` 下的中文入口，保留当前阶段、关键判断和后续安排摘要。详细版本计划、风险表和验收标准以根目录 `ROADMAP.md` 为准。

## 当前阶段

当前仓库已经从 `go-steam-core` 的 Steam 专用方向开始过渡到 `siteboost-core` 的通用站点加速实验方向。

需要特别注意：这个仓库是实验性验证仓库，不是未来正式 Go 开源库本体。后续会新建一个独立仓库专门维护通用 Go 加速库；新仓库可以直接借鉴、拆分或复用本仓库验证过的核心内容。

当前事实：

- 远端仓库已经改为 `gofurry/siteboost-core`。
- Go module 仍是 `github.com/gofurry/go-steam-core`。
- CLI 仍是 `steam-accelerator`。
- `version.go` 已是 `v0.7.0-dev`。
- 主干代码已包含 Windows AppHost Service 与 named pipe IPC：
  - `apphost install|start|stop|status|uninstall|run`。
  - 服务名：`SiteBoostCoreAppHost`。
  - IPC：Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`。
  - 安装为 `StartAutomatic` + `DelayedAutoStart`。
  - 系统修改请求默认走 AppHost named pipe RPC。
  - named pipe 使用 DACL、本机连接限制、pipe client PID 和客户端二进制路径校验。
  - `apphost status` 已支持 `health=ok` 健康检查。
  - AppHost 已从早期 `127.0.0.1:26505` 本地 HTTP 原型迁移到 Windows named pipe，不再暴露 loopback HTTP 控制端口。
  - `stop` / `restore` 后 AppHost 继续 `running health=ok` 是预期行为：它是后续无管理员启动的常驻提权底座，不代表加速仍在运行。

当前 Steam 能力已经比较完整：

- ProxyOnly / PAC / System Proxy / Windows Hosts 四种模式。
- Hosts + DoH 默认闭环。
- Windows Root CA 自动检查 / 安装。
- Windows hosts 写入与 rollback。
- Steam 默认 rules 与 outbound profiles。
- Steam 商店、社区、帮助、登录 / 聊天、静态资源的 Windows 中国网络手动通过记录。
- 出站失败诊断、启动探测、`system_change` 输出和规则版本输出。

当前仍不是稳定通用库，也不计划在本仓库内直接变成稳定通用库：

- 公共 Go API 未冻结，核心仍主要在 `internal/`。
- Go module 和 CLI 仍保留 Steam 历史命名，核心规则/profile 已完成 provider 化。
- GitHub 已有 experimental skeleton provider，可显式启用用于架构验证，但不承诺真实加速。
- AppHost 已完成本机管理员安装、健康检查、named pipe RPC、普通 PowerShell Hosts 闭环、真实中国网络 Steam 访问、`stop` / `restore` 和卸载主流程记录；仍建议补充重启 Windows 后自动拉起的单独 smoke 记录。
- AppHost IPC 已迁移到 Windows named pipe；后续还需要评估用户会话绑定、审计日志和按需启动。

## 总体方向

目标不是只做 Steam，而是在本实验仓库里验证一个类似 Steam++ 网络加速模块的 Go 核心：

- Steam 是第一个稳定 provider。
- GitHub 先做 skeleton provider，用来验证架构，不立刻承诺真实加速。
- 后续支持更多站点和服务。
- 功能和架构验证后，新建独立仓库维护正式通用 Go library。
- 本仓库负责沉淀可迁移核心能力、真实 smoke、失败经验和安全边界。
- CLI / 桌面壳 / sidecar 在本仓库中主要作为实验入口和验证工具。

通用核心应抽象出：

- provider / rule pack / outbound profile
- resolver / DoH / DNS cache
- upstream / candidate dialing / TLS SNI
- takeover mode：ProxyOnly、PAC、System Proxy、Hosts、DNSIntercept；TUN/VPN 暂不进入本仓库实现
- certificate / Root CA / dynamic cert
- privilege / AppHost / restore
- diagnostics / smoke / support bundle

## 后续版本摘要

### v0.6.4 - Windows AppHost Service 闭环验收

**状态：** 主体验闭环已记录，真实重启自动拉起 smoke 建议继续补充。

目标是完成 Steam++ 式“一次管理员初始化，后续普通用户启动”的基础体验。

已经验证：

- 管理员 PowerShell 执行 `apphost install`。
- `apphost status` 显示 `start_type=automatic delayed_auto_start=true ... health=ok`。
- 普通 PowerShell 下 Hosts 模式已运行，Steam 目标域名解析到 `127.0.0.1` 且本地 443 可连通。
- 普通 PowerShell 执行 `stop` / `restore` 成功。
- 验证 named pipe IPC 在普通 PowerShell 下能完成 Root CA、hosts 和 restore 请求。
- `stop` / `restore` 后 AppHost 保持 `running health=ok`，这是为了后续无管理员启动而常驻。
- 管理员 PowerShell 执行 `apphost uninstall` 后，`apphost status` 返回服务不存在错误，符合预期。

仍建议补充：

- 重启电脑后服务自动运行。
- 重启后普通 PowerShell 执行 `start --mode hosts` 成功。

### v0.6.5 - 能力边界冻结与 v0.7 预检

目标是在进入 Provider 架构重构前，把能力边界固定下来。结论是：`v1.1+` 的 GitHub 真实加速、DNSIntercept、TUN/VPN、JS 注入和跨平台权限闭环不提前到 `v0.7` 前实现，只作为 future extension points 和 non-goals 记录。

重点任务：

- 新增 [能力边界冻结](./capability-boundary.md) 文档。
- 明确 Provider 只描述站点规则、outbound profile、hosts exact list 和 smoke targets，不负责 hosts 写入、Root CA、AppHost、TUN 或系统修改。
- 明确 takeover mode 负责流量接管方式，Provider 不携带系统接管职责。
- 明确 AppHost 是平台权限执行器，不是 provider 能力。
- 将 Steam 当前 Windows Hosts + DoH + AppHost smoke 作为 v0.7 重构回归基线。
- 在 v0.7 前只做 Steam provider 和 GitHub skeleton provider，不承诺 GitHub 真实加速。

v0.7 完成后，路线调整为：DNSIntercept 和 Page Enhance 会在抽取开源库前验证，但必须显式启用、可观察、可还原；TUN/VPN 继续延期，优先使用成熟外部库或独立集成。

### v0.7.0 - Provider 架构与通用站点骨架

**状态：** 开发与自动化验证已完成，等待真实 Windows smoke 回归。

目标是把 Steam 从核心硬编码中抽离成内置 provider。

重点任务：

- 已设计 provider 数据结构。
- 已抽出 Steam rules、profiles、smoke targets。
- 已加入 GitHub skeleton provider。
- 已支持 `providers.enabled`。
- 旧 Steam 专用配置 key 会返回迁移错误。
- reverse / resolver / upstream 不再依赖 Steam 默认数据。
- 已通过 `git diff --check`、`go test ./...`、`go vet ./...`、核心 race 子集、Windows 二进制 build 和 `--version`。
- 仍需补做默认 Steam Hosts + DoH + AppHost 真实回归 smoke，以及 `[steam, github]` 显式 provider 配置 smoke。

### v0.7.1 - DNSIntercept 决策与本地 DNS Server 基础

目标是在不默认修改系统 DNS 的前提下，验证 DNSIntercept 的规则决策、本地 DNS server、非目标转发、端口冲突检测、status 统计和关闭恢复。第一阶段只做 `manual` 策略：显式启用、手动使用、不留下系统状态变化。

### v0.7.2 - Windows System DNS 显式接管与恢复

目标是在 `manual` 路径稳定后，为 `dns_intercept.strategy: system` 增加 AppHost 白名单系统 DNS 修改能力。任何系统 DNS 改动都必须先有 rollback，`stop` / `restore` 后必须恢复，失败时不能把开发者机器留在“系统 DNS 指向本地但服务未运行”的状态。

### v0.7.3 - JS 注入与页面增强透明 Pipeline

目标是在 reverse proxy 内增加默认关闭的 response transform pipeline。库只提供 header 修改、HTML 注入、本地 asset、replace 和自定义 transformer 等机械能力，不做隐藏安全跳过；每次应用、跳过或失败都必须能在 status / log 里看到原因。

### v0.8.0 - 独立 Go Library 抽取准备

目标是在 Provider、DNSIntercept 和 Page Enhance 主能力边界验证后，形成未来独立 Go library 的 API 草案、迁移清单和可复用模块边界。

重点任务：

- 设计未来新仓库 API 草案：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`、DNSIntercept 策略和 Page Enhance pipeline。
- 明确哪些包适合迁移到新仓库，哪些只作为实验实现保留。
- 梳理 CLI 与核心能力的边界，避免未来新库被 CLI 细节污染。
- 提供 Steam provider 和 GitHub skeleton provider 的迁移示例。

### v0.9.0 - 高可用性、可恢复性与发布工程

目标是让项目从“能跑”变成“能维护、能诊断、能升级、能恢复”。

重点任务：

- 加固 AppHost IPC。
- 增加诊断命令和支持包。
- 版本化 rollback state。
- 增加跨平台 CI matrix。
- 完成 installer / release checklist。

### v1.0.0-alpha.1 - 实验架构冻结候选

目标是冻结本实验仓库的可迁移架构边界，为未来独立 Go library 仓库提供稳定输入。

### v1.0.0 - 实验验证基线

目标是发布本实验仓库的稳定验证基线，而不是把本仓库直接发布成正式通用库。

稳定主线：

- Steam provider 稳定。
- Windows Hosts + DoH + AppHost 一键闭环稳定。
- ProxyOnly / PAC / System Proxy / Hosts 都有明确支持边界。
- 输出给未来新仓库使用的迁移清单、API 草案和真实 smoke 记录。

### v1.1+

候选方向：

- GitHub provider 真实网络验证。
- 外部 DNS 工具规则导出。
- 更多 provider enhancement pack。
- 如确实需要，基于成熟库或独立项目集成 TUN/VPN adapter。
- macOS / Linux Hosts、证书和权限闭环。

## 交接提示

新的 session 应优先阅读：

1. [handoff.md](./handoff.md)
2. [ROADMAP.md](../../ROADMAP.md)
3. [capability-boundary.md](./capability-boundary.md)
4. [windows-one-click-flow.md](./windows-one-click-flow.md)
5. [smoke-test.md](./smoke-test.md)
6. `internal/privilege/privilege_windows.go`
7. `internal/rules/rules.go`
8. `internal/upstream/profile.go`
9. `cmd/steam-accelerator/main.go`

最重要的下一步不是继续加 Steam 域名，也不是做 TUN/VPN 或 GitHub 真实加速，而是对 v0.7 provider 架构做一次真实 Windows Hosts + DoH + AppHost 回归 smoke，然后进入 DNSIntercept manual 策略和 Page Enhance 透明 pipeline 的设计验证。AppHost 真实重启自动拉起 smoke 仍建议单独补充。

正式通用 Go library 不在本仓库内直接完成；它应在本仓库验证充分后另起仓库维护。
