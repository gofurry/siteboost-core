# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

语言：[English](./README.md)

## 项目简介

steam-accelerator-core 是一个用 Go 编写的 Steam 本地网络加速核心，目标是为本地桌面工具、sidecar 服务，以及后续 SteamScope 或 steam-go 集成提供可复用的网络加速原子能力。

当前仓库仍处于脚手架阶段。真实代理、PAC、Hosts、证书、DNS、反代等运行时能力已进入路线图，但尚未实现。

本项目参考 Watt Toolkit / SteamTools 的网络加速架构思想，包括本地反代、PAC、系统代理、Hosts 模式、证书、DNS 与上游代理等边界；但不包含、不复制、不翻译、不移植 SteamTools 源码。

## 功能特性

计划能力：

- 本地 HTTP Proxy 与 HTTPS CONNECT Proxy。
- Steam 域名规则与安全 host 匹配。
- PAC 生成与本地 PAC Server。
- System Proxy 设置与回滚。
- Hosts 修改、备份与事务化恢复。
- 本地 Root CA 与动态站点证书。
- Hosts 模式下的 HTTPS Reverse Proxy。
- DNS、DoH、缓存与 IPv4 / IPv6 策略。
- Direct、HTTP Proxy、SOCKS5 上游出口。
- Engine 启动、停止、恢复与状态查询。

当前已完成的仓库基础：

- Go module：`github.com/gofurry/go-steam-core`。
- 最小 CLI 入口：`cmd/steam-accelerator`。
- 可运行的 basic 示例。
- 中英文 README 与文档目录。
- GitHub Actions：`gofmt`、`go vet`、`go test`。
- 中文主路线图。

## 安装方式

本地开发：

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

库安装会在 v0.1.0 API 出现后变得有意义：

```bash
go get github.com/gofurry/go-steam-core
```

## 快速开始

运行脚手架 CLI：

```bash
go run ./cmd/steam-accelerator --version
```

运行 basic 示例：

```bash
go run ./examples/basic
```

## 文档

- [中文路线图](./ROADMAP.md)
- [英文路线图](./docs/roadmap.md)
- [使用说明](./docs/zh/usage.md)
- [冒烟测试](./docs/zh/smoke-test.md)
- [热修复流程](./docs/zh/hotfix.md)
- [待办事项](./docs/zh/todo.md)
- [安全策略](./SECURITY.md)
- [SteamTools 参考边界](./docs/zh/steamtools-reference.md)

## 示例

示例位于 `examples/`。

- `examples/basic`：验证模块可以被导入并运行。

ProxyOnly、PAC、System Proxy、Hosts 模式示例会随对应里程碑补齐。

## 项目结构

```text
.
├── cmd/steam-accelerator/     # CLI 入口，当前为脚手架
├── docs/                      # 英文维护文档
├── docs/zh/                   # 中文维护文档
├── examples/basic/            # 最小可运行示例
├── internal/                  # 规划中的私有实现包
├── ROADMAP.md                 # 中文主路线图
├── README.md
├── README_zh.md
└── go.mod
```

## 开发说明

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
```

GitHub Actions 会在 push 和 pull request 时运行同一组格式、vet 与测试检查。

## 路线图

实现顺序坚持“先核心基础，后高风险接管”：

1. `v0.1.0`：ProxyOnly 与 CONNECT 核心。
2. `v0.2.0`：Resolver、DoH 与上游出口。
3. `v0.3.0`：PAC 与 System Proxy。
4. `v0.4.0`：Hosts 与 HTTPS 反代。
5. `v0.5.0`：稳定性、安全恢复与跨平台打磨。
6. `v1.0.0`：稳定 API 与集成发布。

详见 [ROADMAP.md](./ROADMAP.md)。

## 参与贡献

项目仍处于早期阶段。请保持改动小、可测试，并与路线图对齐。不要复制 SteamTools 实现代码；应使用独立 Go 实现，并记录外部依赖选择理由。

## 开源协议

GPL-3.0。详见 [LICENSE](./LICENSE)。
