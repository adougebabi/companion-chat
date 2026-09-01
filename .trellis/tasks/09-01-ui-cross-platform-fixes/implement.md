# Implementation plan

## Completion note

All planned implementation and validation steps completed. Web typecheck/build/tests and Core/BFF Go tests pass; the remaining browser visual check was completed against the production build at desktop and 390px mobile widths with no horizontal overflow.

1. [ ] 读取 frontend/backend Trellis specs，确认 Vue/Pinia、Go 和测试约定；记录受影响文件。
2. [ ] 新增共享展示工具：中文标签、枚举映射、对象/数组安全格式化、时区时间格式化。
3. [ ] 扩展 `InstanceDetailsDialog.vue`：迁移身份与人格、生活世界、关系与记忆只读展示；加入按 schedule timezone 的时间轴和空态；调整手机/PC 样式。
4. [ ] 精简并修正 `GovernanceView.vue`：移除重复只读块，保留治理操作；所有动态值使用共享 formatter/label。
5. [ ] 修复 `SettingsView.vue` section 切换的 Accordion 生命周期/受控状态，补充切换回归断言。
6. [ ] 修复 actor group contract：Go Core 返回 `actor_ids`；Web store/API 兼容 `members` 并保证筛选/加入分组安全。
7. [ ] 新增或更新 Web/Go 回归测试，覆盖 formatter、timezone、settings section、actor group payload 和移动端筛选。
8. [ ] 运行 `node --test apps/web/test/*.test.mjs`、相关 Go 测试、前端类型/构建检查；按失败结果迭代。
9. [ ] 运行 Trellis quality check，确认文档/契约同步，准备提交。

## Validation commands

- `node --test apps/web/test/*.test.mjs`
- `go test ./apps/core-go/...`（若仓库 Go workspace 约定不同，以实际脚本为准）
- `pnpm --filter @fluctlight/web typecheck` / 项目已有的 web build 命令

## Risky files and rollback points

- 高风险：`InstanceDetailsDialog.vue`、`GovernanceView.vue`、`apps/web/src/styles/app.css`（跨端布局）。
- 契约风险：`apps/core-go/internal/core/operations.go`、`apps/web/src/stores/control-center.ts`、`packages/browser-client/src/index.ts`。
- 每个步骤保持独立可回滚；若 Core/API 契约测试失败，先恢复正式字段改动并保留前端双字段归一化。
