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
  # Hosts + Direct 模式默认启用内置 Steam 出站 profile。
  enable_default_steam_profiles: true
  profiles: []

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

说明：`resolver.mode: system` 是通用默认值；当 `mode: hosts` 且 `upstream.type: direct` 时，运行时会自动切到内置 DoH 解析真实 Steam IP，避免 hosts 写入后把反代出站连接解析回 `127.0.0.1`，并启用默认 Steam 出站 profile。外部 HTTP / SOCKS5 upstream 仍然只是可选增强，不是默认加速前提。

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
  # servers 为空时使用内置 DoH 默认列表；也可以显式覆盖。
  servers: []
  prefer_ipv4: true
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct"
  enable_default_steam_profiles: true
```

自定义 Steam 出站 profile：

```yaml
upstream:
  type: "direct"
  enable_default_steam_profiles: true
  profiles:
    - match_domains:
        - "steamcommunity.com"
        - "*.steamcommunity.com"
      forward_host: "steamcommunity-a.akamaihd.net"
      tls_server_name: "steamcommunity-a.akamaihd.net"
    - match_domains:
        - "store.steampowered.com"
        - "checkout.steampowered.com"
        - "help.steampowered.com"
        - "login.steampowered.com"
        - "media.steampowered.com"
      forward_host: "cdn-a.akamaihd.net"
      tls_server_name: "cdn-a.akamaihd.net"
    - match_domains:
        - "community.steamstatic.com"
      forward_host: "community.steamstatic.com"
      tls_server_name: "community.steamstatic.com"
    - match_domains:
        - "steamcdn-a.akamaihd.net"
      forward_host: "steamcdn-a.akamaihd.net"
      tls_server_name: "steamcdn-a.akamaihd.net"
```

profile 的含义接近 Steam++ 的经验做法：`match_domains` 匹配原始 Steam 域名，`forward_host` 用于解析和连接更可达的 CDN 目标，`tls_server_name` 用于出站 TLS SNI，反代发给上游的 HTTP Host 仍保持原始 Steam 域名。`candidate_ips` 可指定固定候选 IP；`ignore_tls_name_mismatch: true` 只应在明确知道证书链可信但名称不匹配时使用。

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

`cert install` 会先按证书 thumbprint 检查当前用户 Root store；如果本项目 Root CA 已安装，会直接返回，不会重复执行安装动作。

Hosts 模式会写入 Windows hosts 文件中的项目标记区块，把 exact Steam 域名指向本地 reverse server；`*.domain` 通配符不会写入 hosts。默认 Hosts + Direct 闭环会使用内置 DoH 做出站真实解析，不需要配置外部上游代理。启动时会先检查 Root CA、hosts 可读写、rollback 目录可写和反代监听；`status` 会显示运行时 `resolver`、`resolver_servers`、`upstream_profiles` 和 `startup_probes`。`stop` 或 `restore` 会删除项目标记区块，但不会卸载用户显式安装的 Root CA。卸载证书请执行：

```bash
go run ./cmd/steam-accelerator cert uninstall
```

如果浏览器或 Steam 内置浏览器仍显示 `upstream request failed`，响应体和日志会带出站诊断摘要，例如 DoH 解析失败、某个候选 IP 的 TCP 连接失败，或 TLS 握手失败。下一步应根据该错误判断是 DNS/DoH、ForwardDestination 可达性、证书/SNI，还是规则/profile 覆盖问题。

在 Hosts + Direct 模式下，`start` 和 `status` 会包含非致命启动探测摘要。`startup_probes: ok=6 failed=0` 表示默认 Steam 探测目标已经通过当前 outbound profile 链路完成 DoH 解析、TCP 443、TLS 和轻量 HTTPS `HEAD /` 检查。失败项会以 `startup_probe_failed` 输出 `host`、`target`、`stage` 和裁剪后的错误。exact hosts 清单、wildcard 缺口、默认探测目标和手动 smoke 表维护在 [Steam 兼容性清单](steam-compatibility.md)。

Windows 自带 `curl.exe` 默认使用 Schannel 检查证书吊销状态。由于本项目动态签发的本地站点证书没有公网 OCSP / CRL，命令行验证时如果看到 `CRYPT_E_NO_REVOCATION_CHECK`，可使用 `--ssl-no-revoke` 跳过吊销检查；这仍会保留证书链和域名校验，比 `-k/--insecure` 更适合验证本地加速链路：

```bash
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcommunity.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://store.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://community.steamstatic.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://media.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcdn-a.akamaihd.net/
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

macOS / Linux Hosts 与证书安装在 v0.6.0 中仍明确不支持，会返回 unsupported。
