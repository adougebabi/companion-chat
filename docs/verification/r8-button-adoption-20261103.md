# R8：领域按钮迁移至 shadcn Button

## 范围与根因

本轮只处理用户给定 UI ITEM。参考证据只读：`/workspace/ux/audit-20261103-r8/`。

领域工具的 623 处模板按钮声明以及 2 处动态创建按钮仍依赖 `btn`、`primary`、`danger`、`sm` 等自定义类。现有 `DomainPrimitives` 虽已转接至真实 shadcn Button，但仍从类名推断 variant，漏掉 `teal`，完全没有传递 size；当前加载的 `domain-tools.css` 还保留另一套 `.btn` 样式。因此既有组件安装和转接不足以保证完整、一致的按钮采用。

本轮所有声明显式提供 `data-domain-variant` / `data-domain-size`，由现有单一 React 应用树中的 `ButtonPort` 传给 `apps/web/src/components/ui/button.tsx`。可见按钮使用真实 Button；原节点只保留为不可见的事件及表单载体，不新建 React root、不复制命令实现。链接继续使用 Button `asChild`，保留 href、target、download，并补齐 rel 转交。当前生产源代码 JS/TS/JSX/TSX 中已无 `btn` 类。

| 原语义 | Button variant / size |
| --- | --- |
| 普通 `btn` | `outline` / `default` |
| `primary` / `accent` / `teal` | `default` |
| `danger`，含 `ghost danger` | `destructive` |
| `ghost` / 未选中的 chip | `ghost` |
| 选中 `on` / `aria-pressed=true` | 对应声明显式使用 `secondary` |
| `sm` | `sm` |
| `icon-only` | `icon` |

动态选中/危险状态保留原表达式，改为直接输出对应 variant。查询 `.primary` 的领域控制器改为查询显式声明，维持提交时禁用、回执后的恢复及原草稿语义。业务结构类继续转交可见 Button；不再复制按钮外观 CSS。

## 文件与复用

- 核心：`apps/web/src/react-app/shared/DomainPrimitives.tsx`、`apps/web/src/v3/proto/components/forms.js`、`apps/web/src/styles/domain-tools.css`。
- 声明：`apps/web/src/v3-domain/*.js` 和 `apps/web/src/v3/proto/components/*.js`；完整清单见下文。
- 复用 shadcn：`Button`，包括 `asChild` 锚点；沿用既有 `ControlHint`（Tooltip）及原有 Sheet / Dialog 承载路径。
- 新增 `apps/web/test/r8-button-adoption.test.mjs`：真实渲染的 variant/size、单次点击、原生 submitter/FormData、disabled/hidden、动态标签及 props、焦点、链接属性、重绘清理、生产声明扫描。
- 既有 `r11-domain-primitives`、`r2-presentation` 的按钮夹具改成显式 variant/size。`r12-regressions` 的衣柜准备夹具补齐 `current.value.garments=[]`；该组件原本已读取此字段，旧夹具因此在断言前抛错。生产衣柜组件未改。

## 验证

Node：`/tmp/node-v24.12.0-linux-x64/bin`。使用冻结锁文件安装依赖，Corepack/pnpm/npm 缓存均位于 `/tmp`，包清单和锁文件未修改。

- `pnpm --filter @fluctlight/web lint`：通过。
- `pnpm --filter @fluctlight/web typecheck`：通过。
- `pnpm --filter @fluctlight/web build`：通过；fixture 排除与 React-only 400 模块生产图守卫通过。构建仍有既有大 chunk 提示。
- 针对性 JS：111 个不同案例全部通过。日志为 `/tmp/r8-button-migration/targeted-tests-final.log`（40）、`domain-tests.log`（46）、`selector-tests.log`（30）；其中 `r2-presentation` 的 5 个案例在两组中重叠。
- 生产浏览器：本地当前 dist + 任务独占的内存 HTTP fixture，1920×1080、390×844，6 个场景、92 个可见 Button 核对通过，无页面 JS 异常、无横向溢出。检查数据页、备份 Sheet、目标规划页；键盘 Enter 打开一次弹层，空必填不能发出写请求，Escape 关闭后恢复触发器焦点，清空草稿可到达原控制器。
- ComfyUI：对照 Git HEAD，对 12 个含 ComfyUI 的生产源文件做全文等价检查，只忽略按钮/链接开始标签中的 class 和新增的 variant/size 映射属性（href、aria 与文字均参与比较）；19 处出现及周边全部文案保持一致。
- `git diff --check`：通过。

浏览器几何、variant/size、载体可见性等原始数据：`/tmp/r8-button-migration/browser/results.json`；本地截图在同目录，未加入仓库。断言审计：`/tmp/r8-button-migration/assertion-audit.json`；ComfyUI 审计：`/tmp/r8-button-migration/comfyui-preservation.json`。

| 视口 | 场景 | 可见 Button 数 | 结果 |
| --- | --- | --- | --- |
| desktop | data | 15 | 通过 |
| desktop | backup-dialog | 17 | 通过 |
| desktop | planning | 14 | 通过 |
| mobile | data | 15 | 通过 |
| mobile | backup-dialog | 17 | 通过 |
| mobile | planning | 14 | 通过 |

运行的 JS 测试文件：

```text
r8-button-adoption.test.mjs
r11-domain-primitives.test.mjs
r11-controls.test.mjs
r11-domain-command.test.mjs
r25-rendered-ui.test.mjs
r2-presentation.test.mjs
r11-flows.test.mjs
r11-overlays.test.mjs
r11-studio.test.mjs
r12-regressions.test.mjs
r14-regressions.test.mjs
r8-recovery.test.mjs
v3-mock-acceptance.test.mjs
goal-planning-recovery.test.mjs
request-reason-diary.test.mjs
v3-life-provider.test.mjs
v3-wardrobe-world.test.mjs
v3-world-provider.test.mjs
```

## 断言变更与边界

既有断言变更：**0**。通过 TypeScript AST 比较三个修改过的既有测试文件，171 个 `assert.*` 调用的表达式与 HEAD 完全一致；只更新上述夹具。新增测试增加覆盖，不删除、不跳过、不放宽原断言。

没有访问 `/workspace/accept` 或验收服务，没有修改任何 `.env`，没有执行 Git 写命令或提交。全部 JS 测试使用本地 DOM/内存 mock 或独占 HTTP fixture，未连接数据库，未清理数据库行；不声称完成真实 Core、数据库、模型或 ComfyUI 服务验收。没有修改图片、模型提示词或后端代码。

## 完整生产源码变更清单

- `apps/web/src/react-app/shared/DomainPrimitives.tsx`
- `apps/web/src/styles/domain-tools.css`
- `apps/web/src/v3-domain/about-you.js`
- `apps/web/src/v3-domain/avatar.js`
- `apps/web/src/v3-domain/backups.js`
- `apps/web/src/v3-domain/data-tools.js`
- `apps/web/src/v3-domain/diagnostics.js`
- `apps/web/src/v3-domain/diary.js`
- `apps/web/src/v3-domain/generate.js`
- `apps/web/src/v3-domain/goal-editor.js`
- `apps/web/src/v3-domain/goal-planning.js`
- `apps/web/src/v3-domain/governance.js`
- `apps/web/src/v3-domain/history.js`
- `apps/web/src/v3-domain/life-schedule.js`
- `apps/web/src/v3-domain/life-timeline.js`
- `apps/web/src/v3-domain/life-wardrobe.js`
- `apps/web/src/v3-domain/material.js`
- `apps/web/src/v3-domain/media.js`
- `apps/web/src/v3-domain/memory.js`
- `apps/web/src/v3-domain/moments.js`
- `apps/web/src/v3-domain/npc-management.js`
- `apps/web/src/v3-domain/operations.js`
- `apps/web/src/v3-domain/reflection.js`
- `apps/web/src/v3-domain/self.js`
- `apps/web/src/v3-domain/studio.js`
- `apps/web/src/v3-domain/visual.js`
- `apps/web/src/v3-domain/workflow.js`
- `apps/web/src/v3-domain/world-details.js`
- `apps/web/src/v3-domain/world-events.js`
- `apps/web/src/v3-domain/world-items.js`
- `apps/web/src/v3-domain/world-management.js`
- `apps/web/src/v3/proto/components/async-page.js`
- `apps/web/src/v3/proto/components/business-backups.js`
- `apps/web/src/v3/proto/components/calendar-reading.js`
- `apps/web/src/v3/proto/components/cards.js`
- `apps/web/src/v3/proto/components/character-card.js`
- `apps/web/src/v3/proto/components/chat-life-summary.js`
- `apps/web/src/v3/proto/components/chat.js`
- `apps/web/src/v3/proto/components/configuration-editor.js`
- `apps/web/src/v3/proto/components/creation-preview.js`
- `apps/web/src/v3/proto/components/cursor-records.js`
- `apps/web/src/v3/proto/components/diagnostic-card.js`
- `apps/web/src/v3/proto/components/diagnostic-clear.js`
- `apps/web/src/v3/proto/components/display-avatar.js`
- `apps/web/src/v3/proto/components/durable-command-drawer.js`
- `apps/web/src/v3/proto/components/event-definition.js`
- `apps/web/src/v3/proto/components/forms.js`
- `apps/web/src/v3/proto/components/foundation-proposal.js`
- `apps/web/src/v3/proto/components/goal-ordering.js`
- `apps/web/src/v3/proto/components/governance-command.js`
- `apps/web/src/v3/proto/components/governance-records.js`
- `apps/web/src/v3/proto/components/initialization-progress.js`
- `apps/web/src/v3/proto/components/inventory-actions.js`
- `apps/web/src/v3/proto/components/inventory-editor.js`
- `apps/web/src/v3/proto/components/media-assets.js`
- `apps/web/src/v3/proto/components/media-generation-drawer.js`
- `apps/web/src/v3/proto/components/media-generation.js`
- `apps/web/src/v3/proto/components/media-lifecycle.js`
- `apps/web/src/v3/proto/components/media-metadata.js`
- `apps/web/src/v3/proto/components/media-reference-grants.js`
- `apps/web/src/v3/proto/components/media-upload.js`
- `apps/web/src/v3/proto/components/moment-card.js`
- `apps/web/src/v3/proto/components/moment-composer.js`
- `apps/web/src/v3/proto/components/npc-runtime-records.js`
- `apps/web/src/v3/proto/components/outfit-catalogue.js`
- `apps/web/src/v3/proto/components/outfit-composer.js`
- `apps/web/src/v3/proto/components/outfit-definition.js`
- `apps/web/src/v3/proto/components/outfit-editor.js`
- `apps/web/src/v3/proto/components/outfit-library.js`
- `apps/web/src/v3/proto/components/pending-groups.js`
- `apps/web/src/v3/proto/components/place-defaults.js`
- `apps/web/src/v3/proto/components/profile.js`
- `apps/web/src/v3/proto/components/promise-catalogue.js`
- `apps/web/src/v3/proto/components/provider-editor.js`
- `apps/web/src/v3/proto/components/reflection-progress.js`
- `apps/web/src/v3/proto/components/request-failure.js`
- `apps/web/src/v3/proto/components/resource-lifecycle.js`
- `apps/web/src/v3/proto/components/revisions.js`
- `apps/web/src/v3/proto/components/routine.js`
- `apps/web/src/v3/proto/components/runtime-world-framework.js`
- `apps/web/src/v3/proto/components/schedule-records.js`
- `apps/web/src/v3/proto/components/schedule-request.js`
- `apps/web/src/v3/proto/components/self-claims.js`
- `apps/web/src/v3/proto/components/states.js`
- `apps/web/src/v3/proto/components/studio-comparison.js`
- `apps/web/src/v3/proto/components/studio-jobs.js`
- `apps/web/src/v3/proto/components/studio-material-retention.js`
- `apps/web/src/v3/proto/components/versioned-fields.js`
- `apps/web/src/v3/proto/components/visual-identity-records.js`
- `apps/web/src/v3/proto/components/visual-review.js`
- `apps/web/src/v3/proto/components/wardrobe-preparation.js`
- `apps/web/src/v3/proto/components/world-definition.js`
- `apps/web/src/v3/proto/components/world-event-runtime.js`
- `apps/web/src/v3/proto/components/world-framework-generator.js`
- `apps/web/src/v3/proto/components/world-items.js`
- `apps/web/src/v3/proto/components/world-presentation.js`
- `apps/web/src/v3/proto/components/world-venues.js`
