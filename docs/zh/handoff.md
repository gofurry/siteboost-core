# Handoff - SiteBoost Core

> 更新时间：2026-06-23
> 当前分支：`master`
> 当前远端：`https://github.com/gofurry/siteboost-core.git`
> 当前工作目录：`D:\WorkSpace\Git\siteboost-core`
> 当前目标：在本实验仓库中验证 Steam++ 式本地加速闭环和多站点 provider 架构，为未来新建独立 Go library 仓库沉淀可复用核心能力

## 一句话状态

这个仓库已经具备 Steam Windows Hosts + DoH + HTTPS Reverse Proxy 的可用闭环，并已把 Windows AppHost Service 从 loopback HTTP 原型迁移到 named pipe IPC，用于靠近 Steam++ / Watt Toolkit 的“一次管理员初始化，后续普通用户启动”体验。当前本机主流程已验证到 `apphost health=ok`、普通 PowerShell Hosts 闭环、真实中国网络 Steam 访问、`stop`、`restore`、`apphost uninstall` 和 Windows system DNS 接管 / 恢复可用；真实重启后 AppHost 自动拉起仍建议补一条单独 smoke 记录。当前代码已进入 `v0.7.3-dev`：Steam 是默认 stable provider，GitHub 是显式启用的 experimental skeleton provider，DNSIntercept manual 已提供本地 UDP/TCP DNS server、目标映射、非目标转发、缓存和 status 统计；Windows system 策略已提供显式网卡 DNS 接管、rollback 和 restore；Page Enhance 已提供默认关闭、显式配置、可观察的 reverse response transform pipeline。这个仓库是实验验证仓库，不是未来正式 Go 开源库本体；后续会新建独立仓库维护通用 Go 加速库，并从这里复用、迁移或重写已经验证过的核心能力。

## 当前事实

- 仓库上游已经改名为 `gofurry/siteboost-core`。
- 本地目录是 `D:\WorkSpace\Git\siteboost-core`。
- Go module 仍是 `github.com/gofurry/go-steam-core`。
- CLI 仍是 `steam-accelerator`。
- `version.go` 已显示 `v0.7.3-dev`。
- 主干代码已经进入 `v0.7.3-dev` 阶段，包含 AppHost Service 自动启动、named pipe IPC、provider registry、Steam stable provider、GitHub experimental skeleton provider、DNSIntercept manual 本地 DNS server、Windows system DNS 显式接管 / rollback / restore 和默认关闭的 Page Enhance pipeline。
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
- 普通 PowerShell 下 Hosts 模式可用，系统修改请求通过 AppHost named pipe 完成，不再要求当前终端本身是管理员。
- 中国网络下 Steam 目标域名可被 hosts 接管到 `127.0.0.1`，本地 443 连通，浏览器可加速访问 Steam。
- 普通 PowerShell 可执行 `stop` / `restore`。
- Windows system DNS smoke 已验证：指定 WLAN DNS 切到 `127.0.0.1` 后，Steam 目标域名解析到本地，非目标域名继续解析，`stop` / `restore` 后 DNS 回到原值。
- `stop` / `restore` 后再次执行 `apphost status` 仍显示 `running health=ok` 是预期行为：AppHost 是常驻提权底座，不是“当前加速状态”。
- `apphost uninstall` 可卸载服务，卸载后 `apphost status` 返回服务不存在错误是预期行为。
- 当前未再暴露早期原型的 `127.0.0.1:26505` HTTP 控制端口；AppHost IPC 走 Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`。
- 如果移动 exe、换安装目录、重新发布到新路径，需要管理员 PowerShell 重新执行一次 `apphost install`，因为 Windows Service 绑定的是安装时的二进制路径。
- 仍建议后续补做一次完整重启验证：重启 Windows 后检查 `apphost status`，再用普通 PowerShell 启动 Hosts 模式。

## 已实现能力

### 基础运行模式

- ProxyOnly：本地 HTTP proxy 与 HTTPS CONNECT。
- PAC：本地 PAC server 与 PAC 文件生成。
- System Proxy：Windows / macOS 系统代理写入与恢复。
- Hosts：Windows-first hosts 写入、本地 HTTP / HTTPS 反代、Root CA、动态站点证书、WebSocket 转发。
- DNSIntercept：manual 策略下启动本地 UDP/TCP DNS server 与本地 HTTP / HTTPS 反代，不自动修改系统 DNS、hosts、Root CA 信任或浏览器设置；system 策略在 Windows 上显式接管指定网卡 DNS，并通过 rollback 恢复。
- Page Enhance：默认关闭，启用后在 reverse proxy 响应上执行显式 transform，支持 provider / host / path / content-type / status 匹配、header 修改、HTML 注入、本地 asset、replace 和 Go transformer 扩展点。

### Provider 架构与 Steam 加速主线

当前 provider registry 已落地：

- `providers.enabled` 默认是 `[steam]`。
- Steam 是默认 stable provider，承载现有 Steam rules、outbound profiles 和 startup probes。
- GitHub 是显式启用的 experimental skeleton provider，承载 GitHub 域名规则和 startup probes，但没有默认 outbound profile，也不承诺真实加速。
- `proxy.non_target_behavior` 取代旧 `proxy.non_steam_behavior`。
- 旧 `rules.enable_default_steam_rules`、`upstream.enable_default_steam_profiles`、`proxy.non_steam_behavior` 会返回迁移错误。
- CLI 使用 `start --non-target reject|direct`；旧 `--non-steam` 会返回迁移错误。

当前 Steam provider 已经覆盖：

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

### DNSIntercept manual / system

v0.7.1 已实现 DNSIntercept manual 基础，v0.7.2 已增加 Windows system 显式接管：

- 配置入口：`mode: dns` 和 `dns_intercept`。
- CLI 覆盖：`start --mode dns --dns-listen 127.0.0.1:15353`。
- 本地 DNS server 同时支持 UDP / TCP。
- 目标域名规则来自 enabled providers、`rules.custom_domains` 和 `hosts.extra_domains`。
- 目标 `A` / `AAAA` 返回本地映射；目标 `HTTPS` / `SVCB` 默认 NODATA；其他目标记录类型默认不转发。
- 非目标 DNS 查询转发到显式 resolver 或 DNS 模式下的 DoH 默认上游，避免系统 DNS 自绕回本机。
- 内置 DNS response cache、超时、端口冲突检测和 status 计数。
- `status` 输出 `dns_intercept: strategy=manual listen=... system_dns=false target=... forwarded=... cache_hits=... blocked=... errors=...`。
- manual 模式不会修改系统 DNS、hosts、Root CA 信任、浏览器配置或任何持久化系统设置；停止进程即可恢复。
- system 模式要求 `mode: dns`、`dns_intercept.strategy: system`、`listen_addr: 127.0.0.1:53` 和显式 `dns_intercept.interfaces`。
- system 模式启动顺序为 preflight -> 启动本地 DNS server -> 写入系统 DNS；停止时先恢复系统 DNS，再关闭本地 DNS server。
- system 模式的 rollback kind 为 `system_dns`，记录每个 interface 原本的 DHCP / static DNS 状态；`restore` 可独立恢复。
- AppHost 白名单新增 `preflight-system-dns`、`apply-system-dns`、`restore-system-dns`，请求只能携带 loopback DNS server、显式接口和允许的 rollback 路径。
- `status` 在 system 模式下输出 `dns_intercept: strategy=system ... system_dns=true ...` 和 `system_change: component=system_dns ...`。

高端口 manual smoke 只能验证 DNS 决策和 server 行为。system smoke 会真实修改指定 Windows 网卡 DNS，必须先确认接口名、AppHost 健康和 rollback 路径，测试后确认 DNS 回到启动前状态。

### Page Enhance

v0.7.3 已实现默认关闭的页面增强能力：

- 配置入口：`page_enhance.enabled`、`on_error`、`max_body_size`、`assets`、`transforms`。
- 匹配条件：provider、host、path prefix、content type、status code。
- 内置机械能力：header set/remove、HTML head/body 注入、本地 asset serving、简单 replace。
- Go 扩展点：`internal/pageenhance.Transformer`，通过 `ResponseMeta` 显式判断是否匹配和是否需要 body。
- 错误策略：`pass_through` 恢复原始响应，`fail_closed` 返回 transform 错误。
- 可观察性：`page_enhance_*` log event 和 `page_enhance:` status 计数显示 apply、skip、error。
- 不做隐藏安全跳过；库不会自行跳过 login、checkout 或任意路径，是否增强由开发者显式配置。
- 不写系统 DNS、hosts、证书、浏览器配置或开发者环境；关闭 `page_enhance.enabled` 或移除 transform 即可恢复原始响应行为。

### Windows AppHost Service

当前代码已经实现 Windows `SiteBoostCoreAppHost` 服务：

- CLI：`apphost install|start|stop|status|uninstall|run`
- 隐藏入口：`__apphost-service`
- IPC：Windows named pipe `\\.\pipe\SiteBoostCoreAppHost`
- named pipe 使用 DACL、本机连接限制、pipe client PID 和客户端二进制路径校验
- 服务启动类型：`StartAutomatic`
- 服务延迟启动：`DelayedAutoStart`
- 已安装旧服务时会升级配置并重启服务
- Windows 系统修改默认走 AppHost named pipe RPC，包括 Root CA、hosts、system DNS、restore 等受限操作
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
- GitHub skeleton provider 只用于验证架构扩展，不应文档承诺真实加速。
- Provider 不写 hosts、不装证书、不调 AppHost、不拥有 DNSIntercept/TUN/Page Enhance 执行职责；这些边界已作为 v0.7 的约束。
- 下一阶段的关键不是继续堆 Steam 域名，而是补齐 Page Enhance 真实 smoke、provider 开发文档、诊断、恢复和未来独立 Go library 的迁移边界。

## 当前仍存在的风险

- AppHost `install -> no-admin Hosts loop -> stop/restore -> uninstall` 主流程已通过；`reboot -> apphost auto-start -> no-admin start` 仍建议补充真实 Windows 重启 smoke 记录。
- AppHost RPC 使用 Windows named pipe，命令面受限，并带 DACL、本机连接限制、pipe client PID 和客户端二进制路径校验；后续仍需要评估用户会话绑定、审计日志和按需启动。
- Go module 和 CLI 仍带 Steam 专用命名，这是实验仓库历史包袱；正式 Go library 应在新仓库内使用中性命名。
- 公共 Go API 尚未抽出，核心仍主要在 `internal/`；正式 public API 应在未来新仓库内冻结。
- rollback state schema 没有版本化迁移。
- installer、服务升级、卸载清理、日志位置、发布包、签名还未产品化。
- Hosts 模式只能覆盖 exact 域名；DNSIntercept manual 可覆盖 wildcard，但只在显式 `mode: dns` 下启动，不会自动接管系统 DNS。DNSIntercept system 可显式接管 Windows 指定网卡 DNS，当前已有真实 apply / restore smoke，负向场景仍需后续补充。
- Page Enhance 默认关闭且不修改系统环境，但启用后会改写响应内容；真实页面级 smoke 仍建议补充。
- macOS / Linux 的 Hosts、证书和权限闭环没有落地。
- `curl.exe` 如报 `CRYPT_E_NO_REVOCATION_CHECK`，通常是 Windows Schannel 吊销检查问题；可用 `curl.exe --ssl-no-revoke -I https://steamcommunity.com/` 辅助验证。

## 代码入口

优先阅读这些文件：

- `cmd/steam-accelerator/main.go`：CLI、隐藏 helper / apphost 入口、命令分发。
- `internal/engine/engine.go`：启动编排、Hosts 模式、Root CA、hosts 写入、rollback、system_change。
- `internal/provider`：provider registry、Steam stable provider、GitHub experimental skeleton provider。
- `internal/privilege/privilege.go`：受限系统修改请求模型和 AppHost 调用入口。
- `internal/privilege/privilege_windows.go`：Windows AppHost Service、RPC server、Windows 权限检测。
- `internal/rules/rules.go`：通用规则匹配、规则元信息。
- `internal/dnsintercept/server.go`：DNSIntercept manual 本地 UDP/TCP DNS server、决策、缓存、转发和 status。
- `internal/systemdns`：Windows system DNS 快照、rollback、apply / restore 和 PowerShell 后端。
- `internal/pageenhance`：Page Enhance 配置转换、transform pipeline、asset serving、事件和状态计数。
- `internal/upstream/profile.go`：通用 outbound profile、ForwardHost、TLS SNI、candidate dialing。
- `internal/resolver/resolver.go`：system / udp / tcp / doh resolver。
- `internal/reverse/reverse.go`：本地 HTTP / HTTPS reverse proxy、WebSocket、502 诊断。
- `internal/certstore/platform_windows.go`：Windows 证书存储安装 / 查询 / 卸载。
- `internal/hosts/hosts.go` 与 `internal/hosts/platform_windows.go`：hosts marker block、preflight、写入与恢复。

优先阅读这些文档：

- `ROADMAP.md`
- `docs/zh/roadmap.md`
- `docs/zh/capability-boundary.md`
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
go run .\cmd\steam-accelerator --version
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

1. 跑完 `v0.7.3-dev` 自动化验证：`git diff --check`、`go test ./...`、`go vet ./...`、race 子集、build、version。
2. 补做 Page Enhance 高端口 manual smoke：`mode: dns`、`dns_intercept.strategy: manual`、`hosts.http_listen_addr: 127.0.0.1:28080`，用本地 asset 验证 `page_enhance:` status，不修改系统环境。
3. 补做默认 Steam Hosts + DoH + AppHost 真实 Windows smoke，并确认 `status` 同时显示 `provider: id=steam ...`、兼容 `rule_set: steam-web@...` 和 Page Enhance 默认不输出。
4. 补做 `providers.enabled: [steam, github]` skeleton smoke：确认 GitHub 显示 `experimental`，但不要求 GitHub live 可达，也不写成真实加速能力。
5. 补做单独重启 smoke：`reboot -> apphost status health=ok -> normal-user start --mode hosts -> stop/restore`，并把输出补进 smoke 文档。
6. 保留 DNSIntercept manual 与 system smoke 记录，后续补 AppHost 缺失、端口占用、崩溃后 restore 等负向场景。
7. 整理未来独立 Go library 的 API 草案和迁移清单：`Config`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`、DNSIntercept 策略、Page Enhance pipeline。
8. 继续设计 AppHost IPC 加固：优先评估用户会话绑定、审计日志和按需启动。
9. 进入 release engineering：CI matrix、rollback schema version、installer、服务升级 / 卸载、签名规划。
10. 在验证充分后新建正式 Go library 仓库，从本仓库迁移已验证的 resolver、reverse、certstore、privilege、provider、dnsintercept、systemdns、pageenhance 和 diagnostics 能力。

## 提醒新 session 的边界

- 不要使用 `git reset --hard` 或 `git checkout --` 回滚用户改动。
- 不要提交 `bin/`、runtime state、日志、证书、私钥、`.env`。
- 不要把 SteamTools 源码复制进仓库。
- 不要承诺 GitHub 已可加速；当前只是 experimental skeleton provider。
- 不要把 Page Enhance 写成默认开启；它必须显式启用、可观察、可关闭。
- 不要把 HTTP / SOCKS5 upstream 写成默认必需能力。
- 不要把 AppHost 描述成 UAC 绕过；它是一次管理员安装后的受控系统服务。
- 不要把本仓库描述成未来正式 Go library；正式库会另起仓库，本仓库只做实验验证和迁移来源。
- 每次改动后先跑相关验证，再本地提交。
