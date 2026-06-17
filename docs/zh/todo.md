# Todo

## 短期任务

- 完成 `v0.1.0` 包边界。
- 增加配置默认值与校验。
- 增加 Steam 默认域名规则。
- 增加 rules matcher 测试。
- 增加 HTTP Proxy 与 CONNECT 骨架。
- 增加 Engine 启动、停止、状态。
- 增加 ProxyOnly CLI 命令。

## 中期任务

- 增加 resolver 与 DoH 实现。
- 增加 DNS 缓存与 fallback。
- 增加 HTTP / SOCKS5 上游代理支持。
- 增加 PAC 生成器与 PAC Server。
- 增加 rollback 状态模型。
- 增加 Windows 与 macOS 系统代理集成。

## 长期想法

- 增加 Hosts 模式反代。
- 增加本地 Root CA 管理。
- 增加动态证书签发。
- 增加 WebSocket 反代覆盖。
- 在 `v1.0.0-alpha.1` 前做 API freeze review。
- 公共 API 稳定后再引入发布自动化。

## 已知限制

- 运行时加速能力尚未实现。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围。
- `v1.0.0` 前公共 API 不稳定。
