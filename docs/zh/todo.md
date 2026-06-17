# Todo

## 短期任务

- 打磨 `v0.1.0` ProxyOnly CLI 冒烟路径。
- 增加 malformed request 和 dial failure 等代理边界测试。
- 在下一轮文档整理时增加配置示例文件。
- 评估 patch 版本是否给 status 增加 JSON 输出。
- 在公共 API 设计稳定前，继续保持运行时实现为 internal。

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

- 当前只实现 ProxyOnly 运行时。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围。
- `v1.0.0` 前公共 API 不稳定。
