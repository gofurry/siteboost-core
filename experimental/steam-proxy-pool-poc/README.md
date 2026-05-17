# steam-proxy-pool-poc

Wails v2 + Vue-TS 探索项目：Steam 网络连通性诊断、本机代理发现和请求出口选择器。

本实验已经放弃公共代理池方向，不提供代理节点，不维护反代，只帮助用户检测自己机器上已有的网络出口。

## 验证目标

1. 检测直连 Steam 是否可用。
2. 检测系统代理是否可用，包括环境变量和 Windows 系统代理。
3. 扫描常见本机代理端口。
4. 支持手动输入代理地址并测试。
5. 推荐一个可用于 SteamScope 请求的出口。

## 诊断口径

本 POC 检测的是 SteamScope 当前进程能否访问 Steam，不等同于整台电脑、Steam 客户端或浏览器是否可访问 Steam。

如果用户使用 UU、雷神等加速器，浏览器或 Steam 客户端可能已经被加速器接管，但 SteamScope 这个独立桌面进程未被接管，此时本 POC 的直连检测仍可能失败。产品化时应把这个结果解释为“SteamScope 当前出口不可用”，并建议用户：

- 将 SteamScope 加入加速器的加速进程或游戏列表。
- 开启加速器的 TUN / 全局模式。
- 使用加速器或代理客户端提供的本机 HTTP / SOCKS 代理端口。

## 默认扫描端口

HTTP / Mixed：

```text
127.0.0.1:7890
127.0.0.1:7897
127.0.0.1:8080
127.0.0.1:10809
127.0.0.1:20171
```

SOCKS5：

```text
127.0.0.1:1080
127.0.0.1:7891
127.0.0.1:10808
```

## 系统代理读取

- 环境变量：`HTTPS_PROXY`、`HTTP_PROXY`、`ALL_PROXY`，兼容小写。
- Windows 注册表：`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
- 支持 `ProxyEnable` + `ProxyServer`。
- 检测到 `AutoConfigURL` / PAC 时只提示，不解析执行。

## 运行

```bash
npm --prefix frontend install
npm --prefix frontend run build
go test ./...
wails dev
```

## 安全边界

- 不读取浏览器 Cookie。
- 不读取 Steam 客户端本地登录态。
- 不保存代理配置。
- HTTP 公共代理不适合承载敏感登录态；本工具只检测本机已有出口并展示风险。
