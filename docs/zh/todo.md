# Todo

## 短期任务

- 推进 `v0.7.0` 通用加速核心重构和命名迁移准备。
- 审计代码、配置、CLI、状态输出和文档中的 Steam 专用命名与硬编码假设。
- 设计 provider / rule pack / outbound profile / takeover mode / restore state 的通用模型。
- 增加一个非 Steam 示例 provider，证明核心可支持其他站点或服务的本地加速。
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
- Windows 普通 PowerShell 已支持通过已安装 AppHost named pipe 完成默认 Hosts / Root CA / restore 系统修改；自定义 hosts / cert / rollback 路径仍需要管理员进程或后续受控桌面集成。
- Hosts 文件不能表达通配符，当前 Hosts 模式只写入 exact 域名。
- Linux 桌面系统代理处理延后。
- DNSIntercept、VPN / TUN、JS 注入不进入 v1.0 范围，但进入 v1.x 高级能力路线。
- `v1.0.0` 前公共 API 不稳定。
