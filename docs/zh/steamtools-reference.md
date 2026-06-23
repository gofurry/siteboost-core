# SteamTools 参考边界

## 参考来源

本项目参考 BeyondDimension/SteamTools，也就是 Watt Toolkit 的公开信息：

- 仓库：https://github.com/BeyondDimension/SteamTools
- README 中的网络加速说明：使用 YARP.ReverseProxy 进行本地反代。
- `ProxyMode` 枚举：`DNSIntercept`、`Hosts`、`System`、`VPN`、`ProxyOnly`、`PAC`。
- SteamTools 许可证：GPL-3.0。

截至 2026-06-21，这些信息用于本项目的架构边界设计，而不是作为代码移植来源。

## 可借鉴内容

- 本地反代可以作为网络加速核心。
- 代理接管方式应拆成多个模式，而不是一个大开关。
- 加速服务需要独立生命周期：启动、停止、恢复、异常退出恢复。
- 证书能力应独立成模块：Root CA、本地证书、动态签发、安装、卸载。
- DNS / DoH 应属于加速核心的一部分，而不是外围诊断功能。
- 上游出口应可配置：Direct、HTTP Proxy、SOCKS5。
- Steam 域名规则应配置化、分组化，并由 PAC、Proxy、Hosts、Reverse Proxy 共用。
- 真实可用的一键体验需要按域名维护出站 profile：原始匹配域名、ForwardDestination、TLS SNI / FakeServerName、候选 IP、证书名称不匹配策略和 fallback 顺序。

## 不进入默认主线的内容

- 不做完整工具箱。
- 不做 Steam 账号、令牌、库存、成就、挂卡等非网络能力。
- 不做公共代理池或节点服务。
- 默认一键闭环不依赖 JS 注入。
- `v1.0.0` 不以 VPN / TUN 为阻塞项。
- DNSIntercept 和 Page Enhance 已在 v0.7.x 前移为抽库前主能力验证，但必须显式开启、可观察、可还原。
- VPN / TUN 进入后续高级能力路线，优先使用成熟外部库或独立项目集成。

## Clean-Room 规则

- 可以阅读公开 README、文档、许可证和模式命名。
- 可以学习架构分层和风险边界。
- 不复制 C# 源码。
- 不翻译 C# 文件为 Go 文件。
- 不搬运具体实现细节、函数结构或私有业务逻辑。
- 新实现必须基于 Go 标准库、独立开源库或本项目自研逻辑。

## 对本项目的落地影响

- `v0.1.0` 先实现 `ProxyOnly`，不改系统状态。
- `v0.3.0` 已实现 PAC 和 System Proxy。
- `v0.4.0` 已实现 Windows-first Hosts、Root CA 与 HTTPS Reverse Proxy。
- `v0.5.0` 已补齐第一版 Hosts + DoH 一键默认闭环，避免 Hosts 模式出站解析自绕回。
- `v0.5.1` 已补齐出站失败诊断和 HTTPS Direct 出口的 TCP + TLS 候选尝试链。
- `v0.6.0` 已落地默认 Steam 出站 profile：community 优先 `steamcommunity-a.akamaihd.net`，store / checkout / help / login / media 优先 `cdn-a.akamaihd.net`，并覆盖 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 这类常见静态资源 / CDN 域名。`v0.7.4` 参考 Steam++ / Watt Toolkit 公开远端加速项目的行为层经验，补齐 `api.steampowered.com -> steamstore.rmbgame.net`。HTTP Host 保留原始 Steam 域名；需要前置域的场景通过显式 profile 处理 TLS SNI / hostname mismatch。
- `v0.7.1` / `v0.7.2` 已落地 DNSIntercept manual 与 Windows system 显式接管；`v0.7.3` 已落地默认关闭的 Page Enhance pipeline。VPN / TUN 继续延期到成熟外部 adapter 或独立项目。

## 相关文档

- [中文主路线图](../../ROADMAP.md)
- [中文使用说明](./usage.md)
- [中文冒烟测试](./smoke-test.md)
- [中文 Todo](./todo.md)
