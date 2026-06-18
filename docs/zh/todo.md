# Todo

## 短期任务

- 打磨 `v0.4.0` Hosts / HTTPS Reverse Proxy 冒烟与 restore 路径。
- 增加 malformed request、dial failure 和 upstream failure 等代理边界测试。
- 在下一轮文档整理时增加配置示例文件。
- 评估 patch 版本是否给 status 增加 JSON 输出。
- 在公共 API 设计稳定前，继续保持运行时实现为 internal。

## 中期任务

- 增加 macOS / Linux Hosts 与证书安装支持评估。
- 增加 DNSIntercept 方案设计。
- 增加真实 Steam 域名的手动兼容性测试记录。

## 长期想法

- 增加更多 WebSocket 反代边界覆盖。
- 在 `v1.0.0-alpha.1` 前做 API freeze review。
- 公共 API 稳定后再引入发布自动化。

## 已知限制

- 当前已实现 ProxyOnly、PAC、System Proxy、Windows Hosts、证书与反代能力。
- Hosts 文件不能表达通配符，v0.4.0 只写入 exact 域名。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围。
- `v1.0.0` 前公共 API 不稳定。
