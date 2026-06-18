# SteamTools 参考边界

## 参考来源

本项目参考 BeyondDimension/SteamTools，也就是 Watt Toolkit 的公开信息：

- 仓库：https://github.com/BeyondDimension/SteamTools
- README 中的网络加速说明：使用 YARP.ReverseProxy 进行本地反代。
- `ProxyMode` 枚举：`DNSIntercept`、`Hosts`、`System`、`VPN`、`ProxyOnly`、`PAC`。
- SteamTools 许可证：GPL-3.0。

截至 2026-06-18，这些信息用于本项目的架构边界设计，而不是作为代码移植来源。

## 可借鉴内容

- 本地反代可以作为网络加速核心。
- 代理接管方式应拆成多个模式，而不是一个大开关。
- 加速服务需要独立生命周期：启动、停止、恢复、异常退出恢复。
- 证书能力应独立成模块：Root CA、本地证书、动态签发、安装、卸载。
- DNS / DoH 应属于加速核心的一部分，而不是外围诊断功能。
- 上游出口应可配置：Direct、HTTP Proxy、SOCKS5。
- Steam 域名规则应配置化、分组化，并由 PAC、Proxy、Hosts、Reverse Proxy 共用。

## 不借鉴内容

- 不做完整工具箱。
- 不做 Steam 账号、令牌、库存、成就、挂卡等非网络能力。
- 不做 JS 注入。
- 首期不做 DNSIntercept。
- 首期不做 VPN / TUN。
- 不做公共代理池或节点服务。

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
- `DNSIntercept`、VPN / TUN 和 JS 注入进入延后路线或明确不做。

## 相关文档

- [中文主路线图](../../ROADMAP.md)
- [中文使用说明](./usage.md)
- [中文冒烟测试](./smoke-test.md)
- [中文 Todo](./todo.md)
