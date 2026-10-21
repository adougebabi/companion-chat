# 本轮提交清单（用户已批准）

拟采用一个完整工作提交，避免拆开共享组件和调用页造成中间版本缺依赖：

`feat(web): polish v3 sitewide UX and responsive continuity`

包括下列 Web 源码、回归测试及交互规范，以及本任务目录内的 PRD、设计、检查和证据记录。2026-10-21 用户明确授权“提交并推送”；推送当前分支到 origin。不包含 NAS 发布、数据迁移或卷操作。

- `.trellis/spec/frontend/product-ui-contract.md`
- `apps/web/src/styles/v3-proto.css`
- `apps/web/src/v3/domain-presentation.mjs`
- `apps/web/src/v3/presentation-time.mjs`
- `apps/web/src/v3/proto/app.js`
- `apps/web/src/v3/proto/components/avatar.js`
- `apps/web/src/v3/proto/components/business-backups.js`
- `apps/web/src/v3/proto/components/cards.js`
- `apps/web/src/v3/proto/components/character-card.js`
- `apps/web/src/v3/proto/components/chat.js`
- `apps/web/src/v3/proto/components/configuration-editor.js`
- `apps/web/src/v3/proto/components/configuration-fields.js`
- `apps/web/src/v3/proto/components/cursor-pager.js`
- `apps/web/src/v3/proto/components/diagnostic-card.js`
- `apps/web/src/v3/proto/components/display-avatar.js`
- `apps/web/src/v3/proto/components/forms.js`
- `apps/web/src/v3/proto/components/goal-records.js`
- `apps/web/src/v3/proto/components/governance-records.js`
- `apps/web/src/v3/proto/components/initialization-progress.js`
- `apps/web/src/v3/proto/components/instant-label.js`
- `apps/web/src/v3/proto/components/media-assets.js`
- `apps/web/src/v3/proto/components/overlay.js`
- `apps/web/src/v3/proto/components/pending-groups.js`
- `apps/web/src/v3/proto/components/place-defaults.js`
- `apps/web/src/v3/proto/components/profile.js`
- `apps/web/src/v3/proto/components/provider-editor.js`
- `apps/web/src/v3/proto/components/routine.js`
- `apps/web/src/v3/proto/components/runtime-catalog.js`
- `apps/web/src/v3/proto/components/schedule-records.js`
- `apps/web/src/v3/proto/components/studio-jobs.js`
- `apps/web/src/v3/proto/components/toast.js`
- `apps/web/src/v3/proto/components/visual-identity-records.js`
- `apps/web/src/v3/proto/mock/chat-page.js`
- `apps/web/src/v3/proto/modules/registry.js`
- `apps/web/src/v3/proto/modules/创作治理.js`
- `apps/web/src/v3/proto/modules/相处体验.js`
- `apps/web/src/v3/proto/pages/companion.js`
- `apps/web/src/v3/proto/pages/govern.js`
- `apps/web/src/v3/proto/pages/npc-management.js`
- `apps/web/src/v3/proto/pages/profile.js`
- `apps/web/src/v3/proto/pages/provider-page.js`
- `apps/web/src/v3/proto/pages/real-avatar.js`
- `apps/web/src/v3/proto/pages/real-backups.js`
- `apps/web/src/v3/proto/pages/real-chat.js`
- `apps/web/src/v3/proto/pages/real-configuration.js`
- `apps/web/src/v3/proto/pages/real-core.js`
- `apps/web/src/v3/proto/pages/real-data-tools.js`
- `apps/web/src/v3/proto/pages/real-diagnostics.js`
- `apps/web/src/v3/proto/pages/real-diary.js`
- `apps/web/src/v3/proto/pages/real-generate.js`
- `apps/web/src/v3/proto/pages/real-goal-editor.js`
- `apps/web/src/v3/proto/pages/real-governance.js`
- `apps/web/src/v3/proto/pages/real-history.js`
- `apps/web/src/v3/proto/pages/real-life-goals.js`
- `apps/web/src/v3/proto/pages/real-life-now.js`
- `apps/web/src/v3/proto/pages/real-life-schedule.js`
- `apps/web/src/v3/proto/pages/real-life-wardrobe.js`
- `apps/web/src/v3/proto/pages/real-media.js`
- `apps/web/src/v3/proto/pages/real-operations.js`
- `apps/web/src/v3/proto/pages/real-reflection.js`
- `apps/web/src/v3/proto/pages/real-studio.js`
- `apps/web/src/v3/proto/pages/real-workflow.js`
- `apps/web/src/v3/proto/pages/world-management.js`
- `apps/web/src/v3/proto/pages/world.js`
- `apps/web/src/v3/runtime-catalog.mjs`
- `apps/web/src/v3/zoned-input.mjs`
- `apps/web/test/t00-profile.test.mjs`
- `apps/web/test/v3-ux-scope.test.mjs`
- `apps/web/test/zoned-input.test.mjs`

不纳入提交：`apps/web/src/v3/proto/data/profile-types.ts`（用户原有未跟踪文件）。

用户已确认按本清单提交并推送。工作提交和远端结果以 Git 记录为准。
