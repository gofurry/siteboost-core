# steam-go 增量建议：基于现有架构接入认证会话与免费领取能力

本文基于 `github.com/gofurry/steam-go` 当前实现整理，不再做凭空包设计。核对基准为远端 `HEAD`：`cbf93e73142c8106ea2d525f35afae902d25ccc3`。已核对的现有结构包括：

- 根包 `steam` 提供 `NewClient(opts ...Option)`。
- 根 `Client` 下已有 `client.API.*` 与 `client.Web.*` 两层入口。
- 官方 Web API 服务位于 `api/<service>/`，例如 `api/storeservice`、`api/playerservice`。
- 非 `api.steampowered.com` 的只读 Web JSON 能力位于 `web/<area>/`，例如 `web/storefront`、`web/community`、`web/market`。
- Addon 已有 `addons/openid`、`addons/a2s`，适合承载非核心或桥接型能力。
- 内部请求基础设施已有 `internal/request.Executor`、`request.RequestSpec`、`TrafficClass*`、`WithHTTPClient`、`WithCookieJar`、`WithTrafficPolicy`、`ProxySelector` 等扩展点。

因此，本次 POC 里的能力应按现有架构拆成三层：

1. **主线 `api/authenticationservice`**：接入 `IAuthenticationService` 原子接口。
2. **主线小幅增强 `web/storefront`**：补齐已有 Storefront 能力中缺少但通用的只读字段/接口。
3. **Addon `addons/websession` 与 `addons/freeclaim`**：把主线原子能力组合成登录态和免费领取链路。

## 一、主线：新增 `api/authenticationservice`

`steam-go` 主线定位是 Steam Web API SDK。`IAuthenticationService` 属于 `api.steampowered.com` 下的 Steam Web API 风格接口，适合进入主线，而不是只放在 addon。

建议新增目录：

```text
api/authenticationservice/
  service.go
  methods.go
  types.go
  methods_test.go
```

并按现有服务风格接入：

- 在 `internal/endpoint/endpoint.go` 增加 endpoint 常量。
- 在 `client.go` 中引入 package。
- 在 `type API struct` 中新增字段：

```go
AuthenticationService *authenticationservice.Service
```

- 在 `NewClient` 中初始化：

```go
AuthenticationService: authenticationservice.NewService(officialExecutor)
```

### 1. endpoint 常量

建议加入 `internal/endpoint/endpoint.go`：

```go
AuthenticationServiceGetPasswordRSAPublicKey        = "/IAuthenticationService/GetPasswordRSAPublicKey/v1/"
AuthenticationServiceBeginAuthSessionViaCredentials = "/IAuthenticationService/BeginAuthSessionViaCredentials/v1/"
AuthenticationServiceBeginAuthSessionViaQR          = "/IAuthenticationService/BeginAuthSessionViaQR/v1/"
AuthenticationServiceUpdateAuthSessionWithSteamGuardCode = "/IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1/"
AuthenticationServicePollAuthSessionStatus          = "/IAuthenticationService/PollAuthSessionStatus/v1/"
```

命名风格应和现有 `StoreServiceGetAppList`、`PlayerServiceGetOwnedGames` 保持一致。

### 2. protobuf form 编码不要做成公共大抽象

当前 `internal/request.RequestSpec` 已支持：

```go
Body        []byte
ContentType string
```

所以 `IAuthenticationService` 的：

```text
input_protobuf_encoded=<base64 protobuf>
```

可以先在 `api/authenticationservice` 内部实现小 helper，而不是新增公开 `core/protobufform` 之类不存在的包。

建议放在 `api/authenticationservice/methods.go` 或非导出文件：

```go
func buildProtoForm(encoded string) []byte {
    values := url.Values{}
    values.Set("input_protobuf_encoded", encoded)
    return []byte(values.Encode())
}
```

如果后续多个 Web API 服务都需要 protobuf form，再考虑下沉到 `internal/request` 或 `internal/protobufform`。第一步不要扩大公共 API。

### 3. protobuf 编码策略

当前 `go.mod` 依赖很少：

```text
github.com/gofurry/a2s-go
golang.org/x/time
```

所以建议优先沿用 POC 中的小型 protobuf wire 编码/解码方案，作为 `api/authenticationservice` 的内部实现，避免为了 5 个接口立刻引入较重的 protobuf 运行时依赖。

可选路线：

- 短期：手写最小 protobuf wire encoder/decoder，只覆盖本组字段。
- 中期：如果更多 Steam Web API protobuf 接口进入主线，再评估是否引入 `google.golang.org/protobuf`。

对外仍然暴露普通 Go struct，不暴露 protobuf 细节。

### 4. GetPasswordRSAPublicKey

接口：

```text
GET https://api.steampowered.com/IAuthenticationService/GetPasswordRSAPublicKey/v1/
```

请求参数：

```text
input_protobuf_encoded=<base64 protobuf>
```

protobuf 请求字段：

```text
1 account_name string
```

响应字段：

```text
1 publickey_mod string
2 publickey_exp string
3 timestamp uint64
```

建议公开类型：

```go
type GetPasswordRSAPublicKeyResponse struct {
    PublicKeyMod string
    PublicKeyExp string
    Timestamp    uint64
}
```

建议方法：

```go
func (s *Service) GetPasswordRSAPublicKey(ctx context.Context, accountName string) (GetPasswordRSAPublicKeyResponse, error)
func (s *Service) GetPasswordRSAPublicKeyRaw(ctx context.Context, accountName string) ([]byte, error)
```

是否保留 Raw 方法可按现有 `api/*` 风格决定。现有服务普遍是 typed + Raw 成对提供。

### 5. BeginAuthSessionViaCredentials

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaCredentials/v1/
```

请求字段：

```text
1  device_friendly_name string
2  account_name string
3  encrypted_password string
4  encryption_timestamp uint64
5  remember_login bool/uint64 = 1
6  platform_type uint64 = 2  // WebBrowser
7  persistence uint64 = 1    // Persistent
8  website_id string = Store
9  device_details message
11 language uint64 = 6       // schinese
```

`device_details` 字段：

```text
1 device_friendly_name string
2 platform_type uint64 = 2
```

响应字段：

```text
1 client_id uint64
2 request_id bytes
3 interval uint32
4 allowed_confirmations repeated Confirmation
5 steamid fixed64/string
6 weak_token string
```

`allowed_confirmations`：

```text
1 confirmation_type uint32
2 associated_message string
```

建议类型：

```go
type BeginAuthSessionViaCredentialsRequest struct {
    DeviceFriendlyName  string
    AccountName         string
    EncryptedPassword   string
    EncryptionTimestamp uint64
    RememberLogin       bool
    PlatformType        AuthSessionPlatformType
    Persistence         AuthSessionPersistence
    WebsiteID           string
    Language            uint64
}

type BeginAuthSessionResponse struct {
    ClientID             uint64
    RequestID            []byte
    Interval             uint32
    AllowedConfirmations []AuthConfirmation
    SteamID              string
    WeakToken            string
}
```

注意边界：

- 主线方法只接收 `EncryptedPassword`，符合原子接口定位。
- 密码 RSA 加密可以提供 helper，但不要把“账号密码登录完整流程”塞进主线方法。
- 完整“输入密码 -> 获取公钥 -> 加密 -> Begin -> Guard -> Poll”应放在 `addons/websession`。

可选 helper：

```go
func EncryptPasswordPKCS1v15(password string, publicKeyModHex string, publicKeyExpHex string) (string, error)
```

如果这个 helper 暴露在主线，应说明它只是 `IAuthenticationService` 辅助函数，不保存密码、不发起登录流程。

### 6. BeginAuthSessionViaQR

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaQR/v1/
```

请求字段：

```text
3 device_details message
```

响应字段：

```text
1 client_id uint64
2 challenge_url string
3 request_id bytes
4 interval uint32
5 allowed_confirmations repeated Confirmation
6 version uint32
```

建议方法：

```go
func (s *Service) BeginAuthSessionViaQR(ctx context.Context, deviceName string) (BeginQRAuthSessionResponse, error)
```

二维码生成、UI 展示、轮询策略不进入主线。

### 7. UpdateAuthSessionWithSteamGuardCode

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1/
```

请求字段：

```text
1 client_id uint64
2 steamid fixed64
3 code string
4 code_type uint64
```

Guard 类型：

```text
2 EmailCode
3 DeviceCode
4 DeviceConfirmation
5 EmailConfirmation
```

建议类型：

```go
type GuardCodeType uint64

const (
    GuardCodeTypeEmailCode         GuardCodeType = 2
    GuardCodeTypeDeviceCode        GuardCodeType = 3
    GuardCodeTypeDeviceConfirmation GuardCodeType = 4
    GuardCodeTypeEmailConfirmation GuardCodeType = 5
)
```

主线只提交验证码，不决定交互策略。

### 8. PollAuthSessionStatus

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/PollAuthSessionStatus/v1/
```

请求字段：

```text
1 client_id uint64
2 request_id bytes
```

响应字段：

```text
1 new_client_id uint64
2 new_challenge_url string
3 refresh_token string
4 access_token string
5 had_remote_interaction bool
6 account_name string
7 new_guard_data string
```

建议类型：

```go
type PollAuthSessionStatusResponse struct {
    NewClientID          uint64
    NewChallengeURL      string
    RefreshToken         string
    AccessToken          string
    HadRemoteInteraction bool
    AccountName          string
    NewGuardData         string
}
```

## 二、主线：基于现有 `web/storefront` 补齐只读 Store 能力

`steam-go` 已有 `client.Web.Storefront`，并已实现：

- `GetAppDetails`
- `GetPackageDetails`
- `GetAppReviews`

本次 POC 中“搜索喜加一”和“AppID 解析 SubID”都依赖 Storefront 只读 Web JSON/HTML 能力。这里应尽量复用现有 `web/storefront`，不要另起 `store/freeclaim` 作为主线包。

### 1. `GetAppDetails` 的 `PackageGroups` 可保持 Raw

当前 `web/storefront/types.go` 中：

```go
PackageGroups json.RawMessage `json:"package_groups,omitempty"`
```

这很符合现有文档里“高波动 payload 可保持 Raw”的策略。免费包解析可以先在 `addons/freeclaim` 中解析 `PackageGroups`，不必强行把 `package_groups` 全部主线类型化。

后续如果确实稳定，可以在 `web/storefront` 增加轻量类型：

```go
type StorePackageGroup struct {
    Name  string            `json:"name"`
    Title string            `json:"title"`
    Subs  []StorePackageSub `json:"subs"`
}

type StorePackageSub struct {
    PackageID                uint32 `json:"packageid"`
    PercentSavingsText       string `json:"percent_savings_text"`
    OptionText               string `json:"option_text"`
    IsFreeLicense            bool   `json:"is_free_license"`
    PriceInCentsWithDiscount int    `json:"price_in_cents_with_discount"`
}
```

但这不是第一优先级，addon 可以先解析 Raw。

### 2. 免费促销搜索不建议直接进主线稳定 API

搜索接口：

```text
GET https://store.steampowered.com/search/results/?query&start=0&count=50&dynamic_data=&force_infinite=1&specials=1&maxprice=free&os=win&snr=1_7_7_7000_7&infinite=1
```

响应：

```text
success int
results_html string
total_count int
```

这是 Store 页面搜索结果 HTML 片段，不是稳定 JSON schema。按当前 `client.Web.*` 的定位，它比 `GetAppDetails` 更脆弱。建议先放在 `addons/freeclaim`，除非后续 `web/storefront` 明确扩展“公开 Store 页面 HTML 搜索”能力。

## 三、Addon：新增 `addons/websession`

`addons/websession` 用来把主线 `client.API.AuthenticationService` 的原子接口桥接成“可用的网页登录态”。它的定位类似现有 `addons/openid`：独立、可选、面向一个明确流程，但不污染根 Client。

建议目录：

```text
addons/websession/
  client.go
  options.go
  errors.go
  credentials.go
  cookies.go
  validate.go
  client_test.go
```

### 1. 依赖主线 AuthenticationService

推荐让 addon 复用主线服务：

```go
type Client struct {
    auth *authenticationservice.Service
    httpClient *http.Client
}

func NewClient(auth *authenticationservice.Service, opts ...Option) (*Client, error)
```

调用方用法接近：

```go
sdk, err := steam.NewClient(
    steam.WithDefaultCookieJar(),
    steam.WithTrafficPolicy(steam.TrafficClassOfficialAPI, steam.TrafficPolicy{...}),
)

sessionClient, err := websession.NewClient(sdk.API.AuthenticationService)
```

如果 `RefreshTokenToWebCookies` 需要独立 HTTP client，则沿用 `addons/openid` 的 option 风格提供：

```go
websession.WithHTTPClient(...)
websession.WithTimeout(...)
websession.WithMaxResponseBodyBytes(...)
```

不要要求 addon 访问 steam-go 内部 executor。

### 2. credentials 登录编排

Addon 负责组合这些主线原子接口：

1. `GetPasswordRSAPublicKey`
2. RSA 加密密码
3. `BeginAuthSessionViaCredentials`
4. 根据 `AllowedConfirmations` 提示调用方
5. `UpdateAuthSessionWithSteamGuardCode` 或等待手机批准
6. `PollAuthSessionStatus`

推荐类型：

```go
type StartWithCredentialsRequest struct {
    AccountName string
    Password    string
    DeviceName  string
    WebsiteID   string
    Language    uint64
}

type LoginChallenge struct {
    SteamID              string
    ClientID             uint64
    RequestID            []byte
    PollInterval         time.Duration
    AllowedConfirmations []authenticationservice.AuthConfirmation
}

type LoginResult struct {
    AccountName   string
    SteamID       string
    RefreshToken  string
    AccessToken   string
}
```

建议方法：

```go
func (c *Client) StartWithCredentials(ctx context.Context, req StartWithCredentialsRequest) (*LoginChallenge, error)
func (c *Client) SubmitSteamGuardCode(ctx context.Context, challenge *LoginChallenge, code string, typ authenticationservice.GuardCodeType) error
func (c *Client) Poll(ctx context.Context, challenge *LoginChallenge) (*LoginResult, error)
```

重要经验：

- 如果 `AllowedConfirmations` 同时包含 `DeviceConfirmation` 和 `DeviceCode`，默认应优先提示手机批准并轮询。
- 手机批准后不要再提交 TOTP，否则容易触发 `EResult 29 DuplicateRequest`。
- 邮箱验证码和手机令牌不是同一链路，邮箱是否投递由 Steam 服务端控制。

### 3. refresh token 换 Web Cookie

这不是 `api.steampowered.com` 官方 Web API，适合留在 `addons/websession`。

接口：

```text
POST https://login.steampowered.com/jwt/finalizelogin
```

表单字段：

```text
nonce=<refresh token>
sessionid=<random session id>
redir=https://steamcommunity.com/login/home/?goto=
```

响应字段：

```json
{
  "error": 0,
  "transfer_info": [
    {
      "url": "https://store.steampowered.com/login/settoken",
      "params": {}
    }
  ]
}
```

Transfer 请求：

```text
POST <transfer.url>
Content-Type: application/x-www-form-urlencoded

steamID=<jwt sub>
<transfer.params 原样展开>
```

推荐结果：

```go
type WebCookieResult struct {
    Jar       http.CookieJar
    SessionID string
    SteamID   string
    Domains   []string
}
```

注意：

- `finalizelogin` 与 transfer 必须复用同一个 CookieJar。
- `sessionid` 是 Store 表单 session token，不是 refresh token，也不是 `steamLoginSecure`。
- 可复用 steam-go 现有 `WithCookieJar` / `WithDefaultCookieJar` 思路，但 addon 不应保存 refresh token。

### 4. session 校验

Community：

```text
GET https://steamcommunity.com/profiles/{steamID}/?xml=1
```

Store：

```text
GET https://store.steampowered.com/account/?l=english
```

经验：

- 不要只测 Community，免费领取依赖 Store Cookie。
- `/my/?xml=1` 容易重定向，`/profiles/{steamID}/?xml=1` 更稳定。
- `/account/licenses/` 可能跳登录，不能作为唯一 Store session 判定。

## 四、Addon：新增 `addons/freeclaim`

“领取免费 License”与业务链路耦合更强，建议作为独立 addon，而不是进入 `client.API.*` 或 `client.Web.*` 主线。

建议目录：

```text
addons/freeclaim/
  client.go
  options.go
  errors.go
  search.go
  package.go
  claim.go
  ownership.go
```

### 1. 依赖方式

不要让 `addons/freeclaim` 管 refresh token 或账号密码。它只需要一个能提供 Store Web Cookie 的依赖。

建议小接口：

```go
type CookieProvider interface {
    WebCookies(ctx context.Context) (*websession.WebCookieResult, error)
}
```

或者更低耦合：

```go
type CookieJarProvider interface {
    CookieJar(ctx context.Context) (http.CookieJar, error)
}
```

这样 `addons/freeclaim` 可以依赖 `addons/websession`，也可以让调用方自己提供 CookieJar。

### 2. 搜索限时免费游戏

接口：

```text
GET https://store.steampowered.com/search/results/?query&start=0&count=50&dynamic_data=&force_infinite=1&specials=1&maxprice=free&os=win&snr=1_7_7_7000_7&infinite=1
```

响应字段：

```text
success int
results_html string
total_count int
```

HTML 解析规则：

```text
a.search_result_row
data-ds-appid
.discount_block data-discount == 100
.discount_block data-price-final == 0
.discount_original_price 非空
```

推荐类型：

```go
type FreePromotion struct {
    AppID         uint32
    Title         string
    StoreURL      string
    CapsuleURL    string
    OriginalPrice string
    FinalPrice    string
    Discount      string
}
```

### 3. AppID 解析 SubID

优先复用主线已有：

```go
client.Web.Storefront.GetAppDetails(ctx, appID, &storefront.GetAppDetailsOptions{
    CountryCode: "us",
    Language: "english",
})
```

当前 `AppDetailsData.PackageGroups` 是 `json.RawMessage`，`addons/freeclaim` 可在本地解析：

```text
package_groups[].subs[].packageid
package_groups[].subs[].is_free_license
package_groups[].subs[].price_in_cents_with_discount
package_groups[].subs[].percent_savings_text
package_groups[].subs[].option_text
```

筛选规则：

```text
packageid > 0
price_in_cents_with_discount == 0
is_free_license == true 或 percent_savings_text 含 100 或 option_text 含 free
```

推荐类型：

```go
type FreePackage struct {
    AppID      uint32
    PackageID  uint32
    Title      string
    OptionText string
}
```

### 4. 领取免费 License

优先接口：

```text
POST https://store.steampowered.com/checkout/addfreelicense
```

备用接口：

```text
POST https://store.steampowered.com/checkout/addfreelicense/
POST https://store.steampowered.com/freelicense/addfreelicense/
```

请求头：

```text
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
Origin: https://store.steampowered.com
Referer: https://store.steampowered.com/app/{appid}/
X-Requested-With: XMLHttpRequest
Cookie: steamLoginSecure=...; sessionid=...
```

表单字段：

```text
action=add_to_cart
sessionid=<store sessionid>
subid=<package id>
snr=<可选，从 app 页面 form 解析>
originating_snr=<可选，从 app 页面 form 解析>
```

领取前建议 GET：

```text
https://store.steampowered.com/app/{appid}/
```

解析：

```html
form[name=add_to_cart_{subid}]
```

并复用隐藏字段。这比硬编码表单更贴近 Steam 当前网页行为。

### 5. 成功判定

推荐顺序：

1. 领取响应 HTML 命中成功页。
2. 响应文本命中已拥有。
3. `dynamicstore/userdata` 中 `rgOwnedApps` 包含 AppID。

成功页文本：

```text
Success!
成功！
已被绑定至您的 Steam 帐户
已被绑定至您的 Steam 账户
```

已拥有文本：

```text
already own
already have
已拥有
已经拥有
```

ownership 校验接口：

```text
GET https://store.steampowered.com/dynamicstore/userdata/?t={timestamp}
```

关键字段：

```text
rgOwnedApps []uint32
```

本轮验证经验：

- `/account/licenses/` 可能重定向到登录，即使 `/account/` 与领取接口可用。
- 因此不要把 `/account/licenses/` 作为唯一成功判定。

## 五、错误模型增量

当前 steam-go 已有公开：

```go
type APIError = sdkerrors.APIError
type ErrorKind = sdkerrors.Kind
```

因此不建议新增完全独立的错误体系。可以做最小增量：

1. 在 `api/authenticationservice` 内部把 Steam 返回的 EResult 转成现有 `ErrorKindAPIResponse`。
2. 如需调用方识别 EResult，再新增一个可 `errors.As` 的公开类型。

建议类型：

```go
type EResultError struct {
    Code    int
    Name    string
    Message string
}
```

重点值：

```text
29 DuplicateRequest
63 AccountLogonDenied
65 InvalidLoginAuthCode
71 ExpiredLoginAuthCode
84 RateLimitExceeded
```

Addon 可根据 `EResultError` 做用户提示，但不应吞掉原始错误。

## 六、网络与流量策略

不要新增一套代理系统。steam-go 已经有：

- `WithHTTPClient`
- `WithCookieJar`
- `WithDefaultCookieJar`
- `WithProxySelector`
- `NewStaticProxySelector`
- `NewRoutingProxySelector`
- `NewStickyProxySelector`
- `WithTrafficPolicy`
- `TrafficClassOfficialAPI`
- `TrafficClassPublicStorePage`
- `TrafficClassCommunityWeb`

本次能力建议复用：

- `IAuthenticationService`：`TrafficClassOfficialAPI`
- Store 页面、appdetails、search、dynamicstore：`TrafficClassPublicStorePage`
- Community XML 校验：`TrafficClassCommunityWeb`
- 同一账号登录/领取流程可通过 `WithRequestSessionKey(ctx, key)` 或 `WithProxySessionKey(ctx, key)` 保持粘性策略。

国内网络代理场景示例应沿用 README 里的方式：

```go
selector, err := steam.NewStaticProxySelector("http://127.0.0.1:7897")
client, err := steam.NewClient(
    steam.WithProxySelector(selector),
    steam.WithDefaultCookieJar(),
)
```

## 七、推荐落地顺序

### 阶段 1：主线 `api/authenticationservice`

最小可验收：

- 新增 service/types/methods。
- 接入 `client.API.AuthenticationService`。
- 覆盖 5 个原子接口。
- 单元测试覆盖请求构造、protobuf form、字段编码、错误映射。
- 不实现完整登录流程。

### 阶段 2：`addons/websession`

最小可验收：

- 基于 `client.API.AuthenticationService` 完成 credentials 登录编排。
- 支持手机批准、手机令牌、邮箱验证码三类路径。
- 支持 refresh token -> Web Cookie。
- 支持 Store 与 Community session 校验。
- 不保存 refresh token。
- 不读取浏览器 Cookie 或 Steam 客户端本地 token。

### 阶段 3：`addons/freeclaim`

最小可验收：

- 搜索限免候选。
- 复用 `client.Web.Storefront.GetAppDetails` 解析 package/subid。
- 单次点击领取一个 package。
- 支持成功、已拥有、登录失效、疑似限流的结果区分。
- 不提供全部领取接口。

## 八、不进入 steam-go 的内容

以下内容留给 SteamScope 或其他业务应用：

- Wails 绑定
- Vue 状态
- DPAPI / Keychain / Credential Manager 具体存储
- 本地账号数据库
- 本地领取记录
- “今日喜加一”页面展示
- 自动全部领取
- 无限重试
- 浏览器 Cookie 读取
- Steam 客户端本地登录态读取
