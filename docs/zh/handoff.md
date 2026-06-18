# Handoff

> 更新时间：2026-06-18
> 当前分支：`master`
> 当前目标：`v0.4.0 - Hosts + HTTPS Reverse Proxy`

## 当前状态

`v0.4.0` 已实现 Windows-first Hosts + HTTPS Reverse Proxy 能力，运行时仍保持在 `internal/`，尚未暴露稳定公共 Go API。

当前内核能力：

- ProxyOnly、PAC、System Proxy、Hosts 四种模式。
- 默认 proxy：`127.0.0.1:26501`。
- 默认 PAC Server：`127.0.0.1:26502/proxy.pac`。
- Steam 域名规则匹配、HTTP Proxy、HTTPS CONNECT。
- system / udp / tcp / doh resolver，DNS 缓存与 IPv4 / IPv6 策略。
- direct / http / socks5 upstream。
- Windows HKCU 系统 PAC / HTTP / HTTPS 代理写入与恢复。
- macOS `networksetup` PAC / HTTP / HTTPS 代理写入与恢复。
- Windows hosts 项目标记区块写入与恢复。
- Root CA 生成、Windows 当前用户 Root store 安装/卸载。
- 动态站点证书签发与缓存。
- 本地 HTTP / HTTPS Reverse Proxy，保留 Host 与 SNI，支持 WebSocket upgrade。
- rollback 状态文件与 `restore` 命令。
- `start` / `status` / `stop` / `restore` / `cert install` / `cert uninstall` CLI。

## 常用验证

自动化验证：

```powershell
go mod tidy
gofmt -w .
git diff --check
go vet ./...
go test ./...
go test -race ./internal/hosts ./internal/certstore ./internal/reverse ./internal/pac ./internal/systemproxy ./internal/engine ./internal/runtime
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

PAC 手动检查：

```powershell
go run ./cmd/steam-accelerator start --mode pac --state .\tmp\runtime.json
curl.exe http://127.0.0.1:26502/proxy.pac
go run ./cmd/steam-accelerator status --state .\tmp\runtime.json
go run ./cmd/steam-accelerator stop --state .\tmp\runtime.json
```

System Proxy 手动检查：

```powershell
go run ./cmd/steam-accelerator start --mode system --state .\tmp\runtime.json
go run ./cmd/steam-accelerator status --state .\tmp\runtime.json
go run ./cmd/steam-accelerator stop --state .\tmp\runtime.json
```

崩溃恢复：

```powershell
go run ./cmd/steam-accelerator restore
```

Hosts 手动检查：

```powershell
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts --state .\tmp\runtime.json
go run ./cmd/steam-accelerator status --state .\tmp\runtime.json
go run ./cmd/steam-accelerator stop --state .\tmp\runtime.json
go run ./cmd/steam-accelerator cert uninstall
```

## 下一步建议

下一阶段进入 `v0.5.0 - 稳定性、安全与跨平台打磨`。

建议起手动作：

1. 阅读 `ROADMAP.md`、`docs/zh/roadmap.md`、`docs/zh/restore.md`、`docs/zh/security.md`。
2. 查看最近提交：`git log --oneline -5`。
3. 确认工作区：`git status --short --branch`。
4. 先整理 v0.4 手动 smoke 记录，再推进 v0.5 的错误信息、日志、安全和跨平台打磨。

## 注意事项

- 不复制、不翻译、不移植 SteamTools 源码。
- 默认只监听 `127.0.0.1`。
- 默认只允许 Steam 规则域名。
- rollback state 成功恢复后才删除。
- 日志不得记录 Cookie、Authorization、完整 query、upstream password、证书私钥。
- v0.3.0 不支持 Linux 桌面系统代理写入。
- v0.4.0 Hosts / cert install 是 Windows-first，macOS / Linux 明确 unsupported。
- hosts 文件不能表达 wildcard，当前只写入 exact 域名。
- `restore` 不卸载 Root CA，证书由 `cert uninstall` 显式卸载。
- 临时测试文件建议放在 `tmp/`，不要提交 runtime state 或日志。
