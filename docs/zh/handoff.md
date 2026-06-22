# Handoff - SiteBoost Core

> 更新时间：2026-06-23
> 当前分支：`master`
> 当前远端：`https://github.com/gofurry/siteboost-core.git`
> 当前工作目录：`E:\Git\开源\go-steam-core`
> 当前目标：从 Steam 专用本地加速核心演进为通用站点加速核心，并为后续抽出 Go library 做架构重构

## 一句话状态

这个仓库已经具备 Steam Windows Hosts + DoH + HTTPS Reverse Proxy 的可用闭环，并已加入 Windows AppHost Service 代码，用于靠近 Steam++ / Watt Toolkit 的“一次管理员初始化，后续普通用户启动”体验；但项目还没有完成通用 provider 架构、公共 Go API 抽离、GitHub provider 真实加速和跨平台闭环。

## 当前事实

- 仓库上游已经改名为 `gofurry/siteboost-core`。
- 本地目录仍是 `E:\Git\开源\go-steam-core`。
- Go module 仍是 `github.com/gofurry/go-steam-core`。
- CLI 仍是 `steam-accelerator`。
- `version.go` 仍显示 `v0.6.3`。
- 主干代码实际已经进入 `v0.6.4-dev` 级别，因为已包含 AppHost Service 自动启动能力。
- 最近关键提交：
  - `e748d5f feat(windows): auto start apphost service`
  - `4e1bad9 feat(windows): add privileged apphost service`
  - `34ec200 fix(windows): check administrator membership from process token`
  - `08679e5 fix(windows): report elevated hosts startup result`
  - `f8e3276 fix(windows): relaunch hosts start as administrator`

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
- 本地监听：`127.0.0.1:26505`
- 服务启动类型：`StartAutomatic`
- 服务延迟启动：`DelayedAutoStart`
- 已安装旧服务时会升级配置并重启服务
- Windows 系统修改默认走 AppHost RPC，包括 Root CA、hosts、restore 等受限操作

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
- GitHub 后续先做 skeleton provider，只用于验证架构扩展，不应文档承诺真实加速。
- 下一阶段的关键不是继续堆 Steam 域名，而是做 provider 化、权限边界、诊断、恢复和 public Go API。

## 当前仍存在的风险

- AppHost `install -> reboot -> no-admin start` 还需要真实 Windows 机器验收。
- AppHost RPC 当前是 loopback HTTP，命令面受限，但还缺少更强的 IPC ACL / token / named pipe / 用户会话绑定设计。
- `version.go`、Go module、CLI、配置字段、包名和 docs 仍大量带 Steam 专用命名。
- 公共 Go API 尚未抽出，核心仍主要在 `internal/`。
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
- `docs/steam-compatibility.md`

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
state=running start_type=automatic delayed_auto_start=true pid=...
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
```

卸载 AppHost 需要管理员 PowerShell：

```powershell
.\bin\steam-accelerator.exe apphost uninstall
```

## 下一阶段建议顺序

1. 完成 `v0.6.4` AppHost 真实验收，并把输出补进 smoke 文档。
2. 更新 README、smoke、security、windows-one-click-flow，把默认路径从短 `runas` helper 改成 AppHost Service。
3. 做 Steam 专用命名审计，不急着改 module，先列迁移表。
4. 设计 `Provider` 接口和 provider registry。
5. 把 Steam rules、profiles、startup probes、smoke targets 收敛为 `provider/steam`。
6. 增加 `provider/github` skeleton，只声明 experimental，不承诺真实加速。
7. 让 reverse / resolver / upstream 只依赖通用 matcher 和 outbound profile，不依赖 Steam 语义。
8. 开始 public Go library 候选 API：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`。
9. 设计 AppHost IPC 加固：优先评估 named pipe / DACL / token / 用户会话绑定。
10. 进入 release engineering：CI matrix、rollback schema version、installer、服务升级 / 卸载、签名规划。

## 提醒新 session 的边界

- 不要使用 `git reset --hard` 或 `git checkout --` 回滚用户改动。
- 不要提交 `bin/`、runtime state、日志、证书、私钥、`.env`。
- 不要把 SteamTools 源码复制进仓库。
- 不要承诺 GitHub 已可加速；当前只计划 skeleton。
- 不要把 HTTP / SOCKS5 upstream 写成默认必需能力。
- 不要把 AppHost 描述成 UAC 绕过；它是一次管理员安装后的受控系统服务。
- 每次改动后先跑相关验证，再本地提交。
