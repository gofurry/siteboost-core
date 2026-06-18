# Todo

## 短期任务

- 打磨 `v0.3.0` PAC / System Proxy 冒烟与 restore 路径。
- 增加 malformed request、dial failure 和 upstream failure 等代理边界测试。
- 在下一轮文档整理时增加配置示例文件。
- 评估 patch 版本是否给 status 增加 JSON 输出。
- 在公共 API 设计稳定前，继续保持运行时实现为 internal。

## 中期任务

- 增加 Hosts 模式反代。
- 增加本地 Root CA 管理。
- 增加动态证书签发。

## 长期想法

- 增加 WebSocket 反代覆盖。
- 在 `v1.0.0-alpha.1` 前做 API freeze review。
- 公共 API 稳定后再引入发布自动化。

## 已知限制

- 当前已实现 ProxyOnly、PAC、System Proxy；Hosts、证书与反代能力延后。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围。
- `v1.0.0` 前公共 API 不稳定。
