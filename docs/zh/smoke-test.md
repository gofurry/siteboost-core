# 冒烟测试

## 快速验证步骤

在仓库根目录运行：

```bash
git diff --check
go test ./...
go vet ./...
go test -race ./internal/hosts ./internal/privilege ./internal/engine ./cmd/steam-accelerator
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
go run ./cmd/steam-accelerator --version
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

DoH 与 HTTP/SOCKS5 upstream 行为由 `go test ./internal/resolver ./internal/upstream ./internal/proxy` 中的本地 fake server 覆盖。手动检查时，可将 `resolver.mode` 配为 `doh`；`servers` 为空会使用内置 DoH 默认列表，也可以显式覆盖。HTTP / SOCKS5 upstream 只在需要外部代理增强时配置。

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

这些检查会默认修改 Windows `LocalMachine\Root` 证书库和 Windows hosts 文件。v0.6.4 起，推荐先构建固定路径的本地二进制，并用管理员 PowerShell 执行一次 `apphost install`；之后普通 PowerShell 必须使用同一个二进制路径运行 hosts 模式，并通过 AppHost named pipe 请求受限系统修改；管理员 PowerShell 会走静默直接路径。测试完成后请执行 `stop` 与 `cert uninstall`。

AppHost 初始化：

```bash
go build -o ./bin/steam-accelerator.exe ./cmd/steam-accelerator
./bin/steam-accelerator.exe apphost install
./bin/steam-accelerator.exe apphost status
```

### v0.6.4 Windows AppHost 主流程记录

2026-06-23，本机 Windows + 中国网络环境下已完成一次 AppHost + Hosts 模式主流程记录。该记录证明 AppHost 可以安装为自动延迟启动服务，健康检查可用，当前 Hosts 反代链路可让 Steam 目标域名解析到本地并连通 443，`stop` / `restore` 后 AppHost 仍作为常驻提权底座保持健康，卸载后 `apphost status` 会返回服务不存在错误。

管理员 PowerShell：

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost install
apphost installed and started

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
apphost: running start_type=automatic delayed_auto_start=true pid=98084 health=ok
```

普通 PowerShell 中，当前已有加速实例运行时，重复启动会返回 `already running`。这不是失败；继续检查 hosts 接管和本地 443 连通性即可：

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe start --mode hosts
error: steam-accelerator is already running

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection steamcommunity.com -Port 443
ComputerName     : steamcommunity.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection store.steampowered.com -Port 443
ComputerName     : store.steampowered.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True

PS D:\WorkSpace\Git\siteboost-core> Test-NetConnection help.steampowered.com -Port 443
ComputerName     : help.steampowered.com
RemoteAddress    : 127.0.0.1
RemotePort       : 443
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : 127.0.0.1
TcpTestSucceeded : True
```

停止、恢复与卸载：

```powershell
PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe stop
stopped

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe restore
restored

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
apphost: running start_type=automatic delayed_auto_start=true pid=98084 health=ok

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost uninstall
apphost uninstalled

PS D:\WorkSpace\Git\siteboost-core> .\bin\steam-accelerator.exe apphost status
error: open apphost service: The specified service does not exist as an installed service.
```

该记录仍不替代一次完整的真实重启 smoke。发布前建议继续补充：

```powershell
.\bin\steam-accelerator.exe apphost install
# reboot Windows
.\bin\steam-accelerator.exe apphost status
.\bin\steam-accelerator.exe start --mode hosts
```

可选预安装本项目 Root CA。`cert.auto_install` 为 true 时，`start --mode hosts` 可以在启动流程内自动安装；普通 PowerShell 下该命令也会通过 AppHost 请求受限系统修改：

```bash
./bin/steam-accelerator.exe cert install
```

使用高端口启动，避免占用 80 / 443：

```bash
./bin/steam-accelerator.exe start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

另一个终端检查状态：

```bash
./bin/steam-accelerator.exe status --state ./tmp/runtime.json
```

默认 Hosts + Direct 闭环的状态中应出现 `provider: id=steam status=stable rule_set=steam-web@2026.06.22 profiles=4 probes=6`、`resolver: doh`、`resolver_servers:`、`rule_set: steam-web@2026.06.22`、`upstream_profiles: 4` 和 `startup_probes:`。默认单 Steam provider 下继续保留独立 `rule_set:` 行，方便沿用既有 smoke 阅读习惯。这表示反代出站解析没有继续使用 system resolver，从而避免 hosts 自绕回。v0.6.0 起，默认 Steam outbound profile 还会让 `steamcommunity.com` 优先连接 `steamcommunity-a.akamaihd.net`，让 `store.steampowered.com` / `checkout.steampowered.com` / `help.steampowered.com` / `login.steampowered.com` / `media.steampowered.com` 优先连接 `cdn-a.akamaihd.net`，并覆盖 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net`，同时保留原始 HTTP Host。

如需验证 v0.7 provider skeleton，可创建临时配置显式启用 GitHub：

```yaml
providers:
  enabled:
    - steam
    - github
```

用该配置启动后检查 `status`。输出应同时包含 `provider: id=steam status=stable ...` 和 `provider: id=github status=experimental rule_set=github-web@2026.06.23 probes=3`。GitHub 只是用于验证架构扩展的 skeleton provider；该 smoke 不应要求 GitHub 真实可达，也不表示已经具备 GitHub 真实加速。

`system_change:` 行应显示 Root CA 检查/安装、hosts preflight、反代监听和 hosts 写入结果。普通 PowerShell 通过 AppHost 成功时，Root CA 或 hosts 行的 detail 中应包含 `helper=elevated`。`startup_probes: ok=6 failed=0` 是理想结果。如果有失败，先查看 `startup_probe_failed` 行再打开浏览器；`stage=resolve`、`stage=tcp`、`stage=tls`、`stage=http` 可以缩小失败层级。默认探测目标、exact hosts 清单、wildcard 缺口和手动记录表维护在 [Steam 兼容性清单](steam-compatibility.md)。

如果访问页面返回 `upstream request failed`，响应体不应只有这一句，还应包含类似 `direct upstream resolve ... failed`、`resolve steamcommunity-a.akamaihd.net:443 failed`、`tcp 1.2.3.4:443 failed` 或 `tls 1.2.3.4:443 failed` 的摘要。该摘要用来判断失败发生在 DoH、ForwardDestination 解析、TCP 直连还是 TLS 握手阶段。

真实 hosts 模式默认写入 80 / 443。高端口主要用于验证 reverse server 生命周期；如果要验证浏览器访问真实 Steam 域名，需要使用默认 80 / 443 并确认本机端口未被占用。

Windows 自带 `curl.exe` 使用 Schannel，默认会检查证书吊销状态。本项目的 Hosts 反代会动态签发本地站点证书，这类本地证书没有公网 OCSP / CRL；如果直接 `curl.exe -I https://steamcommunity.com/` 出现 `CRYPT_E_NO_REVOCATION_CHECK`，说明命令行客户端无法完成吊销检查，不代表出站加速失败。命令行内容验证建议使用：

```bash
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcommunity.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://store.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://community.steamstatic.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://media.steampowered.com/
curl.exe --ssl-no-revoke -I --max-time 30 https://steamcdn-a.akamaihd.net/
```

停止并卸载证书：

```bash
./bin/steam-accelerator.exe stop --state ./tmp/runtime.json
./bin/steam-accelerator.exe cert uninstall
```

如果从普通 PowerShell 测试，`stop` / `restore` 恢复 hosts 和 `cert uninstall` 卸载机器级 Root CA 时会通过 AppHost 处理。AppHost 未安装或未运行时命令应返回可理解错误；项目不应写入新的 hosts 标记区块或留下新的半成品 rollback。

## DNSIntercept Manual Smoke

先使用高端口测试。该 smoke 只验证 DNS 决策、转发、缓存、状态和关闭恢复，不会修改系统 DNS、hosts 或证书信任：

```bash
./bin/steam-accelerator.exe start --mode dns --dns-listen 127.0.0.1:15353 --hosts-http 127.0.0.1:28080 --hosts-https 127.0.0.1:28443 --state ./tmp/dns-runtime.json
```

另开终端：

```powershell
dig @127.0.0.1 -p 15353 steamcommunity.com A
dig @127.0.0.1 -p 15353 example.com A
./bin/steam-accelerator.exe status --state ./tmp/dns-runtime.json
./bin/steam-accelerator.exe stop --state ./tmp/dns-runtime.json
```

期望行为：

- `steamcommunity.com` 的 `A` 记录解析为 `127.0.0.1`。如果没有安装 `dig`，可使用任何支持自定义端口的 DNS 客户端。
- 目标域名的 `HTTPS` / `SVCB` 记录默认返回 NODATA；如需手动 smoke 这条分支，请使用支持这些 RR 类型的 DNS 客户端。
- `example.com` 会转发到显式 resolver 或避免自绕回的 DoH 默认上游。
- `status` 包含 `dns_intercept: strategy=manual ... system_dns=false ... target=... forwarded=... cache_hits=... blocked=... errors=...`。
- DNS manual 模式下不应出现新的 `system_change:` 行。

高端口 DNS smoke 不能证明浏览器接管，因为 DNS 记录不能携带端口。浏览器级 DNSIntercept 测试需要反代监听 80 / 443，并让测试客户端显式使用本地 DNS server；如需自动接管系统 DNS，请使用下面的 system smoke。

## DNSIntercept System Smoke

该 smoke 会真实修改指定 Windows 网卡的 DNS server。先确认 AppHost 已安装并健康，再记录当前 DNS：

```powershell
.\bin\steam-accelerator.exe apphost status
Get-DnsClientServerAddress -AddressFamily IPv4
```

创建临时配置，例如 `tmp\dns-system.yaml`，把 `interfaces` 改成你的网卡名称、index 或 GUID：

```yaml
mode: dns

dns_intercept:
  strategy: "system"
  listen_addr: "127.0.0.1:53"
  interfaces:
    - "Wi-Fi"
  map_ipv4: "127.0.0.1"
  block_https_records: true

hosts:
  http_listen_addr: "127.0.0.1:80"
  https_listen_addr: "127.0.0.1:443"
```

启动并验证：

```powershell
.\bin\steam-accelerator.exe start --config .\tmp\dns-system.yaml --state .\tmp\dns-system-runtime.json
.\bin\steam-accelerator.exe status --state .\tmp\dns-system-runtime.json
Resolve-DnsName steamcommunity.com -Type A
Resolve-DnsName example.com -Type A
```

期望行为：

- `status` 包含 `dns_intercept: strategy=system ... system_dns=true ...`。
- `status` 包含 `system_change: component=system_dns action=preflight status=ok ...` 和 `action=apply`。
- `Get-DnsClientServerAddress -AddressFamily IPv4` 显示指定网卡 DNS 指向 `127.0.0.1`。
- `Resolve-DnsName steamcommunity.com -Type A` 返回 `127.0.0.1`。
- `Resolve-DnsName example.com -Type A` 仍能通过上游 resolver 解析。

停止和恢复：

```powershell
.\bin\steam-accelerator.exe stop --state .\tmp\dns-system-runtime.json
.\bin\steam-accelerator.exe restore --config .\tmp\dns-system.yaml
Get-DnsClientServerAddress -AddressFamily IPv4
```

期望指定网卡 DNS 回到启动前的 DHCP 或静态 DNS。若 `stop` 过程中异常退出，rollback 会保留；修复问题后再次执行 `restore --config .\tmp\dns-system.yaml`。

## Page Enhance Smoke

自动化测试覆盖 header transform、HTML 注入、replace、本地 asset、body size skip、错误策略、Go 自定义 transformer、reverse 接入、engine status 和 CLI 输出：

```bash
go test ./internal/pageenhance ./internal/reverse ./internal/engine ./cmd/steam-accelerator
```

手动 smoke 优先用 DNSIntercept manual 高端口和本地 asset，不修改系统 DNS、hosts、证书、浏览器设置或开发者环境：

```powershell
Set-Content -Path .\tmp\local.js -Value 'console.log("siteboost page enhance");'
@'
mode: dns

dns_intercept:
  strategy: "manual"
  listen_addr: "127.0.0.1:15353"

hosts:
  http_listen_addr: "127.0.0.1:28080"
  https_listen_addr: "127.0.0.1:28443"

page_enhance:
  enabled: true
  on_error: "pass_through"
  assets:
    - path: "/siteboost/local.js"
      file: ".\\tmp\\local.js"
      content_type: "application/javascript"
  transforms:
    - name: "steam-header"
      match:
        providers:
          - "steam"
        hosts:
          - "store.steampowered.com"
      headers:
        set:
          X-SiteBoost-Enhanced: "true"
'@ | Set-Content -Path .\tmp\page-enhance.yaml

.\bin\steam-accelerator.exe start --config .\tmp\page-enhance.yaml --state .\tmp\page-enhance-runtime.json
```

另开终端：

```powershell
curl.exe -H "Host: store.steampowered.com" http://127.0.0.1:28080/siteboost/local.js
.\bin\steam-accelerator.exe status --state .\tmp\page-enhance-runtime.json
.\bin\steam-accelerator.exe stop --state .\tmp\page-enhance-runtime.json
```

期望行为：

- asset 请求返回 `console.log("siteboost page enhance");`。
- `status` 包含 `page_enhance: enabled=true on_error=pass_through transforms=1 assets=1 ...`。
- 因为使用的是 manual DNSIntercept 高端口，`stop` 后不会留下系统 DNS、hosts、证书、浏览器或开发者环境变化。
- 可选真实上游检查：请求带 `Host: store.steampowered.com` 的 `http://127.0.0.1:28080/`，验证响应头 `X-SiteBoost-Enhanced: true`；这依赖 live upstream 行为，不作为默认自动化验证要求。

## 期望输出

版本命令应输出项目名、`v0.7.3-dev` 和模块路径。

basic 示例应输出项目名和模块路径。

前台 `start` 运行时，`status` 应显示 `running: true`。`stop` 应让前台进程退出，并输出 `stopped` 或 `stop requested`。

## 常见失败情况

- 未安装 Go，或 Go 版本低于 `go.mod` 声明。
- 新增 Go 文件未经过 `gofmt`。
- 新增依赖后未运行 `go mod tidy`。
- `127.0.0.1:26501` 端口已被占用。
- UDP / TCP resolver 未配置 `resolver.servers`。
- HTTP 或 SOCKS5 upstream 未配置 `upstream.address`。
- PAC 模式下 `127.0.0.1:26502` 端口已被占用。
- 当前系统不是 Windows 或 macOS，不能使用 PAC/System Proxy 模式写系统代理。
- Hosts 模式下 80 / 443 端口已被占用。
- Hosts 模式未先执行 `cert install`，且 `cert.auto_install` 被设为 false。
- Windows hosts preflight 或写入失败；普通 PowerShell 应通过 AppHost named pipe 完成，若 AppHost 未安装请先用管理员 PowerShell 执行 `apphost install`。若使用了自定义 `hosts.path` / `runtime.rollback_path` / `cert.dir`，请改用管理员终端或默认路径。
- `upstream request failed` 后跟 `direct upstream resolve ... failed`：DoH / DNS 解析失败或网络拦截。
- `upstream request failed` 后跟 `resolve steamcommunity-a.akamaihd.net:443 failed` 或 `resolve cdn-a.akamaihd.net:443 failed`：默认 Steam profile 的 ForwardDestination 解析失败。
- `upstream request failed` 后跟 `tcp ... failed`：候选真实 IP 或 ForwardDestination IP 无法直连。
- `upstream request failed` 后跟 `tls ... failed`：IP 可连，但 TLS / SNI / 证书链路失败。
- restore 失败后 rollback 状态仍会保留；修复平台错误后再次执行 `restore`。
- 状态文件指向旧进程；`status` 或 `stop` 应自动清理 stale 状态。
