# 能力边界冻结

> 适用阶段：`v0.6.5`
> 目的：在进入 `v0.7.0` Provider 架构重构前，明确哪些能力属于核心抽象，哪些只是未来扩展点，避免把 `v1.1+` 的高级能力提前塞进当前库边界。

## 结论

`v0.7.0` 之前不实现 GitHub 真实加速、DNSIntercept、TUN/VPN、JS 注入或跨平台 AppHost 等高级能力。它们只作为 future extension points 和 non-goals 记录。

当前应冻结的是能力边界，而不是提前扩展能力面：

- Provider 描述站点，不修改系统。
- Takeover mode 负责接管方式，不携带站点私有逻辑。
- AppHost 执行白名单系统修改，不暴露任意 shell 或 provider 私有命令。
- Certificate / Root CA 属于 runtime/takeover 层，不属于 Provider。
- Resolver / upstream 只消费通用 rule/profile，不依赖 Steam 语义。
- Diagnostics 负责解释状态和失败层级，不负责改变路由。

## Provider 边界

Provider 是站点定义单元。它可以提供：

- `id`、`name`、`status`：例如 `stable`、`skeleton`、`experimental`。
- rule pack：域名、路径或分组规则。
- outbound profile：ForwardHost、TLS SNI、候选目标、直连或可选 upstream 策略。
- hosts exact list：Hosts 模式能写入的精确域名清单。
- startup probes 和 smoke targets。
- 文档元数据：能力说明、已知限制、测试记录链接。

Provider 不允许：

- 写 hosts 文件。
- 安装或卸载 Root CA。
- 调用 AppHost 或执行系统提权操作。
- 创建 DNSIntercept、TUN/VPN、系统代理或 PAC。
- 执行 shell、任意文件写入、任意路径删除或敏感凭据读取。
- 把某个站点的特殊逻辑写进 reverse / resolver / upstream 核心包。

## Takeover Mode 边界

Takeover mode 描述“如何接管流量”，不是“接管哪个站点”。

当前已存在或已验证的模式：

- `proxy_only`：本地 HTTP proxy 与 HTTPS CONNECT。
- `pac`：PAC server 与 PAC 文件。
- `system_proxy`：写入当前用户系统代理设置。
- `hosts`：写入 exact hosts，本地 HTTP / HTTPS reverse proxy，Root CA 和动态站点证书。

未来扩展点：

- `dns_intercept`：用于覆盖 hosts 无法表达的 wildcard 和非浏览器 DNS 路径。
- `tun` / `vpn`：用于更强流量接管。

`dns_intercept` 和 `tun` 在 `v0.7.0` 前只保留名字和边界，不实现路由、驱动、虚拟网卡、DNS 劫持或系统网络栈修改。

## Privilege / AppHost 边界

AppHost 是平台权限执行器，不是 Provider 能力。

AppHost 可以执行的请求必须满足：

- 请求在白名单命令面内，例如 Root CA、hosts 写入、restore。
- 请求来自同一二进制路径的受控客户端。
- 请求不包含任意 shell、任意路径写入或敏感凭据。
- 失败时返回可诊断错误，指导用户 install、start、restore 或 uninstall。

AppHost 不承担：

- 站点规则选择。
- provider 私有命令。
- TUN/VPN 驱动管理。
- 绕过 UAC。
- 长期持有用户敏感配置。

## Certificate 边界

证书能力属于 runtime/takeover 层：

- Root CA 生成、安装、查询和卸载由 certstore / privilege 负责。
- 动态站点证书由 reverse / certificate runtime 负责。
- Provider 只声明哪些 host 需要 HTTPS 接管，不直接签发证书。
- Root CA 默认行为、信任范围和卸载路径必须在文档里保持可见。

## Resolver / Upstream 边界

Resolver 和 upstream 是通用网络能力：

- Resolver 支持 system / udp / tcp / doh 和后续缓存、fallback、IPv4/IPv6 策略。
- Upstream 支持 direct、可选 HTTP / SOCKS5 upstream、provider outbound profile 和 candidate dialing。
- Provider 可以提供 ForwardHost、TLS SNI 和候选目标，但不应该让核心包出现 Steam 或 GitHub 专用判断。
- HTTP / SOCKS5 upstream 是增强能力，不是默认闭环的前提。

## Runtime / Status / Diagnostics 边界

未来库和当前 CLI 都应能表达这些状态：

- `unsupported`：当前平台或模式不支持。
- `experimental`：能力存在但未承诺稳定。
- `requires_admin_init`：需要一次管理员初始化。
- `running`：运行中。
- `restored`：系统修改已恢复。
- `partial_failure`：部分能力失败，但可给出明确层级和恢复建议。

Diagnostics 应解释失败发生在哪一层：

- AppHost 未安装、未运行或健康检查失败。
- Root CA 检查或安装失败。
- hosts preflight 或写入失败。
- listener 端口占用。
- resolver / DoH 失败。
- TCP / TLS / HTTP smoke 失败。

Diagnostics 不应自动尝试高风险系统修改，除非用户显式执行对应命令或启动流程。

## v0.7.0 前置规则

进入 Provider 架构重构前，必须遵守：

- Steam 是唯一 stable provider。
- GitHub 只能作为 skeleton / experimental provider，不承诺真实加速。
- Provider 接口必须能表达状态和限制。
- Provider 接口必须禁止系统修改职责。
- `reverse`、`resolver`、`upstream` 应只依赖通用 matcher/profile。
- 任何高级接管能力只以 future mode 形式出现，不进入默认行为。

## Deferred 到 v1.1+

以下内容不进入 `v0.7.0`，除非先有新的安全设计和单独 smoke：

- GitHub provider 真实网络验证。
- DNSIntercept。
- VPN / TUN。
- JS 注入 / 页面增强。
- macOS / Linux Hosts、证书和权限闭环。
- AppHost 用户会话绑定和审计日志的完整产品化实现。

## v0.7 后计划修订

Provider registry 落地后，DNSIntercept 和页面增强允许前移到开源库抽取之前验证，但必须遵守可还原设计：

- DNSIntercept 先从 `manual` 策略开始，不修改系统 DNS。
- 任何 `system` DNS 接管都必须显式启用、通过 AppHost、先 preflight、写 rollback，并能通过 `stop` / `restore` 还原。
- 页面增强是透明的 reverse-proxy transform pipeline，默认关闭，不内置隐藏安全跳过。
- Provider 可以声明 metadata 或推荐 enhancement pack，但 Provider 仍不执行 DNS 接管、hosts 写入、证书修改、AppHost 调用或响应改写。
- VPN / TUN 继续延期；如果未来需要，优先使用成熟外部库或独立集成。

## v0.6.5 验收

- 文档明确 v1.1+ 高级能力不会提前进入 v0.7。
- Provider 的允许职责和禁止职责清晰。
- Takeover mode 与 Provider 分离。
- AppHost 被定义为平台权限执行器，而不是站点 provider 的一部分。
- Steam 当前 Windows Hosts + DoH + AppHost smoke 作为 v0.7 重构回归基线。
