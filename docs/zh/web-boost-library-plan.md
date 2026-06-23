# web-boost 开源库抽取规划

> 目标仓库：[gofurry/web-boost](https://github.com/gofurry/web-boost)
> 当前来源仓库：`gofurry/siteboost-core`
> 当前阶段：v0.8.0 抽取准备

## 定位

`web-boost` 是未来正式维护的通用 Go 本地 Web 加速库。它只抽取当前实验仓库中已经验证过的核心能力，不继承 `siteboost-core` / `go-steam-core` 的历史命名、CLI 组织、Windows AppHost 安装流程、Steam 专用入口或实验性目录包袱。

`siteboost-core` 继续作为实验验证仓库，负责验证真实链路、记录 smoke、沉淀失败经验和提供迁移来源。`web-boost` 负责提供高可维护、高可扩展、贴近真实开发集成的 Go library API。

## 抽取原则

- 根包只暴露稳定、少量、可理解的核心 API，不把各种能力平铺到仓库根目录。
- 公共 API 使用中性命名：`webboost`、`Engine`、`Provider`、`Mode`、`Status`、`Start`、`Stop`、`Restore`，不出现 Steam 专用或实验 CLI 命名。
- 所有阻塞、网络、文件、系统修改和长期运行操作都必须接收 `context.Context`。
- 默认不修改系统状态；会修改系统代理、hosts、证书、系统 DNS 的能力必须显式启用、可观察、可 rollback、可 restore。
- DNSIntercept manual 与 Page Enhance 默认不修改开发者系统环境。
- Provider 只描述站点规则、outbound profile、probe、可选 enhancement metadata；不拥有 hosts、证书、DNS 接管、AppHost 或系统修改职责。
- 系统修改能力通过 adapter / executor 边界接入，不能把当前实验仓库的 AppHost service 安装命令带入核心库。
- 依赖保持克制。核心包优先标准库；DNS wire、证书和平台能力的第三方依赖必须有明确收益，并隔离在具体包或 adapter 中。
- 所有可复用类型的并发安全性必须在文档中写明。
- 示例优先：每个关键能力都要有可运行 example，不要求 live GitHub / Steam 可达才能跑默认测试。

## 不抽取内容

- 当前 `cmd/steam-accelerator` CLI。
- `steam-accelerator-core`、`go-steam-core`、`steam-accelerator` 等历史命名。
- AppHost service 的安装、卸载、Windows SCM 操作和 named pipe 协议细节。
- 真实 Steam provider 的产品化承诺、Steam 专用 smoke 文案和本机路径。
- 临时 runtime state、bin、证书、私钥、日志、用户机器上的测试输出。
- 桌面 installer、发布签名、自动升级和 GUI 壳。
- TUN / VPN 实现。未来如需要，只作为 adapter 对接成熟外部库或独立项目。

## 推荐目录层级

建议根目录保持“入口少、能力分层清楚”：

```text
.
├── README.md
├── README_zh.md
├── go.mod
├── config.go              # package webboost: Config / DefaultConfig / Validate
├── engine.go              # package webboost: Engine / New / Start / Stop / Restore
├── status.go              # package webboost: Status / SystemChange / ProbeStatus
├── errors.go              # package webboost: sentinel / typed errors
├── options.go             # package webboost: optional functional options if needed
├── provider/              # provider model and built-in provider helpers
│   ├── provider.go
│   ├── registry.go
│   ├── steam/
│   └── github/
├── rules/                 # domain rule compile / match / rule-set metadata
├── network/
│   ├── resolver/          # system / udp / tcp / doh resolver and cache
│   └── upstream/          # direct / http / socks5 / outbound profile dialing
├── takeover/
│   ├── proxy/             # HTTP proxy / CONNECT proxy mode
│   ├── pac/               # PAC generation and PAC server
│   ├── hosts/             # hosts marker block model and host entries
│   └── dnsintercept/      # manual/system/external DNSIntercept strategy boundary
├── reverse/               # local HTTP/HTTPS reverse proxy
├── pageenhance/           # response transform pipeline and assets
├── certstore/             # root CA / dynamic cert / platform trust abstractions
├── rollback/              # versioned rollback state model
├── diagnostics/           # redacted errors, probes, support diagnostics
├── adapters/
│   ├── windows/           # Windows platform executor implementation
│   ├── externaldns/       # AdGuardHome/dnsmasq/sing-box/Clash rule export adapters
│   └── apphost/           # optional AppHost client adapter only, not service installer
├── internal/
│   ├── platform/          # OS-specific private helpers
│   ├── testutil/
│   └── certtest/
├── examples/
│   ├── basic/
│   ├── custom-provider/
│   ├── dns-manual/
│   ├── page-enhance/
│   └── hosts-windows/
└── docs/
    ├── api.md
    ├── provider.md
    ├── takeover.md
    ├── rollback.md
    └── security.md
```

### 目录约束

- 根目录只放 `package webboost` 的入口文件、README、go.mod 和项目级元数据。
- 不使用无意义的 `pkg/`、`service/`、`repository/`、`handler/` 分层。
- 不把 `resolver`、`upstream`、`hosts`、`dnsintercept`、`pageenhance` 等能力全部散在根目录。
- `adapters/` 只能承载可选集成，不能反向污染核心 API。
- `internal/` 只放外部用户不应 import 的平台细节、测试工具和私有实现，不作为公共 API 逃生口。
- `examples/` 必须可运行，默认不依赖真实外网可达。

## 初始 public API 草案

```go
package webboost

type Mode string

const (
    ModeProxyOnly Mode = "proxy_only"
    ModePAC       Mode = "pac"
    ModeSystem    Mode = "system"
    ModeHosts     Mode = "hosts"
    ModeDNS       Mode = "dns"
)

type Config struct {
    Mode        Mode
    Providers   []provider.Provider
    Rules       rules.Config
    Resolver    resolver.Config
    Upstream    upstream.Config
    Takeover    TakeoverConfig
    Reverse     ReverseConfig
    PageEnhance pageenhance.Config
    Runtime     RuntimeConfig
}

func DefaultConfig() Config

type Engine struct {
    // unexported fields
}

func New(cfg Config, opts ...Option) (*Engine, error)

func (e *Engine) Start(ctx context.Context) error
func (e *Engine) Stop(ctx context.Context) error
func (e *Engine) Restore(ctx context.Context) error
func (e *Engine) Status(ctx context.Context) (Status, error)
```

### 使用样例

```go
cfg := webboost.DefaultConfig()
cfg.Mode = webboost.ModeDNS
cfg.Providers = []provider.Provider{
    steam.Provider(),
}
cfg.PageEnhance.Enabled = true

engine, err := webboost.New(cfg)
if err != nil {
    return err
}
if err := engine.Start(ctx); err != nil {
    return err
}
defer engine.Stop(context.Background())
```

## 能力边界

### Core library

适合进入核心库：

- provider registry / rule pack model
- domain matcher
- resolver / DoH / DNS cache
- upstream / outbound profile / candidate dialing
- local HTTP proxy / CONNECT proxy
- PAC generation and PAC server
- hosts marker block model
- DNSIntercept manual server and strategy model
- reverse proxy
- Page Enhance transform pipeline
- certificate model and dynamic cert generation
- rollback state model
- diagnostics and probes

### Optional adapters

适合作为 adapter：

- Windows system proxy writer
- Windows hosts writer
- Windows system DNS writer
- Windows certificate store writer
- AppHost client executor
- external DNS tool exporter
- macOS / Linux platform writers

### Out of scope

暂不进入 `web-boost` 核心：

- GUI / desktop shell
- installer / updater / signer
- AppHost service installer
- embedded product-specific Steam one-click UX
- TUN / VPN implementation
- provider-specific account or business features

## v0.8.0 交付物

- 在当前仓库维护本规划，并让 roadmap / handoff 指向 `gofurry/web-boost`。
- 标注 `internal/` 包的迁移优先级：直接迁移、重写后迁移、只作参考、不迁移。
- 输出 public API 草案和 config schema 草案。
- 输出最小 provider 开发样例。
- 输出 Page Enhance 和 DNSIntercept 的库级 API 边界。
- 输出系统修改 adapter / rollback 约束。
