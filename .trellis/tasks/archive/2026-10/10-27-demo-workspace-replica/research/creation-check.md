# 第 15 组初始化独立质量门禁（2026-10-28）

范围仅初始化；第 1–14 组沿用既有门禁，第 16 组未实施。本次未勾选任务、修改规格或提交。

## Findings (fixed)

- `apps/web/src/react-app/views/CharacterWizard.jsx:178`、`:1318`、`:1345`：旧草稿缺失 `worldSelectionConfirmed` 时，实际世界绑定匹配并达到 ready/7 项完成，界面仍隐藏进入聊天按钮，已有世界来源还会显示重复复用提示。统一界面判断为本地确认或实际 world ready；实际阶段未 ready 且无本地确认时继续保留核对入口。
- `apps/web/test/demo-replica-creation-ui.test.mjs:115`：新增真实 View 回归，覆盖旧草稿 ready 的聊天链接及无重复复用提示，并切回 pending 确认聊天不出现、世界核对仍可达。

## Findings (not fixed)

无其他确认的阻断缺陷。规格同步由主线程审计后处理，建议记录旧本地检查点缺失时以实际世界绑定读回作为完成与进入聊天依据。

## 检查依据

- 亲读 PRD/design/implement 与 creation-implementation；检查创建 controller、页面、View、七阶段投影、PreparationSteps、草稿 checkpoint、JSON 解析和 Demo CreationPage/creation.css。
- 完整 JSON 保留嵌套扩展、null/false/0；可编辑投影剥离分析凭证，应用时拒绝来源替换。未应用 JSON 阻止激活。400/422 与首次分析版本拒绝保留无旧授权预览副本，必须重新分析；未知激活只能重试冻结 body/identity。
- 检查 Owner/analysis/read epoch 与 disposal；激活回执先持久 checkpoint 后释放命令。阶段恢复待确认记录与 ready 状态独立，已拒绝恢复先实际读回再明确解除，未知回执重复使用原 body/key。
- `creation-progress.mjs:95` 要求非空世界 ID 与 `/world` 实际 ID 匹配，不依赖旧本地选择标记。七阶段由各正式读取及原分析确认构成，未复制 Demo 计时完成。
- 三种实际世界来源、四种人物输入、当前 Owner groups、精确 immutable publication 及失败原 cursor 重读、激活后只读均核对；未引入后端/API/prompt/token 策略修改。
- 检查 geometry.json 和最终 desktop/mobile 图片，抽看 source-description-mobile 对 Demo、world-source-conflict-mobile 的 6/7、complete-desktop 的 7/7。桌面步骤条/主栏/摘要/操作栏几何匹配；390px 实际无文档横向溢出，44px 控件导致的 8px/2px 差异已有明确记录。
- 请求审计核实：拒绝后第二次 analysis 接收保留 false/null/0 和编辑背景的 JSON、无旧分析凭证；未知衣柜恢复两次 body/key 相同；世界 revision3 冲突后明确核对 revision4 生成新复制 key，只有一次激活；取消场景只有 analysis、无 activate。

## Verification

- Lint / TypeCheck: pass，重新执行 `pnpm --filter @fluctlight/web lint`（两者脚本均为 `tsc --noEmit`）。
- Tests: pass，重新执行完整 Web 测试 **610/610**；日志 `/private/tmp/demo-replica-creation-check-tests.log`。新增回归前初始化专项 **18/18** 独立通过。
- Build: pass，重新构建生产 dist，fixture exclusion / React-only graph **371 local modules** 均通过；日志 `/private/tmp/demo-replica-creation-check-build.log`。
- `git diff --check`: pass。

全部 UI 证据为正式 dist + 隔离 API fixture，不能替代 live Core/DB/model/worker/storage 验收。既有大 chunk 提示不属于本组新失败。
