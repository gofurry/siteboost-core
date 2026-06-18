# 恢复说明

## 恢复目标

`steam-accelerator restore` 的目标是恢复本项目修改过的系统状态，包括：

- System Proxy。
- PAC。
- hosts 标记区块。
- Windows Hosts 模式的本地反代状态。

该命令已在 `v0.3.0` 引入 PAC 与 System Proxy 恢复，并在 `v0.4.0` 支持删除 Hosts 模式写入的项目标记区块。

注意：`restore` 不会卸载用户显式安装的本项目 Root CA。证书生命周期由 `cert install` / `cert uninstall` 控制。

## 设计要求

- 每次修改系统状态前保存原值或 rollback 状态。
- rollback 状态应落在用户配置目录，而不是项目源码目录。
- hosts 只删除项目标记区块。
- Root CA 卸载必须由用户显式执行 `cert uninstall`。
- 恢复失败时必须返回清晰错误。
- 恢复命令应能在服务未运行时独立执行。

## 手动恢复方向

- Windows：从 rollback 状态恢复 HKCU WinINet 代理设置。
- Windows Hosts：从 rollback 状态删除 hosts 文件中的项目标记区块。
- macOS：从 rollback 状态恢复 `networksetup` 管理的 PAC、HTTP Proxy、HTTPS Proxy。
- macOS / Linux Hosts 与证书安装：v0.4.0 明确返回不支持。

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
- Hosts 模式下执行 `restore` 能移除项目标记区块。
- 重复执行 `restore` 不应破坏用户原有配置。
- `cert uninstall` 只卸载本项目 Root CA，不删除用户其他证书。
