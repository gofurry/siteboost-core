# 热修复流程

## 热修复工作流

1. 从最新稳定分支创建 hotfix 分支。
2. 用最小测试或命令复现问题。
3. 做最小必要修复。
4. 运行冒烟测试。
5. 如果影响用户行为，更新 CHANGELOG 或 release notes。
6. 提交 pull request，并说明回滚风险。

## 分支命名

使用简短名称：

```text
hotfix/v0.1.1-connect-timeout
hotfix/v0.2.1-doh-fallback
```

## 补丁版本

以下场景使用 patch 版本：

- bug 修复；
- 小型兼容性修复；
- 日志或文档修正；
- CI 调整；
- 低风险测试覆盖补充。

示例：

```text
v0.1.0 -> v0.1.1
v1.0.0 -> v1.0.1
```

## 发布说明

发布说明应包含：

- 用户可见影响；
- 受影响模式；
- 修复摘要；
- 验证命令；
- 回滚说明。

## 回滚说明

如果 hotfix 触及系统代理、PAC、hosts、证书或 restore 行为，必须记录：

- 修改了什么系统状态；
- rollback 如何记录；
- `steam-accelerator restore` 如何恢复；
- restore 失败时用户可以如何手动处理。
