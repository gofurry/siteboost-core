# steamscope

steamscope 是一个本地自用的 Steam 桌面工具箱探索项目，目标是把常用的 Steam 账号、商店、社区与个人工作流能力整理成可验证、可维护、可逐步产品化的桌面工具。

项目当前仍处于早期探索阶段。核心验证代码会优先放在 `experimental/` 下，以独立 POC 的形式验证单条能力链路；验证通过后，再决定是否沉淀到主应用或伴生工具库。

## 当前探索

- `experimental/steam-free-claim-poc`：验证 Steam Store 限时免费游戏搜索、AppID 到 SubID 解析等只读链路。
- `experimental/steam-auth-web-session-poc`：验证个人账号登录、Steam Guard、refresh token 换 Web Cookie、Store/Community session 校验，以及用户手动点击领取免费 License 的最小闭环。
- `docs/`：记录 POC 的关键接口、字段、踩坑经验和后续开发指导。
- `steam-go/`：记录可回流到伴生项目 `github.com/gofurry/steam-go` 的路线建议。

## 项目边界

steamscope 面向本地个人使用，不读取浏览器 Cookie，不读取 Steam 客户端本地登录态，不做无人值守批量机器人。

涉及账号登录态的能力应遵守以下原则：

- 敏感 token 和 Cookie 不返回给前端展示。
- 用户动作由桌面端明确触发。
- 免费 License 领取保持一次点击一个条目。
- 不做无限重试。
- 遇到限流或登录态失效时停止并提示用户。

## 开发

每个实验目录都是相对独立的 Go / Wails / Vue-TS 验证项目。进入对应目录后按该目录 README 操作。

常见本地文件已在根目录 `.gitignore` 中忽略，包括 `node_modules`、Wails `build/bin`、前端 `dist`、日志、环境变量和本地 session/token/cookie 文件。

## 许可证

本项目 Copyright (C) 2026 福狼，使用 GNU General Public License v3.0。详见 [LICENSE](LICENSE)。
