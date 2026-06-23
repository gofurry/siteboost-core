# Todo

## 短期任务

- 完成 `v0.7.1-dev` DNSIntercept manual 高端口 smoke。
- 完成 `v0.7.1-dev` provider registry 与 Hosts/AppHost 路径的真实 Windows smoke。
- 决定 v0.7.2 是否先实现显式 system DNS 接管，再进入 Page Enhance。
- 在 DNSIntercept / Page Enhance 边界验证后准备 `v0.8.0` 公共 Go library 抽离计划和包边界草案。
- 用 GitHub skeleton provider 补一份最小 provider 开发示例文档。
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
- 分阶段评估 Page Enhance 与显式 system DNS 接管；VPN / TUN 继续延期给成熟外部 adapter。

## 已知限制

- 当前已实现 ProxyOnly、PAC、System Proxy、Windows Hosts、证书与反代能力。
- Steam 是默认 stable provider；GitHub 是显式启用的 experimental skeleton provider，不承诺真实加速。
- Windows 普通 PowerShell 已支持通过已安装 AppHost named pipe 完成默认 Hosts / Root CA / restore 系统修改；自定义 hosts / cert / rollback 路径仍需要管理员进程或后续受控桌面集成。
- Hosts 文件不能表达通配符，当前 Hosts 模式只写入 exact 域名；显式启用 DNSIntercept manual 后可覆盖 wildcard 规则。
- Linux 桌面系统代理处理延后。
- DNSIntercept manual 已实现。system DNS 接管、VPN / TUN、JS 注入 / Page Enhance 仍是后续分阶段能力；VPN / TUN 延期给外部 adapter。
- `v1.0.0` 前公共 API 不稳定。
