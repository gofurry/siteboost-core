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
- `version.go` 已是 `v0.6.4-dev`。
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
- Steam 命名和假设仍大量存在。
- GitHub 尚未实现真实 provider。
- AppHost 已完成本机管理员安装、健康检查、named pipe RPC、普通 PowerShell `start --mode hosts`、`stop` / `restore` 主流程验证；仍需要补充重启 Windows 后自动拉起的 smoke 记录。
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
- takeover mode：ProxyOnly、PAC、System Proxy、Hosts、后续 DNSIntercept、TUN
- certificate / Root CA / dynamic cert
- privilege / AppHost / restore
- diagnostics / smoke / support bundle

## 后续版本摘要

### v0.6.4 - Windows AppHost Service 闭环验收

**状态：** 本机主流程已通过，重启自动拉起 smoke 待补。

目标是完成 Steam++ 式“一次管理员初始化，后续普通用户启动”的基础体验。

已经验证：

- 管理员 PowerShell 执行 `apphost install`。
- `apphost status` 显示 `start_type=automatic delayed_auto_start=true ... health=ok`。
- 普通 PowerShell 执行 `start --mode hosts` 成功。
- 普通 PowerShell 执行 `stop` / `restore` 成功。
- 验证 named pipe IPC 在普通 PowerShell 下能完成 Root CA、hosts 和 restore 请求。
- `stop` / `restore` 后 AppHost 保持 `running health=ok`，这是为了后续无管理员启动而常驻。

下一步必须补充：

- 重启电脑后服务自动运行。
- 重启后普通 PowerShell 执行 `start --mode hosts` 成功。
- 将本机输出补进 smoke 文档。

### v0.7.0 - Provider 架构与通用站点骨架

目标是把 Steam 从核心硬编码中抽离成内置 provider。

重点任务：

- 设计 provider 接口。
- 抽出 Steam rules、profiles、smoke targets。
- 加入 GitHub skeleton provider。
- 配置支持 `providers.enabled`。
- 保留旧 Steam 配置的迁移兼容。
- 让 reverse / resolver / upstream 不再依赖 Steam 语义。

### v0.8.0 - 独立 Go Library 抽取准备

目标是形成未来独立 Go library 的 API 草案、迁移清单和可复用模块边界。

重点任务：

- 设计未来新仓库 API 草案：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
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
- DNSIntercept。
- VPN / TUN。
- JS 注入 / 页面增强。
- macOS / Linux Hosts、证书和权限闭环。

## 交接提示

新的 session 应优先阅读：

1. [handoff.md](./handoff.md)
2. [ROADMAP.md](../../ROADMAP.md)
3. [windows-one-click-flow.md](./windows-one-click-flow.md)
4. [smoke-test.md](./smoke-test.md)
5. `internal/privilege/privilege_windows.go`
6. `internal/rules/rules.go`
7. `internal/upstream/profile.go`
8. `cmd/steam-accelerator/main.go`

最重要的下一步不是继续加 Steam 域名，而是补齐 AppHost 重启自动拉起 smoke，并开始 provider 架构重构。

正式通用 Go library 不在本仓库内直接完成；它应在本仓库验证充分后另起仓库维护。
