# steam-accelerator-core 中文 Roadmap

> 状态日期：2026-06-22
> 主维护语言：中文
> 当前阶段：`v0.6.3` 已完成 Windows 受限提权 helper 一键启动，下一阶段进入 `v0.7.0` 通用加速核心重构与命名迁移准备
> Go module：`github.com/gofurry/go-steam-core`

## 当前定位

`steam-accelerator-core` 当前是一个 Go 版 Steam 本地网络加速原子能力内核。它不是完整 Steam++ / Watt Toolkit 替代品，也不覆盖账号、库存、成就、挂卡、游戏下载、UI 面板或公共代理节点。

项目短期目标是先把 Steam 场景的一键闭环验证扎实；中长期目标是沉淀可被 SteamScope、steam-go、Go/Wails 桌面工具或本地 sidecar 复用的底层能力，并在功能验证后重构为更通用的本地网络加速核心。后续仓库和 Go module 可能会从 Steam 专用命名迁移到领域无关命名，让 Steam 变成内置规则 / profile 包之一，而不是核心唯一目标。

当前 `v0.6.3` 已经具备核心零件，并完成第一版 Hosts + DoH 默认闭环、出站失败诊断、默认 Steam 出站 profile、启动探测、Windows 中国网络真实 smoke 记录、Windows 一键证书 / Hosts 编排、机器级默认证书写入和受限提权 helper：

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
- Hosts + Direct 模式默认启用 Steam 出站 profile，核心 store / checkout / help / login / media 域名优先走 `cdn-a.akamaihd.net`，community 域名优先走 `steamcommunity-a.akamaihd.net`，并覆盖 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 这类常见静态资源 / CDN 域名。HTTP Host 保留原始 Steam 域名，TLS SNI 按 profile 使用可达 CDN 域名，并保留原始域名 fallback。
- Hosts + Direct 模式会执行非致命启动探测，并在 `start` / `status` 输出 `startup_probes` 和失败阶段，覆盖 DoH 解析、TCP 443、TLS 握手和轻量 HTTPS `HEAD /`。
- `start --mode hosts` 默认会在一键流程内检查并安装本项目 Root CA；已安装时静默跳过，未安装时走 Windows 证书库 API。默认写入 `LocalMachine\Root`，用于管理员运行 Hosts 模式时减少首次确认；`cert.store_scope: user` 可切回 `CurrentUser\Root`。
- 普通 PowerShell 下的 Windows Hosts 模式会在需要写 `LocalMachine\Root` 或 Windows hosts 时，通过同一可执行文件的隐藏 helper 入口和 Windows `ShellExecute/runas` 主动请求一次 UAC；管理员进程仍走静默直接路径。
- helper 只接受白名单命令：`prepare-hosts-start`、`trust-root-ca`、`restore-hosts`、`untrust-root-ca`，并通过 token、父进程 PID、默认路径约束和超时保护限制提权面。
- `status` / `start` 会输出 `system_change`，展示 Root CA、hosts preflight、反代监听和 hosts 写入结果。

距离完整 Steam++ 式能力仍有差距。主要差距是：

- 真实 Steam 域名、Steam 客户端内置浏览器、网页登录、社区、商店、聊天、静态资源和 WebSocket 场景已经完成至少一轮 Windows 中国网络手动通过记录。
- hosts 文件仍只能覆盖 exact 域名，wildcard 规则需要后续 DNSIntercept 等高级模式。
- Windows 普通进程已能主动请求 UAC 完成 Root CA、hosts 和 restore，但这是显式系统授权，不是绕过 UAC；自定义 hosts / cert / rollback 路径会被 helper 限制，复杂桌面分发仍需要后续独立 AppHost / installer 打包设计。
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
一键编排 Root CA 信任、hosts 写入、权限检查和失败恢复
        ↓
DoH / 指定 DNS 解析真实 Steam IP
        ↓
Direct 按 Steam profile 优先尝试 ForwardDestination / TLS SNI，再 fallback 原始域名
        ↓
stop / restore 可恢复系统修改
```

排序原则：

1. 已先修正 Hosts 模式默认闭环和 DNS 自绕回风险。
2. 接下来用真实 Steam 场景验证规则、证书、反代和恢复路径。
3. 已实现 Windows 受限 elevated helper，让普通启动路径可以主动触发一次 UAC，而不是要求用户手动打开管理员终端。
4. 功能验证后进行通用加速核心重构，让规则、profile、接管模式、证书和权限能力从 Steam 业务命名中解耦。
5. 之后稳定跨平台能力和 Go 集成 API。
6. `v1.0.0` 以可维护、可扩展的一键本地加速闭环作为稳定主线。
7. DNSIntercept、VPN / TUN、JS 注入进入 `v1.x` 高级能力路线，但不阻塞 `v1.0.0`。

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

**状态：** 已完成
**范围：** User-facing / Testing / Documentation / Stability
**目标：** 用真实 Steam 访问场景验证一键闭环，补齐 Steam 默认出站 profile、域名规则和手动验收记录。

#### 重点

- Steam 商店、社区、登录、聊天、静态资源和 WebSocket 场景。
- 规则覆盖率与 hosts exact 域名清单。
- Steam 默认出站 profile：候选 IP、ForwardDestination、TLS / SNI 策略和失败 fallback。
- 真实环境 smoke test。
- 基于 v0.5.1 诊断输出定位真实网络失败。

#### 任务

- [x] 建立真实 Steam 域名兼容性清单，覆盖 store、community、login、api、chat、static、cdn 等分组。
- [x] 为 Hosts 模式维护 exact 域名写入清单，明确 wildcard 规则无法通过 hosts 覆盖的范围。
- [x] 为核心 Steam 域名设计并实现默认出站 profile 骨架，支持候选 IP、ForwardDestination、TLS SNI、证书名不匹配策略和 fallback 顺序。
- [x] 为 `steamcommunity.com` / `*.steamcommunity.com` 增加默认 `steamcommunity-a.akamaihd.net` fallback；为 store / checkout / help / login / media 增加默认 `cdn-a.akamaihd.net` fallback；补齐 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 的默认接管与 profile 覆盖。
- [x] 增加 YAML 自定义 outbound profile 配置和校验，允许用户覆盖 `match_domains`、`candidate_ips`、`forward_host`、`tls_server_name` 和 `ignore_tls_name_mismatch`。
- [x] 增加启动前真实探测：DoH 解析、TCP 443 连通、TLS 握手和轻量 HTTP smoke。
- [x] 增加真实 Steam 访问手动 smoke 文档模板，覆盖 Windows 管理员终端、证书安装、启动、访问、停止、恢复全过程。
- [x] 增加常见失败诊断文档：DNS 失败、证书不受信、端口占用、hosts 被安全软件拦截、WebSocket 失败。
- [x] 完成至少一轮 Windows 真实 Steam 商店 / 社区 / 登录 / 静态资源 / WebSocket 手动验收记录。
- [x] 增加代理请求、反代请求和 resolver 失败的脱敏日志字段扩展，确保不泄露 Cookie / Authorization / URL secret。
- [x] 增加内置规则版本号和规则更新时间，供 status、日志和后续 provider/rule pack 演进使用。

#### 验收标准

- 至少完成一轮 Windows 真实 Steam 商店 / 社区 / 登录 / 静态资源 / WebSocket 手动验收记录。
- 默认规则能覆盖最常见的 Steam Web 访问链路。
- 未覆盖域名和失败原因可以从日志或 status 输出中定位。
- 文档明确区分“本地反代加速”和“外部代理节点加速”。

---

### v0.6.1 - Windows 一键提权与证书写入封装

**状态：** 已完成
**范围：** User-facing / Security-Safety / Windows / Architecture
**目标：** 将 Windows Root CA 写入、hosts 写入、启动检查和失败恢复封装进核心一键流程，并明确后续提权 helper 的安全边界。

#### 重点

- 提权 helper / IPC / 子进程模型边界。
- Windows 证书库 API 后端，默认 `LocalMachine\Root`，并保留 `CurrentUser\Root` 兼容配置，替代裸 `certutil` 调用。
- Root CA 幂等安装、状态检查和卸载体验。
- 一键启动中的证书信任、失败回滚和安全提示。

#### 任务

- [x] 设计 Windows privileged helper / IPC 边界：主进程负责用户交互和状态，提权进程只执行受限系统修改，详见 `docs/zh/windows-one-click-flow.md`。
- [x] 评估并实现 Windows 证书库 API 后端，支持按 thumbprint 查询、安装和删除本项目 Root CA。
- [x] 将 `start --mode hosts` 的一键流程扩展为可选自动确认证书状态：未安装时在显式启动流程内安装，已安装时静默跳过。
- [x] 将 hosts 写入、Root CA 写入、端口监听和 rollback 状态纳入同一个启动编排，任一步失败都给出可执行恢复建议。
- [x] 增加 `status` / 诊断输出，显示证书信任状态、hosts 写入状态和最近一次系统修改结果。
- [x] 增加 Windows 单元测试和可手动验证脚本，覆盖已安装跳过、安装失败、卸载失败和恢复路径。
- [x] 文档明确安全边界：核心不接受任意系统修改，后续桌面壳 / helper 只暴露最小命令面，避免重复弹框和黑盒修改。

#### 验收标准

- 用户从未安装证书的 Windows 环境启动 Hosts 模式时，在进程已具备 hosts 写入权限的前提下，可以一次 `start` 完成证书信任、hosts 写入和反代启动。
- 已安装本项目 Root CA 时，重复启动不会再次触发证书安装动作。
- 系统修改失败时不会留下不可解释的半成品状态，`restore` 仍可执行。
- 证书私钥不进入日志，后续 helper 契约只暴露最小必要命令。

---

### v0.6.2 - Windows 机器级默认证书写入

**状态：** 已完成
**范围：** User-facing / Security-Safety / Windows
**目标：** 将 Windows Root CA 默认写入 `LocalMachine\Root`，减少 `CurrentUser\Root` 首次确认框，同时保留兼容退路。

#### 任务

- [x] 增加 `cert.store_scope` 配置，支持 `machine` 和 `user`。
- [x] 将 Windows 默认 Root store scope 调整为 `machine`，管理员运行 Hosts 模式时写入 `LocalMachine\Root`。
- [x] 保留 `cert.store_scope: user` 作为 `CurrentUser\Root` 兼容路径。
- [x] 在 `system_change` 中显示 Root CA 写入 scope，例如 `detail=store=machine,installed`。
- [x] 更新 usage、security、smoke 和一键流程文档，说明权限边界和测试方式。

#### 验收标准

- 管理员 PowerShell 下首次 `start --mode hosts` 可以静默安装 Root CA。
- 已安装 Root CA 时重复启动不会重复安装。
- 非管理员写 `LocalMachine\Root` 失败时给出明确提示：以 Administrator 运行或改用 `cert.store_scope: user`。

---

### v0.6.3 - Windows 提权 helper 一键启动

**状态：** 已完成
**范围：** User-facing / Security-Safety / Windows / Architecture / Testing
**目标：** 让普通启动路径可以像 Steam++ 一样由程序主动触发一次 UAC，拉起受限 elevated helper / AppHost 完成 Root CA、hosts 和恢复动作，而不是要求用户手动打开管理员 PowerShell。

#### 重点

- 同一可执行文件提供隐藏 `__helper` 入口，由主进程通过 Windows `ShellExecute/runas` 拉起；独立 `siteboost-helper.exe` / AppHost 和 manifest 打包留给后续桌面壳或发布包。
- 主进程检测非管理员状态后，通过 Windows `ShellExecute` / `runas` 拉起 helper。
- helper 只暴露受限命令，不提供任意 shell、任意文件写入或代理凭据访问。
- helper 和主进程之间使用临时 JSON request / response 文件作为窄 IPC，带 token、父进程校验、命令白名单、路径约束和超时。
- 普通 CLI、未来桌面壳和 Go 集成 API 都能复用同一套 privilege boundary。

#### 任务

- [x] 新增 Windows privilege 包，封装管理员检测、`runas` 启动、helper 路径定位和 helper 响应等待。
- [x] 增加隐藏 helper 入口，只允许执行 `prepare-hosts-start`、`trust-root-ca`、`restore-hosts`、`untrust-root-ca` 白名单命令。
- [x] 采用同一可执行文件 hidden helper + `ShellExecute/runas` 的等价 AppHost 路径；独立 helper manifest / installer 打包转入后续桌面集成与发布工程。
- [x] 将 `start --mode hosts` 在非管理员时改为自动请求 helper；管理员进程仍走当前直接路径。
- [x] 将 hosts 写入、Root CA 写入、restore、cert uninstall 接入 helper，保持 rollback 和 `system_change` 输出一致。
- [x] 增加 helper IPC 的 token、父进程 PID、命令白名单、路径约束和超时保护。
- [x] 增加 privilege 边界单元测试和 engine helper 分支测试；真实 UAC、取消授权和普通 PowerShell 路径通过 smoke 文档手动验证。
- [x] 更新安全文档，明确这是显式 UAC 提权，不是绕过 UAC；参考 Steam++ 的提权边界思想，不复制 SteamTools 源码。

#### 验收标准

- 普通 PowerShell 执行 `start --mode hosts` 时，程序可以主动触发一次 Windows UAC，并在授权后完成 Root CA 安装、hosts 写入和反代启动。
- 用户取消 UAC 时，命令返回可理解错误，不修改 hosts，不留下半成品 rollback 状态。
- helper 不能执行任意 shell 命令，不能写入非项目允许的系统路径，不能接收 Cookie、代理密码或其他用户秘密。
- `stop` / `restore` 在普通启动路径下也能通过 helper 恢复项目 hosts 修改。
- 管理员 PowerShell 下仍保持 v0.6.2 的静默机器级证书写入路径。
- 非管理员 helper 只支持默认 Windows hosts 路径、默认项目 runtime/cert 目录下的 rollback 与证书；自定义系统路径需要管理员进程或后续受控桌面集成。

#### 说明

- 该能力可以实现，但不会也不应该绕过 UAC。目标是把“手动以管理员运行”变成“程序在需要时请求一次系统授权”。
- Steam++ / Watt Toolkit 的公开源码中也能看到 `requireAdministrator` manifest、`IPCRoot` 和 `runas` 提权子进程思路；本项目只参考架构边界，保持 clean-room 实现。

---

### v0.7.0 - 通用加速核心重构与命名迁移准备

**状态：** 计划中
**范围：** Architecture / Developer-facing / Maintainability / Documentation
**目标：** 在 Steam 一键功能验证后，把项目从 Steam 专用核心演进为可维护、可扩展的通用本地加速核心。

#### 重点

- 领域无关的规则包、profile 包和接管模式抽象。
- Steam 作为内置 provider / rule pack，而不是硬编码在核心层。
- 仓库改名、module 路径、CLI 名称和配置兼容策略。
- 包边界、公共 API 和内部实现的可维护性重构。

#### 任务

- [ ] 审计代码、配置、CLI、状态输出和文档中的 Steam 专用命名与硬编码假设。
- [ ] 设计通用模型：`rule pack`、`provider profile`、`target group`、`outbound profile`、`takeover mode`、`restore state`。
- [ ] 将 Steam 默认规则和 outbound profile 收敛为内置 Steam provider，核心逻辑只依赖通用接口。
- [ ] 制定仓库改名、Go module 迁移、CLI 命令迁移和配置字段迁移方案；在 v1 前允许必要破坏性变更，但必须提供迁移文档。
- [ ] 整理包边界，降低 `internal` 子包之间的耦合，明确 resolver、upstream、reverse、certstore、privilege、runtime 的职责。
- [ ] 增加非 Steam 示例 provider，用最小规则证明核心可支持其他站点或服务的本地加速。
- [ ] 增加迁移测试和兼容性检查，避免重构破坏已验证的 Steam 一键闭环。

#### 验收标准

- 核心层不再依赖 Steam 专用命名才能工作。
- 新增一个非 Steam provider 不需要修改 reverse / resolver / upstream 核心逻辑。
- 迁移文档能说明旧仓库名、旧 module、旧 CLI 和旧配置如何过渡。
- Steam 一键闭环在重构后仍通过 smoke 验收。

---

### v0.8.0 - 跨平台闭环与集成 API 候选

**状态：** 计划中
**范围：** Cross-platform / Developer-facing / API / Documentation
**目标：** 在通用核心边界清晰后，将一键闭环从 Windows-first 扩展为可评估的跨平台能力，并形成 Go 集成 API 候选。

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
- [ ] 冻结 provider / rule pack / outbound profile 的扩展接口。
- [ ] 完成 CLI 使用示例、Go package 示例和 Wails 集成建议。
- [ ] 完成安全边界说明：Root CA、hosts、系统代理、DoH、日志脱敏、restore。
- [ ] 完成 CHANGELOG、release notes、版本标签和发布检查清单。
- [ ] 完成 Windows 一键 Hosts + DoH 闭环最终验收。
- [ ] 保留 ProxyOnly、PAC、System Proxy 作为可选模式和调试模式。

#### 验收标准

- 用户可以按文档完成一键启动、访问 Steam、停止和恢复。
- 默认不需要配置外部上游代理。
- 通用 provider 模型稳定，Steam 只是内置 provider 之一。
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

- 聚焦 `v0.7.0`，完成通用加速核心重构和可能改名的迁移准备，把 Steam 规则 / profile 收敛为内置 provider，而不是核心唯一目标。

中期：

- 用 `v0.7.0` 完成通用加速核心重构和可能改名的迁移准备，再用 `v0.8.0` 完成跨平台评估和 Go 集成 API 候选。

长期：

- `v1.0.0` 稳定发布后，在 `v1.x` 中逐步加入 DNSIntercept、VPN / TUN、JS 注入等高级能力；这些能力应服务通用加速核心，而不只服务 Steam。

## 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| Hosts 模式 system resolver 自绕回 | 反代连接回本机导致访问失败 | v0.5 强制 Hosts 出站解析避开 system resolver |
| Root CA 用户不信任 | 安全信任问题 | 默认明确提示、显式安装、显式卸载、文档说明 |
| 提权 / 证书写入体验割裂 | 无法达到 Steam++ 式一键体验 | v0.6.3 已实现受限 elevated helper，由程序主动请求一次 UAC，不绕过系统安全边界；独立 AppHost / installer 留给后续发布工程 |
| hosts 写入失败 | 用户网络异常 | 标记区块、rollback、restore、管理员权限检查 |
| 80 / 443 端口占用 | 一键启动失败 | 启动前检查并提示占用进程或替代测试端口 |
| Steam 域名变化 | 覆盖不足 | 规则分组、手动 smoke、规则版本化评估 |
| 通用化重构范围过大 | 破坏已验证 Steam 闭环 | 功能验证后再重构，保留 smoke 回归和迁移文档 |
| 仓库 / module 改名 | 用户集成破坏 | v1 前集中处理，提供兼容策略和迁移说明 |
| DNS / DoH 服务失效 | 加速不可用 | 多服务器 fallback、用户可覆盖配置 |
| 高级模式复杂度过高 | v1.0 被拖慢 | DNSIntercept / VPN / JS 注入放入 v1.x，不阻塞 v1.0 |
| 复制 SteamTools 代码 | GPL 和维护边界风险 | 坚持 clean-room，只参考架构思想 |
