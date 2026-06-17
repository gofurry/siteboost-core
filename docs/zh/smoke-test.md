# 冒烟测试

## 快速验证步骤

在仓库根目录运行：

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
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

## 期望输出

版本命令应输出项目名、版本号和模块路径。

basic 示例应输出项目名和模块路径。

前台 `start` 运行时，`status` 应显示 `running: true`。`stop` 应让前台进程退出，并输出 `stopped` 或 `stop requested`。

## 常见失败情况

- 未安装 Go，或 Go 版本低于 `go.mod` 声明。
- 新增 Go 文件未经过 `gofmt`。
- 新增依赖后未运行 `go mod tidy`。
- `127.0.0.1:26501` 端口已被占用。
- 状态文件指向旧进程；`status` 或 `stop` 应自动清理 stale 状态。
