# 冒烟测试

## 快速验证步骤

在仓库根目录运行：

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine
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

也可以使用显式的 v0.2.0 配置文件检查同一条生命周期：

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
- 状态文件指向旧进程；`status` 或 `stop` 应自动清理 stale 状态。
