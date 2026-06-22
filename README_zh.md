# steam-accelerator-core

![License](https://img.shields.io/github/license/gofurry/go-steam-core)
![Release](https://img.shields.io/github/v/release/gofurry/go-steam-core?include_prereleases)
![Go Version](https://img.shields.io/github/go-mod/go-version/gofurry/go-steam-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofurry/go-steam-core)](https://goreportcard.com/report/github.com/gofurry/go-steam-core)

语言：[English](./README.md)

## 项目简介

steam-accelerator-core 是一个用 Go 编写的 Steam 本地网络加速核心，目标是为本地桌面工具、sidecar 服务，以及后续 SteamScope 或 steam-go 集成提供可复用的网络加速原子能力。

当前 v0.6.3 已经包含可运行的本地加速内核，支持 ProxyOnly、PAC、System Proxy、Windows Hosts 反代、Steam 域名匹配、YAML 配置、可配置 DNS 解析与缓存、IPv4 / IPv6 策略、Direct / HTTP / SOCKS5 上游出口、本地 rollback 状态、前台 CLI 生命周期、本地状态文件，以及带 token 的 loopback 控制接口。Hosts + Direct 模式默认会使用内置 DoH 解析真实 Steam IP，避免 hosts 自绕回；并已加入默认 Steam 出站 profile，让 community 域名优先走 `steamcommunity-a.akamaihd.net`，store / checkout / help / login / media 域名优先走 `cdn-a.akamaihd.net`，并覆盖 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net` 这类常见静态资源 / CDN 域名。HTTP Host 保留原始 Steam 域名，TLS SNI 按 profile 使用可达 CDN 域名。启动探测会在 `start` 和 `status` 中暴露 DoH / TCP / TLS / HTTP smoke 状态。Windows 普通 PowerShell 可通过受限 helper 主动请求一次 UAC，完成机器级 Root CA、hosts 写入和恢复动作；HTTP / SOCKS5 upstream 是可选增强，不是默认加速前提。

本项目参考 Watt Toolkit / SteamTools 的网络加速架构思想，包括本地反代、PAC、系统代理、Hosts 模式、证书、DNS 与上游代理等边界；但不包含、不复制、不翻译、不移植 SteamTools 源码。

## 功能特性

当前能力：

- 本地 HTTP Proxy 与 HTTPS CONNECT Proxy。
- Steam 域名规则与安全 host 匹配。
- YAML 配置与安全默认值。
- System DNS、UDP DNS、TCP DNS、DoH、DNS 缓存与 IPv4 / IPv6 策略。
- Direct、HTTP CONNECT upstream 与 SOCKS5 upstream。
- PAC 生成与本地 PAC Server。
- Windows 与 macOS 系统 PAC / 手动代理设置与回滚。
- Windows Hosts 标记区块写入与恢复。
- 本地 Root CA 生成、Windows 机器级 / 当前用户证书库安装与卸载。
- Hosts 模式下的本地 HTTP / HTTPS Reverse Proxy 与动态站点证书。
- Windows Hosts 一键流程：Root CA 自动安装、受限 UAC helper、hosts preflight、rollback 和系统修改状态输出。
- Hosts + Direct 默认 DoH 出站解析、hosts preflight 与 resolver 状态输出。
- Hosts + Direct 默认 Steam 出站 profile，支持 ForwardDestination、TLS SNI、候选 IP 和原始域名 fallback。
- Hosts + Direct 启动探测：DoH 解析、TCP 443、TLS 握手和轻量 HTTPS smoke 状态。
- 出站失败诊断：候选 IP、TCP / TLS 失败阶段和裁剪后的 502 错误摘要。
- 前台 `start`、`status`、`stop`、`restore` CLI 生命周期。
- 本地 runtime 状态文件与带 token 的 loopback 控制 API。

计划能力：

- macOS / Linux Hosts 与证书安装支持。
- DNSIntercept、VPN / TUN 等更深层接管模式。
- 系统修改模式的 restore 生命周期。

当前仓库基础：

- Go module：`github.com/gofurry/go-steam-core`。
- CLI 入口：`cmd/steam-accelerator`。
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

当前可从源码运行 CLI。稳定的 Go library 公共 API 尚未暴露：

```bash
go get github.com/gofurry/go-steam-core
```

## 快速开始

查看版本：

```bash
go run ./cmd/steam-accelerator --version
```

以前台方式启动 ProxyOnly：

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
```

启动 PAC 或 System Proxy：

```bash
go run ./cmd/steam-accelerator start --mode pac
go run ./cmd/steam-accelerator start --mode system
```

Windows Hosts 模式默认会在启动流程内检查并安装本地 Root CA。管理员 PowerShell 走静默直接路径；普通 PowerShell 会在需要写 Root CA 或 Windows hosts 时由受限 helper 请求一次 UAC：

```bash
go run ./cmd/steam-accelerator start --mode hosts
```

在另一个终端中：

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
go run ./cmd/steam-accelerator restore
```

运行 basic 模块示例：

```bash
go run ./examples/basic
```

Resolver、upstream、PAC、System Proxy 和 Hosts 选项通过 YAML 配置。通用默认仍为 `resolver.mode: system` 与 `upstream.type: direct`；但 `start --mode hosts` 在 Direct 出口下会自动使用内置 DoH 和默认 Steam 出站 profile，`status` 会显示 `resolver: doh`。

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

Hosts 模式当前为 Windows-first。测试高端口可使用 `--hosts-http 127.0.0.1:28080 --hosts-https 127.0.0.1:28443`。

## 项目结构

```text
.
├── cmd/steam-accelerator/     # CLI 入口
├── docs/                      # 英文维护文档
├── docs/zh/                   # 中文维护文档
├── examples/basic/            # 最小可运行示例
├── internal/                  # 私有运行时实现包
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
5. `v0.5.0`：一键 Hosts + DoH 默认闭环。
6. `v0.5.1`：出站失败诊断补丁。
7. `v0.6.0`：真实 Steam 出站 profile、访问验收与规则完善。
8. `v0.6.1`：Windows 证书写入与一键流程封装。
9. `v0.6.2`：Windows 机器级默认证书写入。
10. `v0.6.3`：Windows 受限提权 helper 一键启动。
11. `v0.7.0`：通用加速核心重构与命名迁移准备。
12. `v1.0.0`：稳定 API 与集成发布。

详见 [ROADMAP.md](./ROADMAP.md)。

## 参与贡献

项目仍处于早期阶段。运行时实现会在集成 API 稳定前保持 internal。请保持改动小、可测试，并与路线图对齐。不要复制 SteamTools 实现代码；应使用独立 Go 实现，并记录外部依赖选择理由。

## 开源协议

GPL-3.0。详见 [LICENSE](./LICENSE)。
