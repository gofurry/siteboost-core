# 冒烟测试

## 快速验证步骤

在仓库根目录运行：

```bash
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/hosts ./internal/certstore ./internal/reverse ./internal/pac ./internal/systemproxy ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine ./internal/runtime
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

这些检查会默认修改 Windows `LocalMachine\Root` 证书库和 Windows hosts 文件。v0.6.3 起，普通 PowerShell 运行 hosts 模式时会通过受限 helper 请求一次 Windows UAC；管理员 PowerShell 会走静默直接路径。测试完成后请执行 `stop` 与 `cert uninstall`。

可选预安装本项目 Root CA。`cert.auto_install` 为 true 时，`start --mode hosts` 可以在启动流程内自动安装；普通 PowerShell 下该命令也会通过 helper 请求 UAC：

```bash
go run ./cmd/steam-accelerator cert install
```

使用高端口启动，避免占用 80 / 443：

```bash
go run ./cmd/steam-accelerator start --mode hosts \
  --hosts-http 127.0.0.1:28080 \
  --hosts-https 127.0.0.1:28443 \
  --state ./tmp/runtime.json
```

另一个终端检查状态：

```bash
go run ./cmd/steam-accelerator status --state ./tmp/runtime.json
```

默认 Hosts + Direct 闭环的状态中应出现 `resolver: doh`、`resolver_servers:`、`rule_set: steam-web@2026.06.22`、`upstream_profiles: 4` 和 `startup_probes:`。这表示反代出站解析没有继续使用 system resolver，从而避免 hosts 自绕回。v0.6.0 起，默认出站 profile 还会让 `steamcommunity.com` 优先连接 `steamcommunity-a.akamaihd.net`，让 `store.steampowered.com` / `checkout.steampowered.com` / `help.steampowered.com` / `login.steampowered.com` / `media.steampowered.com` 优先连接 `cdn-a.akamaihd.net`，并覆盖 `community.steamstatic.com` 与 `steamcdn-a.akamaihd.net`，同时保留原始 HTTP Host。

`system_change:` 行应显示 Root CA 检查/安装、hosts preflight、反代监听和 hosts 写入结果。普通 PowerShell 通过 helper 成功时，Root CA 或 hosts 行的 detail 中应包含 `helper=elevated`。`startup_probes: ok=6 failed=0` 是理想结果。如果有失败，先查看 `startup_probe_failed` 行再打开浏览器；`stage=resolve`、`stage=tcp`、`stage=tls`、`stage=http` 可以缩小失败层级。默认探测目标、exact hosts 清单、wildcard 缺口和手动记录表维护在 [Steam 兼容性清单](steam-compatibility.md)。

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
go run ./cmd/steam-accelerator stop --state ./tmp/runtime.json
go run ./cmd/steam-accelerator cert uninstall
```

如果从普通 PowerShell 测试，`stop` / `restore` 恢复 hosts 和 `cert uninstall` 卸载机器级 Root CA 时也可能再次请求 UAC。取消 UAC 时命令应返回可理解错误；项目不应写入新的 hosts 标记区块或留下新的半成品 rollback。

## 期望输出

版本命令应输出项目名、版本号和模块路径。

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
- Windows hosts preflight 或写入失败；普通 PowerShell 应触发 UAC helper，若使用了自定义 `hosts.path` / `runtime.rollback_path` / `cert.dir`，请改用管理员终端或默认路径。
- `upstream request failed` 后跟 `direct upstream resolve ... failed`：DoH / DNS 解析失败或网络拦截。
- `upstream request failed` 后跟 `resolve steamcommunity-a.akamaihd.net:443 failed` 或 `resolve cdn-a.akamaihd.net:443 failed`：默认 Steam profile 的 ForwardDestination 解析失败。
- `upstream request failed` 后跟 `tcp ... failed`：候选真实 IP 或 ForwardDestination IP 无法直连。
- `upstream request failed` 后跟 `tls ... failed`：IP 可连，但 TLS / SNI / 证书链路失败。
- restore 失败后 rollback 状态仍会保留；修复平台错误后再次执行 `restore`。
- 状态文件指向旧进程；`status` 或 `stop` 应自动清理 stale 状态。
