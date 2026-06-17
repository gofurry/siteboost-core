# 使用说明

## 安装方式

本地开发：

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

当前项目仍是脚手架。真实加速命令会在 `v0.1.0` 里逐步引入。

## 基本用法

运行脚手架 CLI：

```bash
go run ./cmd/steam-accelerator --version
```

运行 basic 示例：

```bash
go run ./examples/basic
```

## 配置

配置加载计划在 `v0.1.0` 实现。初版配置预计包含：

- `mode`：`proxy_only`、`pac`、`system`、`hosts`。
- `proxy.listen_addr`：本地代理监听地址。
- `resolver`：DNS / DoH 模式、服务器、缓存、超时与 IP 策略。
- `upstream`：Direct、HTTP Proxy、SOCKS5 Proxy。
- `hosts`：Hosts 模式下的 HTTP / HTTPS 监听设置。
- `cert`：本地 Root CA 相关选项。
- `rules`：默认 Steam 规则与自定义域名。

## 常见示例

当前脚手架检查：

```bash
go test ./...
go vet ./...
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

未来 `v0.1.0` 目标用法：

```bash
steam-accelerator start --mode proxy-only
steam-accelerator status
steam-accelerator stop
```

未来恢复命令目标：

```bash
steam-accelerator restore
```
