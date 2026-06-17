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

## 必要命令

- `go mod tidy`：验证模块元数据。
- `gofmt -w .`：格式化 Go 源码。
- `go vet ./...`：运行 Go 静态检查。
- `go test ./...`：运行全部测试并构建全部包。
- `go run ./cmd/steam-accelerator --version`：验证 CLI 入口可构建。
- `go run ./examples/basic`：验证 basic 示例可构建。

## 期望输出

CLI 应输出项目名、版本号和模块路径。

basic 示例应输出项目名和模块路径。

## 常见失败情况

- 未安装 Go，或 Go 版本低于 `go.mod` 声明。
- 新增 Go 文件未经过 `gofmt`。
- 新增依赖后未运行 `go mod tidy`。
- 命令提前导入了尚未实现的内部包。
