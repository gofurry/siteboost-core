# 恢复说明

## 恢复目标

`steam-accelerator restore` 的目标是恢复本项目修改过的系统状态，包括：

- System Proxy。
- PAC。
- hosts 标记区块。
- 与本项目相关的证书状态。

该命令已在 `v0.3.0` 引入 PAC 与 System Proxy 恢复；hosts 与证书相关恢复会在 `v0.4.0` 引入。

## 设计要求

- 每次修改系统状态前保存原值或 rollback 状态。
- rollback 状态应落在用户配置目录，而不是项目源码目录。
- hosts 只删除项目标记区块。
- 恢复失败时必须返回清晰错误。
- 恢复命令应能在服务未运行时独立执行。

## 手动恢复方向

- Windows：从 rollback 状态恢复 HKCU WinINet 代理设置。
- macOS：从 rollback 状态恢复 `networksetup` 管理的 PAC、HTTP Proxy、HTTPS Proxy。
- Linux：v0.3.0 不统一桌面代理恢复，PAC/System Proxy 模式会明确返回不支持。

手动恢复：

```bash
go run ./cmd/steam-accelerator restore
```

使用隔离 rollback 文件：

```bash
go run ./cmd/steam-accelerator restore --rollback ./tmp/rollback.json
```

## 验收标准

- 模拟崩溃后执行 `restore` 能恢复 PAC 或系统代理。
- v0.4.0 后 Hosts 模式下执行 `restore` 能移除项目标记区块。
- 重复执行 `restore` 不应破坏用户原有配置。
