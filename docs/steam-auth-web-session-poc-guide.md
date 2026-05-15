# steam-auth-web-session-poc 验证经验文档

本文记录 `experimental/steam-auth-web-session-poc` 中已经验证通过的最小闭环：个人账号登录、refresh token 本地保存、换取 Web Cookie、搜索 Steam 限时免费游戏、解析可领取 SubID、用户手动点击领取入库。

该 POC 面向本地自用桌面工具，不做批量机器人，不读取浏览器 Cookie，不读取 Steam 客户端本地登录态。Vue 前端只负责触发用户动作，敏感 token、Cookie、sessionid 均只存在 Go 后端。

## 已验证闭环

核心链路如下：

1. Wails2 + Vue-TS 前端发起登录。
2. Go 后端通过 `IAuthenticationService` 获取 refresh token。
3. refresh token 使用 Windows DPAPI 加密保存。
4. 用户点击测试或领取时，后端用 refresh token 调 `jwt/finalizelogin` 换 Web Cookie。
5. 通过 Steam Store search 找到 `-100%`、`price-final=0` 的候选游戏。
6. 通过 `appdetails` 从 AppID 解析可领取的 package/subid。
7. 用户手动点击一个条目，后端提交领取请求。
8. Steam 返回已拥有或成功页后，本地标记为已入库。

本轮实际验证结果：`Terrors to Unveil - Day Off` 返回 “Steam 返回已拥有，已标记为已入库”，功能闭环通过。

## 登录接口

登录流程参考 `DoctorMcKay/node-steam-session` 的思路，但 Go 侧直接复刻 protobuf 请求。

### 获取 RSA 公钥

接口：

```text
GET https://api.steampowered.com/IAuthenticationService/GetPasswordRSAPublicKey/v1/
```

请求参数：

```text
input_protobuf_encoded=<base64 protobuf>
```

protobuf 字段：

```text
1 account_name string
```

响应字段：

```text
1 publickey_mod string
2 publickey_exp string
3 timestamp uint64
```

注意：密码需要用 RSA PKCS#1 v1.5 加密后 base64 编码，再传给 credentials 登录接口。

### 账号密码开始登录

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaCredentials/v1/
```

请求体为 multipart，其中字段：

```text
input_protobuf_encoded=<base64 protobuf>
```

已验证请求 protobuf 字段：

```text
1  device_friendly_name string
2  account_name string
3  encrypted_password string
4  encryption_timestamp uint64
5  remember_login bool/uint64 = 1
6  platform_type uint64 = 2 WebBrowser
7  persistence uint64 = 1 Persistent
8  website_id string = Store
9  device_details message
11 language uint64 = 6 schinese
```

`device_details` 字段：

```text
1 device_friendly_name string
2 platform_type uint64 = 2 WebBrowser
```

响应字段：

```text
1 client_id uint64
2 request_id bytes
3 interval float32
4 allowed_confirmations repeated
5 steamid uint64/string
6 weak_token string
```

`allowed_confirmations` 字段：

```text
1 confirmation_type uint64
2 associated_message string
```

已观察到的 Guard 类型：

```text
2 email_code
3 device_code
4 device_confirmation
5 email_confirmation
```

重要经验：

- `device_confirmation` 优先级必须高于 `device_code`。如果 Steam 同时返回手机批准和手机令牌，用户在手机上点批准后，桌面端只应继续轮询，不应再提示输入令牌。
- 如果在手机批准后又提交 TOTP，Steam 可能返回 `EResult 29 DuplicateRequest`。
- 邮箱验证码和手机令牌不是同一种链路。手机令牌是本地 TOTP，邮箱验证码依赖 Steam 服务端投递，可能受风控、出口 IP、代理、频繁登录影响。

### 提交 Steam Guard 验证码

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1/
```

请求 protobuf 字段：

```text
1 client_id uint64
2 steamid fixed64
3 code string
4 code_type uint64
```

只应在当前状态确认为 `email_code` 或 `device_code` 时调用。

### 轮询登录结果

接口：

```text
POST https://api.steampowered.com/IAuthenticationService/PollAuthSessionStatus/v1/
```

请求 protobuf 字段：

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

拿到 `refresh_token` 后登录完成。POC 使用 Windows DPAPI 加密保存 refresh token，前端不接触该值。

## refresh token 换 Web Cookie

接口：

```text
POST https://login.steampowered.com/jwt/finalizelogin
```

表单字段：

```text
nonce=<refresh_token>
sessionid=<随机 session id>
redir=https://steamcommunity.com/login/home/?goto=
```

响应 JSON：

```json
{
  "error": 0,
  "transfer_info": [
    {
      "url": "https://store.steampowered.com/login/settoken",
      "params": {
        "...": "..."
      }
    }
  ]
}
```

后续需要对每个 `transfer_info` 做 POST：

```text
POST <transfer.url>
Content-Type: application/x-www-form-urlencoded
```

表单字段：

```text
steamID=<refresh token JWT sub>
<transfer.params 原样展开>
```

关键经验：

- `transfer_info` 请求用 `application/x-www-form-urlencoded` 更贴近网页行为，multipart 可能出现兼容问题。
- 需要同一个 `cookiejar.Jar` 串起所有 transfer 请求。
- 手动给 `steamcommunity.com`、`store.steampowered.com`、`help.steampowered.com` 设置同一个 `sessionid`，作为后续 Store POST 的 CSRF/session token。
- `steamLoginSecure` 是登录态 Cookie；`sessionid` 是 Store 表单请求需要带的 session/CSRF token。

## Web Session 验证

只验证 Community 不够。领取走 Store 域名，所以 POC 中需要分别验证：

```text
Community: https://steamcommunity.com/profiles/{steamID}/?xml=1
Store:     https://store.steampowered.com/account/?l=english
```

经验：

- `/my/?xml=1` 容易触发多次重定向，改用 `/profiles/{steamID}/?xml=1`。
- Store `/account/` 可验证 Store 登录态，但 `/account/licenses/` 在部分会话中会重定向到登录，不适合作为唯一成功判定。

## 喜加一发现

搜索接口：

```text
GET https://store.steampowered.com/search/results/?query&start=0&count=50&dynamic_data=&force_infinite=1&specials=1&maxprice=free&os=win&snr=1_7_7_7000_7&infinite=1
```

响应 JSON 字段：

```text
success int
results_html string
total_count int
```

解析 `results_html` 中的：

```text
a.search_result_row
data-ds-appid
.discount_block[data-discount=100][data-price-final=0]
.discount_original_price
.discount_final_price
.title
.search_capsule img[src]
```

筛选规则：

```text
data-discount == 100
data-price-final == 0
original price 非空
```

注意：搜索只能拿到 AppID，真正领取需要 SubID/PackageID。

## AppID 解析 SubID

接口：

```text
GET https://store.steampowered.com/api/appdetails?appids={appid}&cc=us&l=english
```

关注字段：

```json
{
  "{appid}": {
    "success": true,
    "data": {
      "package_groups": [
        {
          "title": "...",
          "subs": [
            {
              "packageid": 1647244,
              "percent_savings_text": "-100%",
              "option_text": "... Free",
              "is_free_license": true,
              "price_in_cents_with_discount": 0
            }
          ]
        }
      ]
    }
  }
}
```

筛选规则：

```text
packageid > 0
price_in_cents_with_discount == 0
is_free_license == true 或 percent_savings_text 包含 100 或 option_text 包含 free
```

经验：

- Steam Web Store 领取促销免费包使用 SubID/PackageID，不是 AppID。
- AppID 用于商店页和 dynamicstore ownership 校验。
- SubID 用于 `addfreelicense` 请求。

## 领取接口

最终验证可行的核心请求形态：

```text
POST https://store.steampowered.com/checkout/addfreelicense
Host: store.steampowered.com
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
Origin: https://store.steampowered.com
Referer: https://store.steampowered.com/app/{appid}/
X-Requested-With: XMLHttpRequest
Cookie: steamLoginSecure=...; sessionid=...
```

表单字段：

```text
action=add_to_cart
sessionid=<store cookie jar 中的 sessionid>
subid=<free package id>
snr=<从当前 app 页面表单解析到则带上>
originating_snr=<从当前 app 页面表单解析到则带上>
```

POC 当前做法：

1. 先用登录态 GET 当前 app 页面。
2. 解析 `form[name=add_to_cart_{subid}]`。
3. 取其中隐藏字段：`snr`、`originating_snr`、`action`、`sessionid`、`subid`。
4. 优先 POST `/checkout/addfreelicense`。
5. 再试 `/checkout/addfreelicense/`。
6. 最后 fallback 到页面表单 action：`/freelicense/addfreelicense/`。

页面表单示例：

```html
<form name="add_to_cart_1647244"
      action="https://store.steampowered.com/freelicense/addfreelicense/"
      method="POST">
  <input type="hidden" name="snr" value="1_5_9__403">
  <input type="hidden" name="originating_snr" value="">
  <input type="hidden" name="action" value="add_to_cart">
  <input type="hidden" name="sessionid" value="...">
  <input type="hidden" name="subid" value="1647244">
</form>
```

## 成功判定

不要只依赖 `/account/licenses/`。本轮测试中它可能重定向登录，即使 Store `/account/` 已经验证通过。

推荐判定顺序：

1. 如果领取响应 HTML 命中成功页，直接判成功。
2. 如果响应文本包含已拥有，判 already owned。
3. 否则请求 `dynamicstore/userdata` 查询 AppID 是否在 `rgOwnedApps` 中。
4. 都失败则标记 failed，不自动无限重试。

成功页关键文本：

```text
Success!
成功！
已被绑定至您的 Steam 帐户
已被绑定至您的 Steam 账户
```

已拥有关键文本：

```text
already own
already have
已拥有
已经拥有
```

dynamicstore 校验接口：

```text
GET https://store.steampowered.com/dynamicstore/userdata/?t={timestamp}
```

关注字段：

```json
{
  "rgOwnedApps": [3587490]
}
```

经验：

- `dynamicstore/userdata` 可能有短暂缓存，领取后建议做 3-4 次短重试。
- `rgOwnedApps` 是 AppID 列表，不是 SubID 列表。
- `/account/licenses/` 更偏 SubID/License 历史，但不能作为唯一依据。

## 网络代理

国内网络下 Steam 登录、Store、login/settoken 可能需要代理。POC 提供运行时网络代理配置：

```text
127.0.0.1:7897
http://127.0.0.1:7897
socks5://127.0.0.1:7897
```

实现要点：

- 所有 Steam HTTP 请求必须共用同一个可配置 `http.Client`。
- 搜索、appdetails、登录、finalizelogin、transfer、领取、dynamicstore 都应走同一路 client。
- 留空时使用 `http.ProxyFromEnvironment`，显式填写时使用 `http.ProxyURL`。

## 已踩坑总结

- `device_confirmation` 和 `device_code` 不能同时引导用户操作，必须优先手机批准。
- 手机批准后再提交 TOTP 会触发 `EResult 29 DuplicateRequest`。
- 邮箱验证码不等于手机令牌，邮件是否投递受 Steam 服务端控制。
- 只测 Community 登录态会误判，Store 领取必须测 Store 登录态。
- `/my/?xml=1` 容易重定向，使用 `/profiles/{steamID}/?xml=1` 更稳定。
- `/account/licenses/` 可能跳登录，不应作为唯一领取成功判定。
- 促销免费领取用 SubID，不是 AppID。
- `checkout/addfreelicense/{subid}` 和 `freelicense/addfreelicense/` 都存在历史/页面证据，但当前 POC 验证更可靠的是优先 `checkout/addfreelicense`，并保留 fallback。
- `sessionid` 不是 refresh token，也不是 `steamLoginSecure`，它是 Store 表单请求必须携带的 session token。

## 安全和产品边界

该能力应保持手动、单个领取：

- 前端不展示 Cookie、refresh token、access token。
- 后端只在用户点击时临时换 Web Cookie。
- 一次点击只领取一个 SubID。
- 不做全部领取。
- 不做自动无限重试。
- rate limit 后停止，并提示用户稍后再试。
- 建议加 10-30 秒手动冷却。
