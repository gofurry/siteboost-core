# 使用说明

## 安装方式

本地开发：

```bash
git clone https://github.com/gofurry/go-steam-core.git
cd go-steam-core
go mod download
```

## 基本用法

查看版本：

```bash
go run ./cmd/steam-accelerator --version
```

以前台方式启动 ProxyOnly：

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
```

在另一个终端中：

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
```

## 配置

ProxyOnly 可以直接使用默认配置，也可以使用 YAML：

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  non_steam_behavior: "reject" # reject | direct
  allow_lan: false
  read_header_timeout: "10s"
  idle_timeout: "2m"
  dial_timeout: "30s"
  shutdown_timeout: "5s"

rules:
  enable_default_steam_rules: true
  custom_domains: []

resolver:
  mode: "system" # system | udp | tcp | doh
  servers: []
  prefer_ipv4: true
  prefer_ipv6: false
  disable_ipv6: false
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct" # direct | http | socks5
  address: ""
  username: ""
  password: ""

runtime:
  # state_path 默认位于用户 cache 目录。
  control_addr: "127.0.0.1:0"
  stop_timeout: "5s"
```

配置优先级：

```text
内置默认值 < YAML 配置 < CLI flags
```

CLI 覆盖示例：

```bash
go run ./cmd/steam-accelerator start \
  --config config.yaml \
  --listen 127.0.0.1:26501 \
  --non-steam reject
```

v0.2.0 中 resolver 与 upstream 选项只通过 YAML 配置。CLI 保持简单的生命周期与 ProxyOnly 覆盖参数。

## 常见示例

浏览器手动代理设置：

```text
HTTP proxy: 127.0.0.1
Port: 26501
```

默认只允许 Steam 规则域名。非 Steam 流量默认拒绝，除非将 `non_steam_behavior` 设置为 `direct`。在 v0.2.0 中，`direct` 表示“允许转发”，实际出口由 `upstream.type` 决定。

使用 DoH 与 Direct 出口：

```yaml
resolver:
  mode: "doh"
  servers:
    - "https://dns.example/dns-query"
  prefer_ipv4: true
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct"
```

使用 HTTP upstream：

```yaml
upstream:
  type: "http"
  address: "127.0.0.1:8080"
  username: "optional-user"
  password: "optional-password"
```

使用 SOCKS5 upstream：

```yaml
upstream:
  type: "socks5"
  address: "127.0.0.1:1080"
  username: "optional-user"
  password: "optional-password"
```

UDP 与 TCP DNS 模式必须显式配置 `servers`。DNS server 地址省略端口时默认使用 `53`。

测试时使用隔离状态文件：

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

PAC、System Proxy、Hosts、证书管理和 restore 命令将在后续里程碑实现。
