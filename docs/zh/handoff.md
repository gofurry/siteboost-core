# Handoff

> 更新时间：2026-06-17
> 当前分支：`master`
> 当前本地进度：`455be7a v0.2.0`
> 远端状态：本地分支领先 `origin/master`

## 当前状态

`v0.2.0 - Resolver / DoH / Upstream` 已完成并已提交到本地。

当前内核能力：

- ProxyOnly 前台运行模式。
- 默认监听 `127.0.0.1:26501`。
- Steam 域名规则匹配。
- HTTP Proxy 与 HTTPS CONNECT。
- YAML 配置。
- `start` / `status` / `stop` CLI 生命周期。
- 本地状态文件与 token 控制接口。
- `resolver.mode`: `system` / `udp` / `tcp` / `doh`。
- DNS 缓存、fallback、timeout、IPv4 / IPv6 策略。
- `upstream.type`: `direct` / `http` / `socks5`。
- HTTP upstream 使用 CONNECT。
- SOCKS5 upstream 使用 remote DNS。

运行时仍保持在 `internal/`，尚未暴露稳定公共 Go API。

## 已验证内容

自动化验证已通过：

```powershell
go mod tidy
gofmt -w .
go vet ./...
go test ./...
go test -race ./internal/resolver ./internal/upstream ./internal/proxy ./internal/engine
go run ./cmd/steam-accelerator --version
go run ./examples/basic
```

版本输出：

```text
steam-accelerator-core v0.2.0-dev (github.com/gofurry/go-steam-core)
```

手动网络验证：

- 测试 2：`system + direct`，美国网络，`store.steampowered.com` 通过，`example.com` 默认 403。
- 测试 3：`doh + direct`，美国网络，`store.steampowered.com` 通过，`example.com` 默认 403。
- 测试 4：本机可切中国网络与美国代理，本地代理端口 `127.0.0.1:7897`；直连 `curl` 与代理出口结果不同，`7897` 与 `26501` 都已验证可通。

## 常用测试命令

默认 direct：

```powershell
go run ./cmd/steam-accelerator start --config .\tmp\v020-system-direct.yaml --state .\tmp\runtime.json
go run ./cmd/steam-accelerator status --state .\tmp\runtime.json
curl.exe -x http://127.0.0.1:26501 -I https://store.steampowered.com
curl.exe -x http://127.0.0.1:26501 -i http://example.com
go run ./cmd/steam-accelerator stop --state .\tmp\runtime.json
```

HTTP upstream 示例：

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  non_steam_behavior: "reject"

resolver:
  mode: "system"
  prefer_ipv4: true

upstream:
  type: "http"
  address: "127.0.0.1:7897"
```

SOCKS5 upstream 示例：

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  non_steam_behavior: "reject"

resolver:
  mode: "system"
  prefer_ipv4: true

upstream:
  type: "socks5"
  address: "127.0.0.1:7897"
```

如果测试 `prefer_ipv6: true`，需要显式关闭默认 IPv4 偏好：

```yaml
resolver:
  mode: "system"
  prefer_ipv4: false
  prefer_ipv6: true
```

## 下一步建议

下一阶段进入 `v0.3.0 - PAC 与 System Proxy`。

建议新 session 起手动作：

1. 阅读 `ROADMAP.md`、`docs/zh/roadmap.md`、`docs/zh/usage.md`、`docs/zh/smoke-test.md`。
2. 查看最近提交：`git log --oneline -5`。
3. 确认工作区：`git status --short --branch`。
4. 开始实现 PAC 生成器和本地 PAC Server。
5. 再接入 `start --mode pac`，最后做 Windows / macOS System Proxy 写入与 restore。

v0.3.0 初始边界建议：

- PAC 规则必须来自 `rules` 模块。
- PAC Server 默认只监听 loopback。
- System Proxy 修改必须有 rollback 状态。
- 不碰 hosts、本地 CA、HTTPS reverse proxy。
- 不引入公共 Go API，继续保持 runtime 在 `internal/`。

## 注意事项

- 不复制、不翻译、不移植 SteamTools 源码。
- 默认只监听 `127.0.0.1`。
- 默认只允许 Steam 规则域名。
- 日志不得记录 Cookie、Authorization、完整 query、upstream password。
- `non_steam_behavior: direct` 在 v0.2.0 表示允许转发，实际出口由 `upstream.type` 决定。
- 临时测试文件建议放在 `tmp/`，不要提交 runtime state 或日志。
