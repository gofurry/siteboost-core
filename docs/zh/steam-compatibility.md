# Steam 兼容性清单

本文记录默认 Steam Web 加速覆盖情况。这是一份 clean-room 兼容性清单；Steam++ / Watt Toolkit 只作为公开行为层参考，用来验证域名/profile 选择，不复制实现代码。

## 启动探测

Hosts + Direct 模式会通过反代实际使用的同一条 DoH 和 outbound profile 链路执行非致命启动探测。探测覆盖解析、TCP 连接、TLS 握手和轻量 HTTPS `HEAD /` 请求。

`start` 和 `status` 会输出：

```text
startup_probes: ok=7 failed=0
startup_probe_failed: host=store.steampowered.com target=cdn-a.akamaihd.net stage=tcp error=tcp ...
```

默认探测目标：

| Host | 预期 profile 目标 | 用途 |
|---|---|---|
| `steamcommunity.com` | `steamcommunity-a.akamaihd.net` | 社区入口 |
| `store.steampowered.com` | `cdn-a.akamaihd.net` | 商店入口 |
| `api.steampowered.com` | `steamstore.rmbgame.net` | Steam 官方 Web API |
| `help.steampowered.com` | `cdn-a.akamaihd.net` | 帮助入口 |
| `media.steampowered.com` | `cdn-a.akamaihd.net` | 媒体 / 静态资源域名 |
| `community.steamstatic.com` | `community.steamstatic.com` | 社区静态资源 |
| `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | CDN 资源域名 |

探测失败不会阻止启动。它的作用是帮助判断问题发生在 DNS / DoH、TCP 可达性、TLS / SNI，还是规则 / profile 覆盖缺口。

## Hosts Exact 清单

Hosts 模式只能写入 exact 域名。默认 exact 写入清单是：

```text
api.steampowered.com
akamai.steamstatic.com
checkout.steampowered.com
community.steamstatic.com
help.steampowered.com
login.steampowered.com
media.steampowered.com
partner.steam-api.com
steam-chat.com
steamcdn-a.akamaihd.net
steamcommunity.com
steamstatic.com
store.steampowered.com
```

以下 wildcard 规则可被 ProxyOnly / PAC / System 模式匹配，但无法直接写进 hosts 文件：

```text
*.akamai.steamstatic.com
*.steam-chat.com
*.steamcommunity.com
*.steamstatic.com
```

如果必须用 Hosts 模式测试某个 wildcard 子域名，需要把具体域名加入 `hosts.extra_domains` 或 `rules.custom_domains`。如果要在不写 hosts 的情况下覆盖 wildcard 规则，可以使用 DNSIntercept manual 模式，并显式把测试 DNS 客户端指向本地 DNS 监听地址。

## 覆盖表

| 分组 | Hosts / rules | 默认 outbound profile | 启动探测 | 手动 smoke 状态 |
|---|---|---|---|---|
| 商店 | `store.steampowered.com`、`checkout.steampowered.com`、`help.steampowered.com`、`login.steampowered.com`、`media.steampowered.com` | `cdn-a.akamaihd.net` | store / help / media | Windows 中国网络 smoke 通过 |
| 社区 | `steamcommunity.com`，以及 Hosts exact 无法覆盖的 `*.steamcommunity.com` | `steamcommunity-a.akamaihd.net` | `steamcommunity.com` | Windows 中国网络 smoke 通过 |
| API | `api.steampowered.com`、`partner.steam-api.com` | `api.steampowered.com` 优先走 `steamstore.rmbgame.net`，保留证书链校验并容忍 hostname mismatch；`partner.steam-api.com` 仍走原始域名 fallback | `api.steampowered.com` | Go API smoke 待记录 |
| 聊天 | `steam-chat.com`，以及 Hosts exact 无法覆盖的 `*.steam-chat.com` | 原始域名 direct fallback | 默认不探测 | `steamcommunity.com/chat/` smoke 通过 |
| 静态资源 | `community.steamstatic.com`、`steamstatic.com`、`akamai.steamstatic.com`，以及 Hosts exact 无法覆盖的 static wildcard | `community.steamstatic.com` 有显式 profile；其他 static 域名走原始域名 fallback | `community.steamstatic.com` | Windows 中国网络 smoke 通过 |
| CDN | `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | `steamcdn-a.akamaihd.net` | Windows 中国网络 smoke 通过 |

## 手动 Smoke 记录模板

完成一次真实 Windows Hosts 模式测试后，用下表记录结果。不要把未实际测试的项目标记为通过。

环境：

```text
日期：
系统：
网络：
启动命令：
Root CA 状态：
端口：
浏览器 / Steam 客户端：
```

| 场景 | URL / 操作 | 期望结果 | 实际结果 | 通过 |
|---|---|---|---|---|
| 社区首页 | `https://steamcommunity.com/` | 页面内容加载 |  |  |
| 商店首页 | `https://store.steampowered.com/` | 页面内容加载 |  |  |
| 帮助首页 | `https://help.steampowered.com/` | 页面内容加载 |  |  |
| 官方 Web API | `https://api.steampowered.com/ISteamWebAPIUtil/GetSupportedAPIList/v1/?format=json` | 返回 JSON 响应 |  |  |
| 登录页 | 打开 Steam 登录流程 | 登录页资源加载 |  |  |
| 静态资源 | `https://community.steamstatic.com/` | 返回 HTTP 响应 |  |  |
| CDN 资源 | `https://steamcdn-a.akamaihd.net/` | 返回 HTTP 响应 |  |  |
| 聊天 / WebSocket | Steam Web Chat 或客户端内置浏览器 | WebSocket 连接成功，或失败可诊断 |  |  |

## 已记录 Smoke：Windows Hosts / 中国网络 / 2026-06-22

环境：

```text
日期：2026-06-22
系统：Windows
网络：中国网络
模式：Hosts + Direct + 默认 DoH + 默认 Steam outbound profiles
Root CA 状态：已安装并受信
端口：127.0.0.1:80 / 127.0.0.1:443
浏览器 / Steam 客户端：浏览器登录与 https://steamcommunity.com/chat/ 均可正常使用
```

| 场景 | URL / 操作 | 结果 | 通过 |
|---|---|---|---|
| Hosts 接管 | `Test-NetConnection steamcommunity.com -Port 443` | `RemoteAddress=127.0.0.1`，TCP 成功 | 是 |
| Hosts 接管 | `Test-NetConnection store.steampowered.com -Port 443` | `RemoteAddress=127.0.0.1`，TCP 成功 | 是 |
| Hosts 接管 | `Test-NetConnection help.steampowered.com -Port 443` | `RemoteAddress=127.0.0.1`，TCP 成功 | 是 |
| 社区首页 | `curl.exe --ssl-no-revoke -I https://steamcommunity.com/` | `HTTP/1.1 200 OK` | 是 |
| 商店首页 | `curl.exe --ssl-no-revoke -I https://store.steampowered.com/` | `HTTP/1.1 200 OK` | 是 |
| 帮助首页 | `curl.exe --ssl-no-revoke -I https://help.steampowered.com/` | `HTTP/1.1 302 Found` 到 `/en/` | 是 |
| 静态资源 | `curl.exe --ssl-no-revoke -I https://community.steamstatic.com/` | 返回 `HTTP/1.1 403 Forbidden`，说明 TLS 与上游 HTTP 响应链路可达 | 是 |
| 媒体资源 | `curl.exe --ssl-no-revoke -I https://media.steampowered.com/` | `HTTP/1.1 200 OK` | 是 |
| CDN 资源 | `curl.exe --ssl-no-revoke -I https://steamcdn-a.akamaihd.net/` | `HTTP/1.1 200 OK` | 是 |
| 登录流程 | Steam 登录页面 | 页面与交互正常使用 | 是 |
| 聊天 / WebSocket | `https://steamcommunity.com/chat/` | 页面与聊天功能正常使用 | 是 |

## 常见失败映射

| 现象 | 可能层级 | 下一步检查 |
|---|---|---|
| `startup_probe_failed ... stage=resolve` | DoH / DNS | 检查 `resolver_servers`、防火墙、DNS 劫持 |
| `stage=tcp` | Direct 可达性 | 检查候选 IP 可达性和 profile 目标 |
| `stage=tls` | TLS / SNI / 证书链 | 检查 `tls_server_name`、证书验证、本机时间 |
| 浏览器证书警告 | 本地 Root CA 信任 | 执行 `cert install`，检查配置的 Windows Root store |
| 浏览器能访问，`curl.exe` 报 `CRYPT_E_NO_REVOCATION_CHECK` | Windows Schannel 吊销检查 | 命令行验证使用 `curl.exe --ssl-no-revoke` |
| 主页面能打开，子资源失败 | 规则 / profile 缺口 | 用浏览器 devtools 查看失败 host，Hosts 模式下补 exact 域名 |
| WebSocket 失败 | 规则 / profile 或 upgrade 链路 | 检查日志，确认 websocket host 已被覆盖 |
