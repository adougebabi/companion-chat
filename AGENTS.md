

## 禁止提交图片

- 用户明确要求：不再向 Git 提交图片，包括回归截图、参考截图、生成图和 SVG 等图片文件。
- 图片只用于本地验证，放在 Git 忽略范围或仓库外；报告只提交文字、检查结果和必要的几何数据。
- 既有已跟踪图片也不得修改后提交；删除图片允许提交。只有用户后续明确授权提交特定图片时才能例外。
- 不使用 `git add -f`、跳过提交钩子或其他包装方式规避此要求。
- 当前仓库使用 `.githooks/pre-commit` 执行 `scripts/check-no-images-in-commit.mjs`。新检出后运行 `pnpm hooks:install` 安装钩子，提交前可运行 `pnpm check:commit-images`。

## 前端 UI 验收

修改 `apps/web` 的页面、组件、样式或修复 UI Bug 时遵守（详见
[前端 UI 质量门禁](docs/frontend-ui-quality.md) 与 `.impeccable.md`）：

1. 先从 `.impeccable.md` 的共享组件清单（`@/components/patterns`）选择页面模式与组件；
   只有确实没有合适模式时才新增组件。
2. 新组件先加入 UI Lab（`apps/web/ui-lab/`），覆盖适用的正常 / 长文本 / 空态 / 加载 /
   失败 / 禁用状态与三个视口，再接入真实业务数据与事件。
3. 不在业务 CSS 中覆盖共享组件的关键几何属性；修 UI Bug 不能只补一条 CSS 覆盖了事，
   要修正共享模式或其用法，并补上能复现该问题的浏览器检查。
4. 运行 `node apps/web/scripts/ui-gate.mjs`（或 `pnpm --filter @fluctlight/web ui:gate`）。
   **只有结果为 `pass`（或无 UI 改动时的 `not-applicable`）才能报告 UI 验收完成。**
   `fail` 与 `unverified`（浏览器不可用、缺少已审核基准、改动无浏览器覆盖）都必须如实
   报告为未通过 / 未验证，不得表述为通过。"代码能运行、jsdom 测试通过"不是 UI 验收结果。
5. 视觉基准只能在设计变更审核后用 `ui:baselines approve --reason …` 更新；不得为通过测试
   重录基准、放宽阈值或删除断言。基准图片在仓库根 `.ui-baselines/`，不提交。

## LLM 提示词修改必读

修改任何 Agent、Writer、媒体/视觉任务、Kev 或模型修复请求前，先读
[提示词构造规范](.ael/knowledge/spec/legacy-9b89c41b4be449b963dcba95.md)。

- **不得通过新增、恢复或调整 prompt/context 输入 token 上限、response/output
  输出 token 上限、分段配额、预算拒绝或按 token 估算截断来优化提示词。**
  不发送 `max_tokens` / `max_completion_tokens`；估算只用于诊断。
- 统一结构：运行协议与必要人格约束 + 按任务选择的 JSON 动态数据 + 按需历史
  + 当前输入一次。保留原生工具协议和真实图片。
- 无用数据库 ID、版本、digest、执行/调度元数据留在 Core。必要业务引用须有明确
  消费方；禁止按字段后缀递归删键，禁止改写用户正文或 schema 属性。
- 保留记忆/摘要优先及完整对话组的覆盖去重；同来源不同事实必须保留。
- 新增或修改模型入口时同步
  [Agent 提示词组成清单](docs/llm-prompt-composition.md)和真实请求回归。

<!-- AEL:BEGIN -->
AEL project (remote). Before any AEL command, first decide whether the user's goal is a project iteration, bug fix, or other engineering change to the project itself.
For discussion, lookup, analysis, review, or review-only materials, even when saved to a file, work directly and communicate with the user; do not run AEL commands or create tasks or enter implementation/acceptance workflows for that request. Read relevant project files normally as needed.
For an independently requested formatting, whitespace, or typo correction with clear scope and unchanged behavior, contracts and permissions, fix and verify directly without starting a session or creating a task unless tracking was requested. For other authorized engineering changes, use $ael-work. For failures in existing work or delivery, apply its FEEDBACK routing before starting the regular workflow. For regular work, run `./ael doctor` and `./ael task list` from the project root, reuse the relevant task, and load its rules with `./ael context show --task <id>`. If the request later becomes an authorized engineering change, apply the same routing before making that change.
If the user explicitly requests an AEL command or an AEL task, follow that request. Use $ael-migrate-legacy only for reviewed legacy proposals.
Do not treat historical imports as current constraints or verified checks.
In portable clones, use ./ael doctor to discover the automatically registered runner and omit --runner when starting the session. Before new tracked Remote work, run `./ael session start --session <host-session-id> --runner <runner-id>` and read its returned frozen Skill path. Pass `--session <host-session-id>` on subsequent Remote commands. After context compaction, session resume, task switch, or uncertain progress, first run `./ael recovery check --session <host-session-id> --json`. Verify Server/project, current parent/leaf task, step, versions, accepted checks and latest blocker before continuing the unfinished item. Server records govern task identity and scope. Until online verification succeeds, limit work to read-only inspection and saving the scene. Bind a new session explicitly with `./ael recovery save --session <host-session-id> --runner <runner-id>` and refresh it after each verifiable unit.
<!-- AEL:END -->
