# 冒烟测试

## 快速验证步骤

在仓库根目录运行：

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/hosts ./internal/certstore ./internal/reverse ./internal/pac ./internal/systemproxy ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine ./internal/runtime
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

## CLI 运行时检查

在一个终端启动代理：

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
```

在另一个终端中：

```bash
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

也可以使用显式的 proxy_only 配置文件检查同一条生命周期：

```yaml
mode: proxy_only

resolver:
  mode: "system"
  prefer_ipv4: true

upstream:
  type: "direct"
```

启动命令：

```bash
go run ./cmd/steam-accelerator start --config ./tmp/proxy-system-direct.yaml --state ./tmp/runtime.json
```

DoH 与 HTTP/SOCKS5 upstream 行为由 `go test ./internal/resolver ./internal/upstream ./internal/proxy` 中的本地 fake server 覆盖。手动检查时，可将 `resolver.mode` 配为 `doh` 并显式填写 `servers`，或将 `upstream.type` 配为 `http` / `socks5` 并填写本地代理地址。

## PAC 与 System Proxy 检查

这些检查会修改当前用户的 Windows 或 macOS 系统代理设置，`stop` 应恢复原值。

PAC 模式：

```bash
go run ./cmd/steam-accelerator start --mode pac --state ./tmp/runtime.json
curl http://127.0.0.1:26502/proxy.pac
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

System Proxy 模式：

```bash
go run ./cmd/steam-accelerator start --mode system --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

崩溃恢复：

```bash
go run ./cmd/steam-accelerator restore
```

## Windows Hosts 与 HTTPS Reverse Proxy 检查

这些检查会修改当前用户 Root 证书库和 Windows hosts 文件。请使用管理员 PowerShell 运行 hosts 模式，并在测试完成后执行 `stop` 与 `cert uninstall`。

首次安装本项目 Root CA：

```bash
go run ./cmd/steam-accelerator cert install
```

使用高端口启动，避免占用 80 / 443：

```bash
go run ./cmd/steam-accelerator start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

另一个终端检查状态：

```bash
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
```

真实 hosts 模式默认写入 80 / 443。高端口主要用于验证 reverse server 生命周期；如果要验证浏览器访问真实 Steam 域名，需要使用默认 80 / 443 并确认本机端口未被占用。

停止并卸载证书：

```bash
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
go run ./cmd/steam-accelerator cert uninstall
```

## 期望输出

版本命令应输出项目名、版本号和模块路径。

basic 示例应输出项目名和模块路径。

前台 `start` 运行时，`status` 应显示 `running: true`。`stop` 应让前台进程退出，并输出 `stopped` 或 `stop requested`。

## 常见失败情况

- 未安装 Go，或 Go 版本低于 `go.mod` 声明。
- 新增 Go 文件未经过 `gofmt`。
- 新增依赖后未运行 `go mod tidy`。
- `127.0.0.1:26501` 端口已被占用。
- 非 system resolver 未配置 `resolver.servers`。
- HTTP 或 SOCKS5 upstream 未配置 `upstream.address`。
- PAC 模式下 `127.0.0.1:26502` 端口已被占用。
- 当前系统不是 Windows 或 macOS，不能使用 PAC/System Proxy 模式写系统代理。
- Hosts 模式下 80 / 443 端口已被占用。
- Hosts 模式未先执行 `cert install`。
- Windows hosts 写入失败；请使用管理员终端运行。
- restore 失败后 rollback 状态仍会保留；修复平台错误后再次执行 `restore`。
- 状态文件指向旧进程；`status` 或 `stop` 应自动清理 stale 状态。
