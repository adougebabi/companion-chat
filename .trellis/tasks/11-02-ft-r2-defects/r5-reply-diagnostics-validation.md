# R5 P0 回复泄漏、D7 统计与 D1 恢复入口

2026-11-02。本轮用户授权修复；未提交或暂存 Git，未部署，未修改图片、
任何 `.env` 或 `/workspace/accept`。验收环境只读取容器日志、执行数据库 SELECT。

## 根因与证据

- 已查看 `/workspace/ux/audit-20261102-r5/` 的 07、12、11 三张截图。
- Writer 的 `parseReplyDraft` 默认把整段普通文本作为 Text；以前只解析 JSON
  envelope，后续 `normalizeReplyText` 仅规范空白，无法阻止 Markdown 内部协议入库。
- 原生 `conversation.reply` 还能绕过 Writer 解析。共同发布服务只 trim 后 INSERT，
  原生能力回执又返回原始参数 Text，存在第二条泄漏路径。
- 只读 SELECT 确认四条污染草稿由 Cydonia-24B-v4.1-MLX-Mixed 生成，冻结协议均为
  `fact-only-reply.v5`，草稿创建时间为 CST 20:48:16、20:51:40、20:54:32、
  20:57:02；污染早于推测的 Huihui 切换窗口。格式包括三段、缺少认知段和重复回复段。
- 21:01:37、21:02:48 的 `adk_final_message_empty` 确实发生，但对应失败 inbox 的
  assistant 消息数均为 0。ADK 拒绝空 final，不把 reasoning 转成回复；
  `agent_run_failed` 结算关闭未发布草稿、仅重放既有已提交回复，没有原文 fallback。
- D7 原来的摘要只有三组，遗漏等待、未知和其他终态；标签、筛选、计数各维护枚举。
  本工作区修复前已把 started 纳入 running，因此截图中的 running=0 不能单独证明
  当前代码遗漏 started；本轮以统一状态投影和真实渲染测试锁定该要求。

## 行为与文件

- `apps/core-go/internal/core/reply_body.go`：提取中英文回复正文，兼容标题装饰、
  双语、重复段、缺失段、围栏及压平格式；认知、事务和工具/时间字段进入私有段，
  直到下一显式回复段。重复相同正文去重，不同回复段以普通段落连接。
- `reply_provider.go`：围栏 JSON 仍按 envelope 校验；解析失败清空 Text；
  没有安全正文时返回错误，不返回整段原文。
- `tool_publication.go`、`builtin_capabilities.go`、`fact_only_reply_capability.go`：
  原生和准备草稿共用发布前解析，入库、回执及 committed_text 均使用净正文。
  原有私有 RawCandidate 诊断不变，不作为用户可见正文。
- `apps/web/src/lib/reply-body.mjs`：旧记录防御；接入 `ChatPage.tsx`、
  `ConversationHistory.tsx`、`InboxPage.jsx` 的回复预览和详情。清理后为空不回退
  原始协议；用户原文和非回复通知保持原状。
- `approved-ui/task-presentation.mjs`、`DiagnosticsWorkspace.jsx`、
  `TaskDetailPage.tsx`：统一 attention/running/waiting/completed/ended/unknown
  六组。标签计数、摘要总和与当前筛选列表一致。started 计执行中，超过 15 分钟
  仍计执行中并提示核对；unknown 单独计数。
- `WorldAutomaticEvents.jsx`：时钟根据实际周期显示“生成中”/“等待下一小时”；
  缺周期证据显示待确认，普通 waiting 任务显示“等待中”。
- `PersonaPreparationFeedback.tsx`：确认恢复卡实际位于聊天标题后、消息列表前；
  新增真实人格准备任务链接和模型用途入口，保留返回聊天地址、输入草稿及原请求恢复。
- 同步 `docs/llm-prompt-composition.md`、后端 fact-only reply 与前端 feedback spec。
  没有修改人格提示词、输入组成或任何 token 限制。

协议过滤保留正常 Markdown、代码与正文英文，不按语言猜删单词。共享样本按实际
审计格式重建，未复制私有会话全文，也未加入图片。

## 定向验证

全部通过，无跳过。仅运行相关测试，未运行全量测试或生产部署验收。

- Go：18 个顶层测试、51 个子用例。使用用户给定的隔离 PostgreSQL URL 和
  `GOCACHE=/tmp/fluctlight-go-cache`。新测试文件为 `reply_body_test.go`、
  `reply_body_integration_test.go`，共享 fixture 为 `testdata/reply_output_drift.json`。
  覆盖 33 种格式、无可见正文的拒绝、无原文 fallback、真实 Provider HTTP
  请求路径、原生/准备/围栏 JSON 发布、净正文回执和纯认知零消息。
  同时保留并运行 envelope、code、ownership、幂等、draft replay、失败/取消、
  reasoning-only、malformed settlement、世界时钟终态和人格准备恢复回归。
- JS：101 项通过。文件为 `reply-body.test.mjs`、`demo-replica-chat.test.mjs`、
  `demo-replica-inbox.test.mjs`、`d7-task-taxonomy.test.mjs`、
  `diagnostic-workspace.test.mjs`、`d12-world-tasks.test.mjs`、
  `ft-r1-defects.test.mjs`、`diagnostic-task-detail.test.mjs`。
  包含同一组 33 个 Go/JS fixture 和真实 React DOM 行为；D7 的 20 行样本为
  7 running + 6 unknown + 5 completed + 1 attention + 1 waiting，筛选后同样验证
  摘要总和、标签和列表数一致；世界页和诊断页验证相同的时钟文案。
- Web typecheck、lint：Node `/tmp/node-v24.12.0-linux-x64/bin`，执行
  `npm --prefix apps/web run typecheck` 和 `npm --prefix apps/web run lint` 均通过。
  pnpm 的 Corepack 全局缓存目录只读，改用 npm 执行仓库已有的相同 tsc 脚本。
- `go vet ./internal/core`、改动 Go 文件 gofmt、`git diff --check` 均通过。

完整本地测试输出在 `/tmp/fluctlight-r5-go-tests.jsonl` 和
`/tmp/fluctlight-r5-js-tests.log`，不纳入 Git。

## 旧断言变更清单

旧断言修改、删除或放宽：**0**。

唯一替换的旧测试数据行在 `ft-r1-defects.test.mjs`：给 `/detail` 返回的
`persona_preparation` 增加 `id:'persona:actual-task/A'`，用于验证真实形状任务链接
及编码；不改变 status、revision、can_recover 或旧断言。
其余旧测试文件只追加用例/断言，新测试验证新增行为。
