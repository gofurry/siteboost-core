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

以前台方式启动 ProxyOnly、PAC、System Proxy 或 Hosts：

```bash
go run ./cmd/steam-accelerator start --mode proxy-only
go run ./cmd/steam-accelerator start --mode pac
go run ./cmd/steam-accelerator start --mode system
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

在另一个终端中：

```bash
go run ./cmd/steam-accelerator status
go run ./cmd/steam-accelerator stop
go run ./cmd/steam-accelerator restore
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

pac:
  listen_addr: "127.0.0.1:26502"
  allow_lan: false

hosts:
  map_ip: "127.0.0.1"
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
  allow_lan: false
  path: "C:\\Windows\\System32\\drivers\\etc\\hosts"
  extra_domains: []

# cert.dir 默认位于用户 config 目录，通常不需要手动配置。

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
  # rollback_path 默认位于用户 cache 目录。
  control_addr: "127.0.0.1:0"
  stop_timeout: "5s"

system_proxy:
  # 仅 macOS 使用。为空表示所有启用的 network services。
  services: []
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
  --pac-listen 127.0.0.1:26502 \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --non-steam reject
```

resolver、upstream 与 macOS system service 选项只通过 YAML 配置。CLI 保持简单的生命周期与本地监听覆盖参数。

## 常见示例

浏览器手动代理设置：

```text
HTTP proxy: 127.0.0.1
Port: 26501
```

默认只允许 Steam 规则域名。非 Steam 流量默认拒绝，除非将 `non_steam_behavior` 设置为 `direct`。`direct` 表示“允许转发”，实际出口由 `upstream.type` 决定。

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

启动 PAC 模式：

```yaml
mode: pac

pac:
  listen_addr: "127.0.0.1:26502"
```

PAC 模式会启动本地代理、启动 `http://127.0.0.1:26502/proxy.pac`，并在 Windows 或 macOS 写入系统 PAC URL。`stop` 会恢复原系统设置。若进程异常退出，执行：

```bash
go run ./cmd/steam-accelerator restore
```

启动 System Proxy 模式：

```yaml
mode: system
```

System 模式会把系统 HTTP 与 HTTPS 代理写入本地代理地址。非 Steam 流量仍遵循 `proxy.non_steam_behavior`，默认是 `reject`。

启动 Windows Hosts 模式：

```yaml
mode: hosts

hosts:
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
```

首次使用前必须显式安装本项目 Root CA：

```bash
go run ./cmd/steam-accelerator cert install
go run ./cmd/steam-accelerator start --mode hosts
```

Hosts 模式会写入 Windows hosts 文件中的项目标记区块，把 exact Steam 域名指向本地 reverse server；`*.domain` 通配符不会写入 hosts。`stop` 或 `restore` 会删除项目标记区块，但不会卸载用户显式安装的 Root CA。卸载证书请执行：

```bash
go run ./cmd/steam-accelerator cert uninstall
```

测试时可以使用高端口，避免占用 80 / 443：

```bash
go run ./cmd/steam-accelerator start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

测试时使用隔离状态文件：

```bash
go run ./cmd/steam-accelerator start --state ./tmp/runtime.json
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
```

macOS / Linux Hosts 与证书安装在 v0.4.0 中明确不支持，会返回 unsupported。
