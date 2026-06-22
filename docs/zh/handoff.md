# Handoff - SiteBoost Core

> 更新时间：2026-06-23
> 当前分支：`master`
> 当前远端：`https://github.com/gofurry/siteboost-core.git`
> 当前工作目录：`E:\Git\开源\go-steam-core`
> 当前目标：在本实验仓库中验证 Steam++ 式本地加速闭环和多站点 provider 架构，为未来新建独立 Go library 仓库沉淀可复用核心能力

## 一句话状态

这个仓库已经具备 Steam Windows Hosts + DoH + HTTPS Reverse Proxy 的可用闭环，并已把 Windows AppHost Service 从 loopback HTTP 原型迁移到 named pipe IPC，用于靠近 Steam++ / Watt Toolkit 的“一次管理员初始化，后续普通用户启动”体验。当前本机主流程已验证到 `apphost health=ok`、普通 PowerShell `start --mode hosts`、`stop` 和 `restore` 可用；重启后自动拉起还需要补一条 smoke 记录。这个仓库是实验验证仓库，不是未来正式 Go 开源库本体；后续会新建独立仓库维护通用 Go 加速库，并从这里复用、迁移或重写已经验证过的核心能力。

## 当前事实

- 仓库上游已经改名为 `gofurry/siteboost-core`。
- 本地目录仍是 `E:\Git\开源\go-steam-core`。
- Go module 仍是 `github.com/gofurry/go-steam-core`。
- CLI 仍是 `steam-accelerator`。
- `version.go` 已显示 `v0.6.4-dev`。
- 主干代码已经进入 `v0.6.4-dev` 阶段，包含 AppHost Service 自动启动与 named pipe IPC 能力。
- 本仓库定位为实验场和迁移来源；正式 Go library 会另起新仓库维护。
- 最近关键提交：
  - `c93de53 fix(windows): stabilize apphost named pipe responses`
  - `c3bb577 fix(windows): wait for apphost service deletion`
  - `f577289 feat(windows): move apphost IPC to named pipe`
  - `1eba323 docs: clarify experimental repo roadmap`
  - `4614e64 docs: rewrite roadmap for siteboost core`
  - `e748d5f feat(windows): auto start apphost service`
  - `4e1bad9 feat(windows): add privileged apphost service`
  - `34ec200 fix(windows): check administrator membership from process token`
  - `08679e5 fix(windows): report elevated hosts startup result`
  - `f8e3276 fix(windows): relaunch hosts start as administrator`

## 当前本机验证结论

- `apphost status` 可返回 `apphost: running start_type=automatic delayed_auto_start=true pid=... health=ok`。
- 普通 PowerShell 可执行 `start --mode hosts`，系统修改请求通过 AppHost named pipe 完成，不再要求当前终端本身是管理员。
- 普通 PowerShell 可执行 `stop` / `restore`。
- `stop` / `restore` 后再次执行 `apphost status` 仍显示 `running health=ok` 是预期行为：AppHost 是常驻提权底座，不是“当前加速状态”。
- 当前未再暴露早期原型的 `127.0.0.1:26505` HTTP 控制端口；AppHost IPC 走 Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`。
- 如果移动 exe、换安装目录、重新发布到新路径，需要管理员 PowerShell 重新执行一次 `apphost install`，因为 Windows Service 绑定的是安装时的二进制路径。
- 仍建议下一 session 补做一次完整重启验证：重启 Windows 后检查 `apphost status`，再用普通 PowerShell 启动 Hosts 模式。

## 已实现能力

### 基础运行模式

- ProxyOnly：本地 HTTP proxy 与 HTTPS CONNECT。
- PAC：本地 PAC server 与 PAC 文件生成。
- System Proxy：Windows / macOS 系统代理写入与恢复。
- Hosts：Windows-first hosts 写入、本地 HTTP / HTTPS 反代、Root CA、动态站点证书、WebSocket 转发。

### Steam 加速主线

当前 Steam 是唯一真实落地 provider，已经覆盖：

- `steamcommunity.com`
- `store.steampowered.com`
- `help.steampowered.com`
- `media.steampowered.com`
- `community.steamstatic.com`
- `steamcdn-a.akamaihd.net`
- Steam 登录、社区聊天、静态资源、CDN 常见路径

已知手动验证记录：

- 用户在中国网络下确认浏览器可以打开 Steam 内容。
- 用户确认 `https://steamcommunity.com/chat/` 可正常使用。
- 用户确认 Steam 登录可正常使用。
- `Test-NetConnection <host> -Port 443` 在 Hosts 模式下解析到 `127.0.0.1` 且 TCP 443 可连通，这是预期表现。

### 默认 Steam outbound profile

当前默认不是“必须配置上游代理”。默认主线是本地反代 + DoH + provider outbound profile：

- `steamcommunity.com` / `*.steamcommunity.com` 优先走 `steamcommunity-a.akamaihd.net`。
- store / checkout / help / login / media 优先走 `cdn-a.akamaihd.net`。
- `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 覆盖静态资源 / CDN。
- 反代保持原始 HTTP Host，同时按 profile 使用可达 CDN 的 TLS SNI。
- HTTP / SOCKS5 upstream 只是可选增强，不是默认加速前提。

### Windows AppHost Service

当前代码已经实现 Windows `SiteBoostCoreAppHost` 服务：

- CLI：`apphost install|start|stop|status|uninstall|run`
- 隐藏入口：`__apphost-service`
- IPC：Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`
- named pipe 使用 DACL、本机连接限制、pipe client PID 和客户端二进制路径校验
- 服务启动类型：`StartAutomatic`
- 服务延迟启动：`DelayedAutoStart`
- 已安装旧服务时会升级配置并重启服务
- Windows 系统修改默认走 AppHost named pipe RPC，包括 Root CA、hosts、restore 等受限操作
- `apphost status` 会发起健康检查，成功时输出 `health=ok`
- 早期 `127.0.0.1:26505` 本地 HTTP IPC 已废弃，后续不要再按端口服务方向继续实现
- `stop` / `restore` 不停止 AppHost；彻底移除提权底座要用管理员 PowerShell 执行 `apphost uninstall`
- Windows SCM 的 `marked for deletion` 场景已经增加等待与提示，遇到卡住时优先关闭 Services.msc、任务管理器或其他持有服务句柄的进程
- named pipe 响应已增加 flush 与断线重试，之前的 `No process is on the other end of the pipe` 不应再作为正常结果出现

目标体验：

1. 管理员 PowerShell 执行一次 `apphost install`。
2. Windows 重启后 Service Control Manager 自动拉起 AppHost。
3. 普通 PowerShell 执行 `start --mode hosts`，无需再手动以管理员运行。
4. 普通 PowerShell 执行 `stop` / `restore`，通过 AppHost 恢复 hosts。

## 重要设计结论

- 这个项目要靠近 Steam++ 的“一键可用默认闭环”，但不能绕过 Windows UAC。
- Steam++ 类体验的正确工程路径是安装一个受控的提权 Root/AppHost 进程或服务，而不是每次启动都弹 UAC，也不是尝试绕过权限。
- 当前项目采用 clean-room 实现：参考 Steam++ / Watt Toolkit 的架构思想和公开行为，不复制、不翻译、不移植 SteamTools 源码。
- Root CA 默认走 Windows `LocalMachine\Root`，管理员上下文下可以静默安装；`CurrentUser\Root` 更容易触发系统确认弹窗。
- AppHost 常驻是正常设计：它只是等待白名单系统修改请求，不代表 Steam 加速仍处于开启状态。
- 继续向 Steam++ 靠齐时，应优先收紧 IPC 权限边界、安装 / 升级 / 卸载体验和诊断，而不是尝试绕过 Windows UAC。
- GitHub 后续先做 skeleton provider，只用于验证架构扩展，不应文档承诺真实加速。
- 下一阶段的关键不是继续堆 Steam 域名，而是做 provider 化、权限边界、诊断、恢复和未来独立 Go library 的迁移边界。

## 当前仍存在的风险

- AppHost `install -> no-admin start/stop/restore` 主流程已通过；`reboot -> apphost auto-start -> no-admin start` 仍需要真实 Windows 重启 smoke 记录。
- AppHost RPC 使用 Windows named pipe，命令面受限，并带 DACL、本机连接限制、pipe client PID 和客户端二进制路径校验；后续仍需要评估用户会话绑定、审计日志和按需启动。
- `version.go`、Go module、CLI、配置字段、包名和 docs 仍大量带 Steam 专用命名。
- 公共 Go API 尚未抽出，核心仍主要在 `internal/`；正式 public API 应在未来新仓库内冻结。
- rollback state schema 没有版本化迁移。
- installer、服务升级、卸载清理、日志位置、发布包、签名还未产品化。
- Hosts 模式只能覆盖 exact 域名，wildcard 完整覆盖需要 DNSIntercept 或 TUN。
- macOS / Linux 的 Hosts、证书和权限闭环没有落地。
- `curl.exe` 如报 `CRYPT_E_NO_REVOCATION_CHECK`，通常是 Windows Schannel 吊销检查问题；可用 `curl.exe --ssl-no-revoke -I https://steamcommunity.com/` 辅助验证。

## 代码入口

优先阅读这些文件：

- `cmd/steam-accelerator/main.go`：CLI、隐藏 helper / apphost 入口、命令分发。
- `internal/engine/engine.go`：启动编排、Hosts 模式、Root CA、hosts 写入、rollback、system_change。
- `internal/privilege/privilege.go`：受限系统修改请求模型和 AppHost 调用入口。
- `internal/privilege/privilege_windows.go`：Windows AppHost Service、RPC server、Windows 权限检测。
- `internal/rules/rules.go`：Steam 默认规则、规则匹配、规则版本。
- `internal/upstream/profile.go`：Steam outbound profile、ForwardHost、TLS SNI、candidate dialing。
- `internal/resolver/resolver.go`：system / udp / tcp / doh resolver。
- `internal/reverse/reverse.go`：本地 HTTP / HTTPS reverse proxy、WebSocket、502 诊断。
- `internal/certstore/platform_windows.go`：Windows 证书存储安装 / 查询 / 卸载。
- `internal/hosts/hosts.go` 与 `internal/hosts/platform_windows.go`：hosts marker block、preflight、写入与恢复。

优先阅读这些文档：

- `ROADMAP.md`
- `docs/zh/roadmap.md`
- `docs/zh/windows-one-click-flow.md`
- `docs/zh/smoke-test.md`
- `docs/zh/security.md`
- `docs/zh/steam-compatibility.md`

## 本机验证命令

### 自动化验证

```powershell
git diff --check
go test ./...
go vet ./...
go test -race ./internal/hosts ./internal/privilege ./internal/engine ./cmd/steam-accelerator
go build -o .\bin\steam-accelerator.exe .\cmd\steam-accelerator
```

如果只是改文档，至少执行：

```powershell
git diff --check
```

### AppHost 真实 Windows 验收

管理员 PowerShell：

```powershell
.\bin\steam-accelerator.exe apphost install
.\bin\steam-accelerator.exe apphost status
```

期望看到类似：

```text
apphost: running start_type=automatic delayed_auto_start=true pid=... health=ok
```

然后重启 Windows。

普通 PowerShell：

```powershell
.\bin\steam-accelerator.exe apphost status
.\bin\steam-accelerator.exe start --mode hosts
Test-NetConnection steamcommunity.com -Port 443
Test-NetConnection store.steampowered.com -Port 443
Test-NetConnection help.steampowered.com -Port 443
```

浏览器打开：

```text
https://steamcommunity.com/
https://steamcommunity.com/chat/
https://store.steampowered.com/
https://help.steampowered.com/
```

停止和恢复：

```powershell
.\bin\steam-accelerator.exe stop
.\bin\steam-accelerator.exe restore
.\bin\steam-accelerator.exe apphost status
```

此时 `apphost status` 仍应显示 `running health=ok`。这表示提权底座仍在等待下一次普通用户启动，不表示 Hosts 加速仍开启。

卸载 AppHost 需要管理员 PowerShell：

```powershell
.\bin\steam-accelerator.exe apphost uninstall
```

## 下一阶段建议顺序

1. 补做 `v0.6.4` 重启 smoke：`reboot -> apphost status health=ok -> normal-user start --mode hosts -> stop/restore`，并把输出补进 smoke 文档。
2. 做 Steam 专用命名审计，不急着改 module，先列迁移表。
3. 设计 `Provider` 接口和 provider registry。
4. 把 Steam rules、profiles、startup probes、smoke targets 收敛为 `provider/steam`。
5. 增加 `provider/github` skeleton，只声明 experimental，不承诺真实加速。
6. 让 reverse / resolver / upstream 只依赖通用 matcher 和 outbound profile，不依赖 Steam 语义。
7. 整理未来独立 Go library 的 API 草案和迁移清单：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
8. 继续设计 AppHost IPC 加固：优先评估用户会话绑定、审计日志和按需启动。
9. 逐步把内部 `helper` 命名收敛为 AppHost / privileged request 语义，避免误导后续维护者。
10. 进入 release engineering：CI matrix、rollback schema version、installer、服务升级 / 卸载、签名规划。
11. 在验证充分后新建正式 Go library 仓库，从本仓库迁移已验证的 resolver、reverse、certstore、privilege、provider 和 diagnostics 能力。

## 提醒新 session 的边界

- 不要使用 `git reset --hard` 或 `git checkout --` 回滚用户改动。
- 不要提交 `bin/`、runtime state、日志、证书、私钥、`.env`。
- 不要把 SteamTools 源码复制进仓库。
- 不要承诺 GitHub 已可加速；当前只计划 skeleton。
- 不要把 HTTP / SOCKS5 upstream 写成默认必需能力。
- 不要把 AppHost 描述成 UAC 绕过；它是一次管理员安装后的受控系统服务。
- 不要把本仓库描述成未来正式 Go library；正式库会另起仓库，本仓库只做实验验证和迁移来源。
- 每次改动后先跑相关验证，再本地提交。
