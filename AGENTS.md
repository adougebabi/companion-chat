

## 禁止提交图片

- 用户明确要求：不再向 Git 提交图片，包括回归截图、参考截图、生成图和 SVG 等图片文件。
- 图片只用于本地验证，放在 Git 忽略范围或仓库外；报告只提交文字、检查结果和必要的几何数据。
- 既有已跟踪图片也不得修改后提交；删除图片允许提交。只有用户后续明确授权提交特定图片时才能例外。
- 不使用 `git add -f`、跳过提交钩子或其他包装方式规避此要求。
- 当前仓库使用 `.githooks/pre-commit` 执行 `scripts/check-no-images-in-commit.mjs`。新检出后运行 `pnpm hooks:install` 安装钩子，提交前可运行 `pnpm check:commit-images`。

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
AEL project (active). For an engineering work request, use $ael-work; resume an existing task first. From the project root run `./ael doctor` and `./ael task list`; load exact task rules with `./ael context --task <id>`. Discussion/query alone creates no task.
Use $ael-work for authorized task execution; $ael-migrate-legacy only for reviewed legacy proposals.
Do not treat historical imports as current constraints or verified checks.
<!-- AEL:END -->
