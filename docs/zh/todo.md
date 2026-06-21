# Todo

## 短期任务

- 推进 `v0.6.0` 真实 Steam 访问验收，覆盖商店、社区、登录、聊天、静态资源和 WebSocket。
- 维护 Hosts exact 域名写入清单，标注 wildcard 规则无法通过 hosts 覆盖的范围。
- 基于 v0.5.1 出站诊断结果设计 Steam 默认出站 profile。
- 增加 ForwardDestination、TLS / SNI pattern、候选 IP fallback 的最小实现方案。
- 增加 malformed request、dial failure 和 upstream failure 等代理边界测试。
- 在下一轮文档整理时增加配置示例文件。
- 在公共 API 设计稳定前，继续保持运行时实现为 internal。

## 中期任务

- 增加 macOS / Linux Hosts 与证书安装支持评估。
- 评估 patch 版本是否给 status 增加 JSON 输出。

## 长期想法

- 增加更多 WebSocket 反代边界覆盖。
- 在 `v1.0.0-alpha.1` 前做 API freeze review。
- 公共 API 稳定后再引入发布自动化。
- 在 `v1.x` 分阶段评估 DNSIntercept、VPN / TUN、JS 注入等高级 Steam++ 能力。

## 已知限制

- 当前已实现 ProxyOnly、PAC、System Proxy、Windows Hosts、证书与反代能力。
- Hosts 文件不能表达通配符，当前 Hosts 模式只写入 exact 域名。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围，但进入 v1.x 高级能力路线。
- `v1.0.0` 前公共 API 不稳定。
