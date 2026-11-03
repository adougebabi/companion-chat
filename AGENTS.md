<!-- TRELLIS:START -->
# mini-trellis

This project uses mini-trellis as a memory layer. Working knowledge lives under `.trellis/`:

- `.trellis/spec/` — durable contracts (short markdown; read `index.md` first)
- `.trellis/research/` — topic inbox (`<topic>.md`; `README.md` is not a topic)
- `.trellis/workspace/` — per-developer journals

Record a session with `/mini-trellis:remember` (Claude/OpenCode), `/mini-trellis-remember` (Pi), or `$mini-trellis-remember` (Codex). When earlier decisions or relevant project rules are needed, use `mini-trellis mem recall "<topic or path>"`; expand evidence with `mini-trellis mem read "<ref>"`. No search is required for unrelated work. Promote lasting boundaries into spec with `mini-trellis-update-spec`.

Record only confirmed decisions, reusable research findings, or the session/compact handoff. Skip ordinary turns and unconfirmed guesses. New installs write notes without an automatic git commit; use `mini-trellis doctor` to check host wiring and `mini-trellis refresh --dry-run` before refreshing assets.

Keep confirmed rules short and faithful. Unconfirmed scope, exceptions, and implementation choices stay open or are labeled as inference, not added to the contract.

For a confirmed update, follow this short order: read the existing topic; edit that file; read it back and check the new rule, limits, and negations; run `python3 ./.trellis/scripts/add_session.py --title "..." --summary "..." --topic <actual-topic-path>`; read the journal and check its topic link. Journal-only success is not topic success; report partial writes accurately. New notes follow the requested or discussion language; updates preserve the document language.

When recall excerpts already answer the question with source and qualifiers, cite them and stop. Expand only missing context; do not repeat a lookup or fall back to dialogue-only `search` merely to reconfirm sufficient evidence.

mini-trellis does not create or drive tasks.

Managed by mini-trellis. Edits outside this block are preserved.
<!-- TRELLIS:END -->

## 禁止提交图片

- 用户明确要求：不再向 Git 提交图片，包括回归截图、参考截图、生成图和 SVG 等图片文件。
- 图片只用于本地验证，放在 Git 忽略范围或仓库外；报告只提交文字、检查结果和必要的几何数据。
- 既有已跟踪图片也不得修改后提交；删除图片允许提交。只有用户后续明确授权提交特定图片时才能例外。
- 不使用 `git add -f`、跳过提交钩子或其他包装方式规避此要求。
- 当前仓库使用 `.githooks/pre-commit` 执行 `scripts/check-no-images-in-commit.mjs`。新检出后运行 `pnpm hooks:install` 安装钩子，提交前可运行 `pnpm check:commit-images`。

## LLM 提示词修改必读

修改任何 Agent、Writer、媒体/视觉任务、Kev 或模型修复请求前，先读
[提示词构造规范](.trellis/spec/backend/prompt-composition-contract.md)。

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
