# steam-accelerator-core 中文实施文档与 Roadmap

> 文档版本：v0.2
> 维护语言：中文  
> 项目定位：Go 版 Steam 网络加速原子能力内核  
> 目标读者：项目作者、后续贡献者、SteamScope / steam-go 集成开发者  
> 参考项目：https://github.com/BeyondDimension/SteamTools
> 当前阶段：v0.2.0 Resolver / DoH / 上游代理已实现

---

## 1. 项目背景

`steam-accelerator-core` 的目标不是做一个完整的 Steam++ / Watt Toolkit 替代品，而是抽离并重新实现其中与“网络加速”相关的底层原子能力，形成一个可被 Go 项目复用的加速内核。

SteamTools / Watt Toolkit 的网络加速能力可以作为架构参考。其 README 中说明网络加速使用 `YARP.ReverseProxy` 进行本地反代，以支持更快访问游戏网站；源码中也能看到它把代理模式抽象为 `DNSIntercept`、`Hosts`、`System`、`VPN`、`ProxyOnly`、`PAC` 等模式。

本项目的重点不是网络诊断、端口扫描、Steam 登录或 UI 面板，而是提供可以被上层工具调用的核心能力：本地代理、PAC、系统代理接管、Hosts 接管、本地 CA、HTTPS 反代、自定义 DNS / DoH、上游出口等。

---

## 2. 项目目的

### 2.1 核心目的

构建一个用 Go 编写的 Steam 网络加速核心，提供以下能力：

1. 本地 HTTP Proxy / HTTPS CONNECT Proxy。
2. PAC 模式：只将 Steam 相关域名转发到本地代理。
3. System Proxy 模式：将系统代理切换到本地代理，并可恢复。
4. Hosts 模式：将 Steam 域名定向到本地反代，并可恢复。
5. 本地 CA 生成、安装、卸载。
6. 根据 SNI / Host 动态签发站点证书。
7. HTTPS 本地反向代理。
8. 自定义 DNS / DoH 解析与缓存。
9. 上游出口支持：Direct、HTTP Proxy、SOCKS5 Proxy。
10. 可启动、停止、恢复的 Engine 生命周期。

### 2.2 非目标

首期不做以下内容：

1. 不做本机代理端口扫描。
2. 不做网络诊断大面板。
3. 不做 Steam 登录、令牌、库存、成就、挂卡等 Steam 账号能力。
4. 不做游戏下载加速。
5. 不做 JS 注入。
6. 不做非 Steam 站点的大范围加速。
7. 不做节点池、公共代理池或代理节点服务。
8. 首期不做 VPN / TUN 模式。
9. 首期不做 DNSIntercept / 驱动级 DNS 拦截。
10. 不直接复制 SteamTools 源码。

---

## 3. 与 SteamTools 的关系

### 3.1 可以借鉴的内容

可以借鉴 SteamTools / Watt Toolkit 的以下架构思想：

1. 使用本地反代作为网络加速核心。
2. 将代理接管方式拆分为不同模式：ProxyOnly、PAC、System、Hosts、DNSIntercept、VPN。
3. 将加速服务拆成独立生命周期：启动、停止、恢复、异常退出恢复。
4. 将证书能力作为独立模块：根证书、本地证书、动态签发、信任安装、卸载。
5. 将 DNS / DoH 作为加速核心的一部分，而不是外围诊断能力。
6. 将代理出口作为可配置项：直连、二级代理、SOCKS5、HTTP Proxy。
7. 将被加速域名规则配置化，按服务分组。

### 3.2 不建议借鉴或首期不做的内容

1. 不做 JS 注入。该能力需要 HTTPS 解密并修改响应内容，安全争议和维护成本高。
2. 不做大而全的工具箱功能，只实现加速核心。
3. 不做 SteamTools 的 UI、账号、库存、令牌等非网络能力。
4. 不做驱动级 DNSIntercept，除非后续有明确 Windows 专项维护能力。
5. 不做 VPN / TUN 模式，避免把项目复杂度拉到系统级网络工具。

### 3.3 许可证边界

SteamTools 使用 GPL-3.0 许可证。若直接复制、改写或剥离其源码并发布衍生项目，通常会受到 GPL 许可证约束，需要按 GPL 兼容方式开源和分发。

本项目推荐采用 clean-room 思路：

1. 可以阅读 SteamTools 的公开文档、README、架构、模块划分。
2. 可以学习其代理模式划分和工程边界。
3. 不复制代码。
4. 不搬运 C# 实现。
5. 不把 SteamTools 的具体实现文件直接翻译成 Go。
6. 代码实现以 Go 标准库、独立开源库和自研逻辑为主。

建议在仓库 README 中明确写明：

```text
本项目参考 Watt Toolkit / SteamTools 的网络加速架构思想，但不包含、不复制、不移植其源码。
```

---

## 4. 项目定位

建议项目命名：

```text
steam-accelerator-core
```

定位：

```text
A Go-based Steam local acceleration core.
```

中文描述：

```text
一个用 Go 编写的 Steam 本地网络加速核心，提供本地代理、PAC、Hosts 反代、自定义 DNS、HTTPS 反代与上游代理出口等原子能力。
```

适合被以下项目集成：

1. SteamScope 桌面端。
2. steam-go 工具链。
3. Go/Wails 桌面工具。
4. 独立 CLI 加速器。
5. 本地服务 sidecar。

---

## 5. 总体设计

### 5.1 整体架构

```text
steam-accelerator-core
├── engine       生命周期控制：Start / Stop / Restore / Status
├── rules        Steam 域名规则与匹配
├── proxy        HTTP Proxy / HTTPS CONNECT / PAC Server
├── reverse      Hosts 模式下的 HTTP/HTTPS Reverse Proxy
├── resolver     DNS / DoH / 缓存 / IP 选择
├── upstream     Direct / HTTP Proxy / SOCKS5 Proxy
├── cert         Root CA / 动态证书签发 / 信任安装卸载
├── patcher      Hosts / System Proxy / PAC 配置修改与恢复
├── config       配置加载、校验、默认值
├── log          结构化日志
└── cmd          CLI 或服务入口
```

### 5.2 流量模式总览

#### ProxyOnly 模式

```text
应用手动配置代理
        ↓
127.0.0.1:26501
        ↓
HTTP Proxy / CONNECT Proxy
        ↓
resolver + upstream
        ↓
Steam 真实服务
```

特点：

1. 不修改系统。
2. 不安装证书。
3. 不解密 HTTPS。
4. 最适合作为第一阶段实现。

#### PAC 模式

```text
系统 / 浏览器读取 PAC
        ↓
命中 Steam 域名 → 127.0.0.1:26501
其他域名 → DIRECT
        ↓
HTTP Proxy / CONNECT Proxy
        ↓
Steam 真实服务
```

特点：

1. 只接管 Steam 域名。
2. 对浏览器、WebView 友好。
3. 比全局系统代理更安全。
4. 需要平台级 PAC 配置写入与恢复。

#### System Proxy 模式

```text
系统代理 → 127.0.0.1:26501
        ↓
本地代理判断域名
        ↓
Steam 域名转发
非 Steam 域名可 DIRECT 或拒绝
```

特点：

1. 接管范围更大。
2. 需要谨慎处理非 Steam 流量。
3. 必须完整恢复系统代理。
4. Windows / macOS / Linux 差异较大。

#### Hosts + HTTPS Reverse Proxy 模式

```text
steamcommunity.com / store.steampowered.com
        ↓
hosts 指向 127.0.0.1
        ↓
本地 80 / 443 反代服务
        ↓
本地 CA 动态签发证书
        ↓
HTTPS Reverse Proxy
        ↓
resolver + upstream
        ↓
Steam 真实服务
```

特点：

1. 最接近 Steam++ 的核心体验。
2. 可以让不走系统代理的程序也命中本地反代。
3. 需要本地 CA、hosts、监听 80/443、权限提升。
4. 风险最高，必须做事务化恢复。

---

## 6. 推荐实现顺序

推荐不要一上来实现 Hosts + HTTPS 反代，而是先把代理核心打稳。

```text
v0.1：ProxyOnly + CONNECT
v0.2：DNS / DoH / resolver
v0.3：PAC + System Proxy
v0.4：Hosts + HTTPS Reverse Proxy
v0.5：稳定性、安全恢复、跨平台打磨
v1.0：作为可集成 core 发布
```

---

## 7. 模块设计

### 7.1 engine 模块

职责：

1. 读取配置。
2. 初始化 rules、resolver、upstream、proxy、reverse、patcher。
3. 根据模式启动对应服务。
4. 提供停止和恢复能力。
5. 暴露状态查询。

核心接口：

```go
type Engine interface {
    Start(ctx context.Context, cfg Config) error
    Stop(ctx context.Context) error
    Restore(ctx context.Context) error
    Status() Status
}
```

状态结构：

```go
type Status struct {
    Running     bool
    Mode        Mode
    ListenAddr  string
    StartedAt   time.Time
    LastError   string
    RuleCount   int
    ActiveConns int64
}
```

实现要求：

1. `Start` 必须幂等，重复启动时返回明确错误或自动重启。
2. `Stop` 必须先停服务，再恢复系统变更。
3. `Restore` 必须能在服务未运行时独立执行，用于崩溃恢复。
4. 所有系统修改都必须记录 rollback 状态。

---

### 7.2 rules 模块

职责：

1. 管理 Steam 域名规则。
2. 支持精确域名、通配符、后缀匹配。
3. 支持按业务分组：store、community、api、chat、static、cdn。
4. 为 PAC、Proxy、Hosts、Reverse Proxy 提供统一匹配结果。

示例规则：

```go
type RuleGroup struct {
    Name    string
    Domains []string
}

var DefaultSteamRules = []RuleGroup{
    {
        Name: "store",
        Domains: []string{
            "store.steampowered.com",
            "checkout.steampowered.com",
            "help.steampowered.com",
        },
    },
    {
        Name: "community",
        Domains: []string{
            "steamcommunity.com",
            "*.steamcommunity.com",
        },
    },
    {
        Name: "api",
        Domains: []string{
            "api.steampowered.com",
            "partner.steam-api.com",
        },
    },
    {
        Name: "chat",
        Domains: []string{
            "steam-chat.com",
            "*.steam-chat.com",
        },
    },
    {
        Name: "static",
        Domains: []string{
            "steamstatic.com",
            "*.steamstatic.com",
            "akamai.steamstatic.com",
            "*.akamai.steamstatic.com",
        },
    },
}
```

匹配接口：

```go
type Matcher interface {
    MatchHost(host string) (MatchResult, bool)
}

type MatchResult struct {
    Host      string
    GroupName string
    Rule      string
}
```

实现要求：

1. host 需要统一小写。
2. 需要去掉端口。
3. 需要处理 IDNA/Punycode。
4. 通配符规则仅允许前缀 `*.`。
5. 禁止默认代理非规则域名。

---

### 7.3 proxy 模块

职责：

1. 实现 HTTP Proxy。
2. 实现 HTTPS CONNECT 隧道。
3. 只允许规则命中的 Steam 域名进入加速链路。
4. 生成 PAC 文件。
5. 暴露本地 PAC Server。

CONNECT 处理流程：

```text
收到 CONNECT store.steampowered.com:443
        ↓
解析 Host
        ↓
rules.MatchHost(host)
        ↓
命中 → upstream.DialContext
未命中 → DIRECT / 拒绝 / 按配置处理
        ↓
返回 200 Connection Established
        ↓
io.Copy 双向转发
```

关键接口：

```go
type ProxyServer interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Addr() string
}
```

配置建议：

```go
type ProxyConfig struct {
    ListenAddr       string
    AllowNonSteam    bool
    NonSteamBehavior string // direct | reject
    IdleTimeout      time.Duration
    ReadTimeout      time.Duration
    WriteTimeout     time.Duration
}
```

实现要求：

1. HTTPS CONNECT 不解密。
2. HTTP 普通请求可以转发，但仍必须走规则匹配。
3. CONNECT 隧道要设置超时，避免连接泄露。
4. 必须限制只监听 `127.0.0.1`，除非用户显式开启局域网访问。
5. 日志不要记录 Cookie、Authorization、完整 URL 查询参数。

---

### 7.4 PAC 模块

职责：

1. 根据 rules 生成 PAC 文件。
2. 启动本地 PAC 服务。
3. 可供 System Proxy 模块写入 PAC 地址。

PAC 输出示例：

```js
function FindProxyForURL(url, host) {
  host = host.toLowerCase();

  if (
    dnsDomainIs(host, "steamcommunity.com") ||
    shExpMatch(host, "*.steamcommunity.com") ||
    dnsDomainIs(host, "store.steampowered.com") ||
    dnsDomainIs(host, "api.steampowered.com") ||
    dnsDomainIs(host, "steam-chat.com") ||
    shExpMatch(host, "*.steamstatic.com")
  ) {
    return "PROXY 127.0.0.1:26501";
  }

  return "DIRECT";
}
```

实现要求：

1. PAC 只命中 Steam 域名。
2. PAC 规则从 rules 模块生成，不手写两套规则。
3. PAC Server 默认监听 `127.0.0.1`。
4. PAC 地址示例：`http://127.0.0.1:26502/proxy.pac`。

---

### 7.5 resolver 模块

职责：

1. 提供统一域名解析接口。
2. 支持系统 DNS、UDP DNS、TCP DNS、DoH。
3. 支持缓存。
4. 支持 IPv4 / IPv6 策略。
5. 支持解析失败回退。

接口：

```go
type Resolver interface {
    Resolve(ctx context.Context, host string) ([]net.IP, error)
}
```

配置：

```go
type ResolverConfig struct {
    Mode        string   // system | udp | tcp | doh
    Servers     []string
    PreferIPv4  bool
    PreferIPv6  bool
    DisableIPv6 bool
    CacheTTL    time.Duration
    Timeout     time.Duration
}
```

解析策略：

```text
1. 检查缓存
2. 根据 resolver mode 查询
3. 过滤不可用 IP 类型
4. 按 IPv4/IPv6 偏好排序
5. 返回候选 IP 列表
```

实现要求：

1. DNS 结果必须缓存。
2. 失败时需要回退到备用 DNS。
3. DoH 地址不可写死为单点。
4. 不做公共代理节点，只做解析与转发。
5. 后续可以加入连接成功率缓存，但 v0.2 先不做复杂评分。

---

### 7.6 upstream 模块

职责：

1. 提供统一拨号能力。
2. 支持直连。
3. 支持 HTTP 上游代理。
4. 支持 SOCKS5 上游代理。
5. 与 resolver 结合，实现“域名解析后拨号到指定 IP，同时保留原始 SNI”。

接口：

```go
type Dialer interface {
    DialContext(ctx context.Context, network, address string) (net.Conn, error)
}
```

上游类型：

```go
type UpstreamType string

const (
    UpstreamDirect UpstreamType = "direct"
    UpstreamHTTP   UpstreamType = "http"
    UpstreamSOCKS5 UpstreamType = "socks5"
)
```

实现要求：

1. direct 模式使用 resolver 解析真实 IP。
2. HTTP/SOCKS5 上游可以选择让上游解析域名，也可以本地解析后连接 IP。
3. 支持认证代理，但配置中不要明文打印密码。
4. 所有连接需要超时。

---

### 7.7 reverse 模块

职责：

1. Hosts 模式下监听本地 HTTP / HTTPS。
2. 根据 Host 反代到真实 Steam 服务。
3. HTTPS 入口使用本地动态证书。
4. 出口到 Steam 时仍使用真实域名作为 SNI。
5. 支持 WebSocket。

关键流程：

```text
用户请求 https://steamcommunity.com
        ↓
hosts → 127.0.0.1
        ↓
本地 HTTPS Server
        ↓
cert.GetCertificate 根据 SNI 签发证书
        ↓
httputil.ReverseProxy
        ↓
resolver 解析真实 Steam IP
        ↓
Transport Dial 到真实 IP
        ↓
TLS ServerName 仍为 steamcommunity.com
```

Transport 注意事项：

1. `req.URL.Scheme = "https"`。
2. `req.URL.Host = originalHost`。
3. `req.Host = originalHost`。
4. TLS `ServerName` 必须是原始域名，而不是解析后的 IP。
5. 保留必要 Header，但不要破坏 Cookie。
6. 对 `Connection`、`Upgrade`、`Proxy-*` 头要谨慎处理。

实现要求：

1. 只反代规则命中的 Steam 域名。
2. 非规则域名拒绝。
3. 反代错误返回清晰的 502。
4. 不修改响应体。
5. 不注入 JS。
6. 支持 WebSocket 连接升级。

---

### 7.8 cert 模块

职责：

1. 生成本地 Root CA。
2. 安装 Root CA 到系统信任区。
3. 卸载 Root CA。
4. 根据 SNI / Host 动态签发站点证书。
5. 缓存已签发证书。

接口：

```go
type CertManager interface {
    EnsureRootCA(ctx context.Context) error
    InstallRootCA(ctx context.Context) error
    RemoveRootCA(ctx context.Context) error
    GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
}
```

证书策略：

1. Root CA 存储在用户配置目录。
2. Root CA 名称带项目标识，例如 `steam-accelerator-core local root ca`。
3. 每个域名动态签发短周期证书。
4. 站点证书缓存到内存即可，必要时落盘。
5. 停止加速不一定卸载 CA，但必须提供显式卸载能力。

安全要求：

1. 私钥文件权限必须收紧。
2. 不导出用户证书私钥。
3. 明确告知用户安装本地 CA 的风险。
4. 默认不启用 Hosts + HTTPS 反代模式。
5. 提供 `restore-cert` / `uninstall-ca` 命令。

---

### 7.9 patcher 模块

职责：

1. 修改 hosts。
2. 恢复 hosts。
3. 设置系统代理。
4. 恢复系统代理。
5. 设置 PAC。
6. 管理 rollback 状态。

hosts block 设计：

```text
# steam-accelerator-core start
127.0.0.1 steamcommunity.com
127.0.0.1 store.steampowered.com
127.0.0.1 api.steampowered.com
127.0.0.1 help.steampowered.com
127.0.0.1 steam-chat.com
# steam-accelerator-core end
```

实现要求：

1. 只删除自己标记区块。
2. 写入前备份原始 hosts。
3. 写入失败必须回滚。
4. 不能破坏用户原有 hosts 内容。
5. Windows 需要管理员权限。
6. macOS/Linux 需要 sudo 或权限提升。
7. 系统代理设置必须记录原值，停止时恢复原值。

---

## 8. 配置设计

建议先使用 YAML 或 TOML。示例：

```yaml
mode: proxy_only

proxy:
  listen_addr: "127.0.0.1:26501"
  pac_addr: "127.0.0.1:26502"
  allow_non_steam: false
  non_steam_behavior: "direct"

resolver:
  mode: "doh"
  servers:
    - "https://dns.google/dns-query"
    - "https://cloudflare-dns.com/dns-query"
  prefer_ipv4: true
  disable_ipv6: false
  cache_ttl: "10m"
  timeout: "5s"

upstream:
  type: "direct"
  address: ""
  username: ""
  password: ""

hosts:
  enabled: false
  listen_http: "127.0.0.1:80"
  listen_https: "127.0.0.1:443"

cert:
  auto_install: false
  ca_name: "steam-accelerator-core local root ca"

rules:
  enable_default_steam_rules: true
  custom_domains: []
```

---

## 9. CLI 设计

建议先提供 CLI，后续再接 Wails UI。

```bash
steam-accelerator start --mode proxy-only
steam-accelerator start --mode pac
steam-accelerator start --mode system
steam-accelerator start --mode hosts
steam-accelerator stop
steam-accelerator status
steam-accelerator restore
steam-accelerator cert install
steam-accelerator cert uninstall
steam-accelerator rules list
steam-accelerator config init
```

命令说明：

| 命令 | 作用 |
|---|---|
| `start` | 启动加速服务 |
| `stop` | 停止服务并恢复系统配置 |
| `status` | 查看当前运行状态 |
| `restore` | 强制执行 hosts、system proxy、PAC、证书相关恢复 |
| `cert install` | 安装本地 Root CA |
| `cert uninstall` | 卸载本地 Root CA |
| `rules list` | 查看当前 Steam 域名规则 |
| `config init` | 生成默认配置 |

---

## 10. 测试设计

### 10.1 单元测试

| 模块 | 测试重点 |
|---|---|
| rules | 精确匹配、通配符匹配、端口剥离、大小写处理 |
| resolver | 缓存、超时、fallback、IPv4/IPv6 策略 |
| proxy | CONNECT 握手、非 Steam 拒绝、连接关闭 |
| pac | PAC 生成内容、规则一致性 |
| hosts patcher | block 插入、删除、重复执行、备份恢复 |
| cert | Root CA 生成、证书签发、缓存 |
| upstream | direct/http/socks5 拨号接口 |

### 10.2 集成测试

1. 启动 ProxyOnly 后通过本地代理访问测试 HTTPS 服务。
2. 启动 PAC Server 后检查 PAC 内容。
3. 使用临时 hosts 文件测试 patcher，不直接改系统 hosts。
4. 使用本地 HTTPS 测试服务验证动态证书签发。
5. 使用 `httptest.Server` 模拟 Steam 域名反代。
6. 使用 fake resolver 固定解析结果，避免测试依赖公网。

### 10.3 手动验收

| 场景 | 验收标准 |
|---|---|
| ProxyOnly | 浏览器手动配置代理后 Steam 域名可访问 |
| PAC | 系统 PAC 后 Steam 域名走代理，其他域名 DIRECT |
| System Proxy | 开启后系统代理指向本地，关闭后恢复原值 |
| Hosts | hosts 写入区块正确，关闭后完整移除 |
| HTTPS Reverse | 浏览器访问 Steam 域名时证书可信，内容可正常返回 |
| Restore | 模拟崩溃后执行 restore 能恢复 hosts / 代理配置 |

---

## 11. 安全设计

### 11.1 基本原则

1. 默认只监听 `127.0.0.1`。
2. 默认只代理 Steam 规则域名。
3. 默认不解密 HTTPS。
4. 默认不安装 CA。
5. 默认不修改 hosts。
6. 所有系统修改必须可恢复。
7. 日志不记录敏感头、不记录 Cookie、不记录完整 Token。
8. 不提供公共代理节点。
9. 不转发非用户主动配置的外部入口。

### 11.2 Hosts + HTTPS 反代安全提示

开启 Hosts + HTTPS 反代前必须提示：

1. 该模式会修改系统 hosts。
2. 该模式需要安装本地 Root CA。
3. 该模式会让本地服务终止 HTTPS。
4. 本项目不会注入 JS，不会修改响应内容。
5. 可随时执行 `steam-accelerator restore` 恢复。
6. 可执行 `steam-accelerator cert uninstall` 卸载本地 CA。

---

## 12. Roadmap 维护方式

本项目需要维护中文 Roadmap。建议仓库中放置：

```text
ROADMAP.md
```

Roadmap 使用中文维护，每个版本包含：

1. 版本目标。
2. 核心任务。
3. 非目标。
4. 验收标准。
5. 风险说明。
6. 状态：计划中 / 开发中 / 已完成 / 延后。

Issue 标签建议：

| 标签 | 含义 |
|---|---|
| `area:proxy` | 本地代理能力 |
| `area:resolver` | DNS / DoH |
| `area:pac` | PAC 模式 |
| `area:system-proxy` | 系统代理修改 |
| `area:hosts` | hosts 修改与恢复 |
| `area:reverse-proxy` | HTTPS 反代 |
| `area:cert` | 本地 CA 与证书 |
| `area:upstream` | 上游代理出口 |
| `area:security` | 安全与权限 |
| `area:docs` | 文档 |
| `priority:p0` | 必须完成 |
| `priority:p1` | 高优先级 |
| `priority:p2` | 普通优先级 |
| `status:blocked` | 阻塞 |
| `status:deferred` | 延后 |

---

## 13. 中文 Roadmap

### v0.1.0：ProxyOnly 加速内核

目标：完成最小可用的本地代理核心，不修改系统、不安装证书。

任务：

- [x] 建立项目结构。
- [x] 实现 Config 加载与默认配置。
- [x] 实现 Steam 默认域名规则。
- [x] 实现 rules.Matcher。
- [x] 实现 HTTP Proxy 基础框架。
- [x] 实现 HTTPS CONNECT 隧道。
- [x] 实现 Direct upstream。
- [x] 实现基础日志。
- [x] 实现 Engine Start / Stop / Status。
- [x] 提供 CLI：`start --mode proxy-only`、`stop`、`status`。
- [x] 添加 rules、proxy、engine 单元测试。

非目标：

- 不做 PAC。
- 不做系统代理修改。
- 不做 hosts。
- 不做本地 CA。
- 不做 HTTPS 反代。

验收标准：

1. 本地监听 `127.0.0.1:26501`。
2. 浏览器手动配置 HTTP Proxy 后，Steam 规则域名通过本地代理转发。
3. 非 Steam 域名默认不进入加速链路。
4. `stop` 后端口释放。

---

### v0.2.0：Resolver / DoH / 上游代理

目标：完成 DNS / DoH 与上游出口能力，让加速核心具备真实可配置性。

任务：

- [x] 实现 Resolver 接口。
- [x] 实现系统 DNS resolver。
- [x] 实现 UDP DNS resolver。
- [x] 实现 TCP DNS resolver。
- [x] 实现 DoH resolver。
- [x] 实现 DNS 缓存。
- [x] 实现 IPv4 / IPv6 策略。
- [x] 实现 HTTP upstream。
- [x] 实现 SOCKS5 upstream。
- [x] 实现代理认证配置。
- [x] 在 proxy 的 Dial 链路中接入 resolver + upstream。
- [x] 添加 resolver、upstream 单元测试。

非目标：

- 不做节点测速。
- 不做本地端口扫描。
- 不做 DNSIntercept。

验收标准：

1. 可通过配置切换 system / udp / tcp / doh。
2. DoH 失败可 fallback。
3. 支持通过用户配置的 HTTP/SOCKS5 上游转发 Steam 流量。
4. 日志不输出代理密码。

---

### v0.3.0：PAC 与 System Proxy

目标：支持通过 PAC 或系统代理接管 Steam 相关域名流量。

任务：

- [ ] 实现 PAC 生成器。
- [ ] 实现 PAC Server。
- [ ] 实现 `start --mode pac`。
- [ ] Windows 系统 PAC 写入与恢复。
- [ ] macOS 系统 PAC 写入与恢复。
- [ ] Windows 系统 HTTP/HTTPS 代理写入与恢复。
- [ ] macOS 系统 HTTP/HTTPS 代理写入与恢复。
- [ ] 实现 rollback 状态文件。
- [ ] 实现 `restore` 命令。
- [ ] 添加 PAC 与 System Proxy 集成测试。

非目标：

- Linux 桌面环境系统代理首期可以只提供文档，不强行统一。
- 不做 hosts。
- 不做本地 CA。

验收标准：

1. PAC 文件只命中 Steam 规则域名。
2. 开启 PAC 后系统 PAC 指向本地 PAC Server。
3. 关闭后恢复原系统代理配置。
4. 模拟崩溃后执行 `restore` 可以恢复系统代理。

---

### v0.4.0：Hosts + HTTPS Reverse Proxy

目标：实现接近 Steam++ 的本地反代模式。

任务：

- [ ] 实现 hosts patcher。
- [ ] 实现 hosts 备份与恢复。
- [ ] 实现 Root CA 生成。
- [ ] 实现 Root CA 安装与卸载。
- [ ] 实现动态站点证书签发。
- [ ] 实现本地 HTTP Server。
- [ ] 实现本地 HTTPS Server。
- [ ] 实现 HTTPS Reverse Proxy。
- [ ] 保留原始 Host 与 SNI。
- [ ] 支持 WebSocket。
- [ ] 实现 `start --mode hosts`。
- [ ] 实现 `cert install` / `cert uninstall`。
- [ ] 添加 hosts、cert、reverse 集成测试。

非目标：

- 不做 JS 注入。
- 不修改响应体。
- 不做 DNSIntercept。
- 不做 VPN / TUN。

验收标准：

1. hosts 中只写入项目标记区块。
2. 停止后完整移除项目标记区块。
3. 本地 CA 安装后浏览器访问规则域名证书可信。
4. 反代出口仍使用真实 Steam 域名作为 SNI。
5. `restore` 可恢复 hosts 与证书相关状态。

---

### v0.5.0：稳定性、安全与跨平台打磨

目标：把 v0.1 至 v0.4 的能力打磨为可集成的 core。

任务：

- [ ] 完善错误码。
- [ ] 完善结构化日志。
- [ ] 增加连接数统计。
- [ ] 增加 active connection graceful shutdown。
- [ ] 增加配置校验。
- [ ] 增加安全说明文档。
- [ ] 增加崩溃恢复文档。
- [ ] 增加 Windows 管理员权限说明。
- [ ] 增加 macOS 权限说明。
- [ ] 完善 CI。
- [ ] 完善 Go Report / lint / staticcheck。
- [ ] 添加基础 benchmark。

验收标准：

1. 常见错误有明确提示。
2. 启动、停止、恢复流程稳定。
3. 所有系统修改都有回滚路径。
4. README 能指导用户理解每种模式的风险。

---

### v1.0.0：稳定 API 与集成发布

目标：形成稳定 API，允许 SteamScope / steam-go / Wails 桌面端集成。

任务：

- [ ] 稳定 Engine API。
- [ ] 稳定 Config 结构。
- [ ] 稳定 Mode 枚举。
- [ ] 提供 Go package 集成示例。
- [ ] 提供 CLI 使用示例。
- [ ] 提供 Wails 集成建议。
- [ ] 提供安全边界说明。
- [ ] 提供 CHANGELOG。
- [ ] 发布 v1.0.0。

验收标准：

1. 可作为 Go library 使用。
2. 可作为 CLI 使用。
3. 可被 Wails UI 调用。
4. 文档覆盖 ProxyOnly、PAC、System、Hosts 模式。
5. 中文 Roadmap 与 CHANGELOG 完整。

---

## 14. 延后路线

以下内容不进入 v1.0，但可以作为远期研究方向。

### DNSIntercept

价值：

1. 对不走系统代理、不易被 hosts 接管的流量可能更有效。
2. 更接近 SteamTools 的高级模式。

问题：

1. Windows Only 倾向明显。
2. 可能需要驱动、WFP、WinDivert 或类似技术。
3. 权限和安全风险高。
4. 维护复杂度高。

建议：v1.0 后再评估。

### VPN / TUN 模式

价值：

1. 流量接管能力更强。
2. 可以统一处理系统流量。

问题：

1. 复杂度接近 VPN 客户端。
2. 路由、DNS、虚拟网卡、权限、兼容性问题多。
3. 偏离“Steam 加速核心”的轻量定位。

建议：除非项目成熟且有明确需求，否则不做。

### JS 注入

价值：

1. 可以增强网页体验。
2. 可做类似网页插件的功能。

问题：

1. 需要 MITM 修改响应内容。
2. 安全争议明显。
3. 不符合“加速原子能力”的定位。

建议：不做。

---

## 15. 推荐仓库文件

```text
steam-accelerator-core/
├── README.md
├── ROADMAP.md
├── CHANGELOG.md
├── SECURITY.md
├── LICENSE
├── cmd/
│   └── steam-accelerator/
├── internal/
│   ├── engine/
│   ├── rules/
│   ├── proxy/
│   ├── reverse/
│   ├── resolver/
│   ├── upstream/
│   ├── cert/
│   ├── patcher/
│   ├── config/
│   └── log/
├── examples/
│   ├── proxy-only/
│   ├── pac/
│   └── hosts/
└── docs/
    ├── design.zh-CN.md
    ├── security.zh-CN.md
    ├── modes.zh-CN.md
    ├── restore.zh-CN.md
    └── steamtools-reference.zh-CN.md
```

---

## 16. README 首屏建议

```markdown
# steam-accelerator-core

steam-accelerator-core 是一个用 Go 编写的 Steam 本地网络加速核心，提供本地代理、PAC、Hosts 反代、自定义 DNS、HTTPS 反代与上游代理出口等原子能力。

本项目参考 Watt Toolkit / SteamTools 的网络加速架构思想，但不包含、不复制、不移植其源码。

## 特性

- HTTP Proxy / HTTPS CONNECT
- Steam 域名规则匹配
- PAC Server
- System Proxy 模式
- Hosts 模式
- 本地 Root CA 与动态证书
- HTTPS Reverse Proxy
- DoH / DNS 缓存
- Direct / HTTP Proxy / SOCKS5 上游出口

## 非目标

- 不提供代理节点
- 不做 Steam 登录
- 不做 JS 注入
- 不做游戏下载加速
- 不做完整 Steam++ 替代品
```

---

## 17. 开发优先级建议

最高优先级：

1. Engine 生命周期。
2. rules.Matcher。
3. HTTP Proxy / CONNECT。
4. Direct upstream。
5. Resolver 接口。
6. PAC 生成。

中优先级：

1. DoH。
2. HTTP/SOCKS5 upstream。
3. System Proxy。
4. rollback 状态。
5. CLI。

高风险但高价值：

1. Hosts patcher。
2. Root CA。
3. HTTPS Reverse Proxy。
4. 动态证书签发。

延后：

1. DNSIntercept。
2. VPN / TUN。
3. JS 注入。
4. 多站点加速。

---

## 18. 关键风险

| 风险 | 影响 | 应对 |
|---|---|---|
| hosts 修改失败 | 用户网络异常 | 事务化写入、备份、restore |
| 系统代理恢复失败 | 影响全局网络 | 保存原值、单独 restore 命令 |
| 本地 CA 引发用户不信任 | 安全信任问题 | 默认关闭、明确说明、可卸载 |
| 非 Steam 流量被代理 | 隐私风险 | 默认只代理规则域名 |
| 端口冲突 | 启动失败 | 明确报错，允许配置端口 |
| DNS / DoH 服务失效 | 加速不可用 | 多服务器 fallback |
| 复制 SteamTools 代码 | GPL 合规风险 | clean-room 实现 |
| Steam 域名变化 | 加速覆盖不足 | 维护 rules，允许用户扩展 |

---

## 19. 最小可用定义

`steam-accelerator-core` 的最小可用版本不是 Hosts 反代，而是 ProxyOnly 模式。

MVP 标准：

1. 本地启动 HTTP Proxy。
2. 支持 HTTPS CONNECT。
3. 只代理 Steam 规则域名。
4. 支持 Direct upstream。
5. 支持配置文件。
6. 支持启动、停止、状态查询。
7. 有基础测试。

完成 MVP 后，再进入 PAC、System Proxy、Hosts 反代。

---

## 20. 总结

`steam-accelerator-core` 应该是一个聚焦、可复用、可恢复、可集成的 Go 加速内核。

短期目标不是复制 Steam++ 的全部功能，而是用 Go 实现它网络加速部分最关键的工程能力：

```text
规则 → 接管 → 代理 → DNS → 上游 → 恢复
```

长期目标是形成一个能被 SteamScope、steam-go 或其他 Go/Wails 桌面工具复用的 Steam 网络加速基础设施。

实现策略应坚持：

1. 先 ProxyOnly，后 PAC/System，再 Hosts 反代。
2. 先不解密 HTTPS，后提供高级模式。
3. 默认安全，显式开启高风险能力。
4. 所有系统修改都必须可恢复。
5. 借鉴 SteamTools 架构思想，但不复制其 GPL 源码。

---

## 参考资料

1. SteamTools / Watt Toolkit GitHub 仓库：`https://github.com/BeyondDimension/SteamTools`
2. SteamTools 网络加速 README 说明：使用 `YARP.ReverseProxy` 本地反代。
3. SteamTools ProxyMode：`DNSIntercept`、`Hosts`、`System`、`VPN`、`ProxyOnly`、`PAC`。
4. SteamTools License：GPL-3.0。
5. YARP Reverse Proxy：`https://github.com/dotnet/yarp`
