# 中文 Roadmap

主路线图维护在仓库根目录：[ROADMAP.md](../../ROADMAP.md)。

本文件作为 `docs/zh/` 下的中文入口，保留当前阶段、关键判断和后续安排摘要。详细版本计划、风险表和验收标准以根目录 `ROADMAP.md` 为准。

## 当前阶段

当前仓库已经从 `go-steam-core` 的 Steam 专用方向开始过渡到 `siteboost-core` 的通用站点加速方向。

当前事实：

- 远端仓库已经改为 `gofurry/siteboost-core`。
- Go module 仍是 `github.com/gofurry/go-steam-core`。
- CLI 仍是 `steam-accelerator`。
- `version.go` 仍是 `v0.6.3`。
- 主干代码已包含 `v0.6.4-dev` 级别的 Windows AppHost Service：
  - `apphost install|start|stop|status|uninstall|run`。
  - 服务名：`SiteBoostCoreAppHost`。
  - 本地监听：`127.0.0.1:26505`。
  - 安装为 `StartAutomatic` + `DelayedAutoStart`。
  - 系统修改请求默认走 AppHost RPC。

当前 Steam 能力已经比较完整：

- ProxyOnly / PAC / System Proxy / Windows Hosts 四种模式。
- Hosts + DoH 默认闭环。
- Windows Root CA 自动检查 / 安装。
- Windows hosts 写入与 rollback。
- Steam 默认 rules 与 outbound profiles。
- Steam 商店、社区、帮助、登录 / 聊天、静态资源的 Windows 中国网络手动通过记录。
- 出站失败诊断、启动探测、`system_change` 输出和规则版本输出。

当前仍不是稳定通用库：

- 公共 Go API 未冻结，核心仍主要在 `internal/`。
- Steam 命名和假设仍大量存在。
- GitHub 尚未实现真实 provider。
- AppHost 自动启动闭环需要实机验证。
- AppHost IPC 还需要从 loopback HTTP 进一步评估到 named pipe / ACL / token 等更安全形态。

## 总体方向

目标不是只做 Steam，而是做一个类似 Steam++ 网络加速模块的 Go 核心：

- Steam 是第一个稳定 provider。
- GitHub 先做 skeleton provider，用来验证架构，不立刻承诺真实加速。
- 后续支持更多站点和服务。
- 最终把核心能力抽成一个通用 Go library。
- CLI / 桌面壳 / sidecar 只是这个 library 的调用方。

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

**状态：** 代码已完成，等待真实 Windows 验收。

目标是完成 Steam++ 式“一次管理员初始化，后续普通用户启动”的基础体验。

下一步必须验证：

- 管理员 PowerShell 执行 `apphost install`。
- `apphost status` 显示 `start_type=automatic delayed_auto_start=true`。
- 重启电脑后服务自动运行。
- 普通 PowerShell 执行 `start --mode hosts` 成功。
- 普通 PowerShell 执行 `stop` / `restore` 成功。

### v0.7.0 - Provider 架构与通用站点骨架

目标是把 Steam 从核心硬编码中抽离成内置 provider。

重点任务：

- 设计 provider 接口。
- 抽出 Steam rules、profiles、smoke targets。
- 加入 GitHub skeleton provider。
- 配置支持 `providers.enabled`。
- 保留旧 Steam 配置的迁移兼容。
- 让 reverse / resolver / upstream 不再依赖 Steam 语义。

### v0.8.0 - 通用 Go Library 抽离候选

目标是形成第一个 public Go API 候选。

重点任务：

- 设计 `Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
- 明确哪些包迁出 `internal/`。
- CLI 改成 public API 调用方。
- 提供 Steam provider 和 GitHub skeleton provider 的 Go 示例。

### v0.9.0 - 高可用性、可恢复性与发布工程

目标是让项目从“能跑”变成“能维护、能诊断、能升级、能恢复”。

重点任务：

- 加固 AppHost IPC。
- 增加诊断命令和支持包。
- 版本化 rollback state。
- 增加跨平台 CI matrix。
- 完成 installer / release checklist。

### v1.0.0-alpha.1 - API 与架构冻结候选

目标是冻结 v1 候选 API 和架构，允许外部工具开始试用。

### v1.0.0 - 稳定发布

目标是发布第一个稳定通用站点本地加速核心。

稳定主线：

- Steam provider 稳定。
- Windows Hosts + DoH + AppHost 一键闭环稳定。
- ProxyOnly / PAC / System Proxy / Hosts 都有明确支持边界。
- Go library API 在 v1 内保持兼容。

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

最重要的下一步不是继续加 Steam 域名，而是验证 AppHost 自动启动闭环，并开始 provider 架构重构。
