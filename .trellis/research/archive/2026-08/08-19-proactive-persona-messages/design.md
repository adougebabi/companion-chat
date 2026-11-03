# 技术设计：人格主动私聊与待定事件

## 1. 设计边界

本版本只扩展现有单体 Node/Express + SQLite durable job 架构，不引入外部队列、第二个数据库、浏览器推送或正式流式 `tool_calls`。聊天中的待定事件能力沿用当前媒体能力的 marker 传输方式；后续迁移到 `tools` 需要单独设计兼容层。

现有关键入口：

- `server.js:40`：应用拥有的系统能力提示与媒体 marker 契约。
- `server.js:1394`：`createEvent(persona, event, options)`，生活事件事实和主动作业的创建边界。
- `server.js:1899`：`extractMediaIntent()`，模型 marker 的解析模式。
- `server.js:2480`：`streamPersonaChat()`，聊天完成、可见消息落库和媒体作业创建。
- `server.js:3176`：`completeProactiveMessageJob()`，主动消息写入时的租约、资格和幂等门。
- `server.js:3205`：`runProactiveMessageJob()`，主动评估模型调用入口。
- `server.js:3449`：`runMediaJob()`，通用 durable job 分发入口。

## 2. 触发模型

```text
生活事件（非活跃聊天）
  -> createEvent(..., {proactive: true})
  -> proactive_message job
  -> 一次结构化主动评估
  -> 资格复核 + 幂等投递

正常聊天
  -> 模型可选 <pending-event> marker
  -> 校验并落库 pending event
  -> pending_event job(run_after = notBefore)
  -> 到期时读取当前聊天上下文
  -> 一次结构化主动评估
  -> 资格复核 + 幂等投递
```

生活事件和待定事件共享评估输出与投递边界，但活跃聊天规则不同：

- 生活事件发生时，最近用户消息不足 10 分钟则不创建主动评估、待定事件或主动消息作业；只保留生活事件事实。
- 已由人格在聊天中显式登记的待定事件是用户授权的后续关注；到期时即使最近用户消息不足 10 分钟，也允许评估。评估 prompt 必须带入当前人格上下文、冻结待定事实和最近 18 条消息，让人格自行决定如何自然介入或 `send=false`。

## 3. 持久化模型

新增 migration v8：

### `companion_pending_events`

| 字段 | 说明 |
| --- | --- |
| `id` | `pending_event_*` 主键 |
| `persona_id` | 所属人格，所有读取必须带 persona 约束 |
| `source_message_id` | 登记该能力的当前用户消息 ID；服务端绑定，忽略模型伪造的 ID；消息删除时置空 |
| `status` | `pending`、`triggered`、`consumed`、`cancelled`、`expired` |
| `summary` | 后续关注所需的最小事实，最多 280 字 |
| `not_before` | 最早可触发时间，ISO 字符串 |
| `expires_at` | 最晚有效时间，ISO 字符串 |
| `dedupe_key` | 模型提供的稳定短键，服务端限长并与时间组成重复登记保护 |
| `payload_json` | 版本化原始结构化事实和来源摘要，不保存完整聊天 prompt |
| `created_at` / `updated_at` | 审计时间 |
| `triggered_at` / `consumed_at` / `cancelled_at` | 生命周期时间戳，可为空 |

约束与索引：

- `UNIQUE(persona_id, dedupe_key, not_before)`，允许同一主题在不同明确时间再次登记，但阻止同一候选被重复创建。
- `companion_pending_events_due_idx(persona_id, status, not_before, expires_at)` 支持诊断和有界查询；正常运行不依赖全表扫描。
- `source_message_id` 以 `ON DELETE SET NULL` 关联 `companion_messages`；删除人格时先删除 pending jobs 和 pending events，再删除消息，避免相互 provenance 外键阻塞清理。

### `companion_messages`

新增 nullable `proactive_pending_event_id` 外键（`ON DELETE SET NULL`）和 persona-scoped 索引；保留现有 `proactive_event_id` 以兼容生活事件。`messageShape()` 暴露可选 `proactivePendingEventId`，不要求客户端新增 UI。

### `companion_jobs`

新增 `job_type = 'pending_event'`。payload 至少包含 `{pendingEventId}`；`run_after` 冻结为 `notBefore`，`maxAttempts` 复用 durable worker 的有限重试。作业结果记录触发／跳过／过期／投递结果，不把完整 prompt 或敏感配置写入结果。

## 4. Marker 契约

系统能力层新增并列契约：

```xml
<pending-event>{
  "schemaVersion": 1,
  "summary": "下午有一场面试，之后可以问结果",
  "notBefore": "2026-08-19T18:00:00+08:00",
  "expiresAt": "2026-08-20T12:00:00+08:00",
  "dedupeKey": "下午面试"
}</pending-event>
```

服务端规则：

- 一条聊天回复最多接受一个待定事件 marker；未知字段忽略，缺字段、非法 JSON、时间无效、`expiresAt <= notBefore`、超过最大未来窗口（30 天）或摘要为空时安全拒绝，不创建记录。
- `sourceMessageId` 始终使用本次请求已经落库的用户消息 ID，不信任 marker 中的来源 ID。
- `notBefore` 和 `expiresAt` 必须是带时区、可解析的绝对时间；服务端不从自然语言猜测时间。
- marker 从最终持久化的用户可见助手正文中移除；无合法 marker 时不创建待定事件。
- marker 解析先移除完整或未闭合的 marker 区段，再在有界正文上做 JSON 校验；超长、损坏或重复 marker 只丢弃能力调用，不把内部标签写入可见正文。SSE 流使用 bounded redactor，在 token 到达浏览器前抑制 `<media-intent>` 与 `<pending-event>` 区段。
- 待定事件能力只负责登记事实和时间，不直接写入主动消息。

## 5. 主动评估契约

生活事件与待定事件到期时，复用同一结构化模型输出：

```json
{
  "schemaVersion": 1,
  "send": true,
  "reason": "面试已经结束，适合自然问候",
  "message": "面试结束了吗？今天感觉怎么样？"
}
```

服务端绑定来源类型和 ID，不接受模型改变来源。校验规则：

- `send` 必须为 boolean。
- `send=false` 时 `message` 为空；`send=true` 时 message 非空、最多 90 个中文字符（保留现有主动文案约束），再通过 `appendUserVisibleAssistantReply()` 落库。
- `reason` 仅作有界诊断，不暴露给用户。
- 首次作业运行时调用一次 LLM；先在当前租约下把结构化决定写入 job `result_json.decision`，再执行投递。重试看到冻结决定时跳过 LLM，避免文案漂移。
- 模型请求设置 60 秒超时，短于当前非媒体 job 的 90 秒默认 lease；超时进入既有有限重试，避免 lease 过期后重复并发评估。
- LLM 调用失败按现有 `settleJob()` 重试；达到上限后按来源状态安全失败，不生成 fallback 主动消息。

待定事件评估输入：

- `userVisibleChatPrompt(personaId, taskInstruction)` 产生的当前人格上下文和系统能力层；
- 冻结待定事件事实（summary、notBefore、expiresAt、source）；
- 最近 18 条消息，必要时遵循现有 prompt 总长度预算裁剪。

生活事件评估输入保持当前事件事实和同一人格上下文；不增加后台聊天扫描。

## 6. 生命周期与幂等

1. `createPendingEvent()` 在短事务中插入候选和 `pending_event` job；重复的 `(persona, dedupeKey, notBefore)` 返回既有候选，不重复排队。
2. `runPendingEventJob()` 先在租约内读取候选：缺失、已消费／取消或当前时间超过 `expires_at` 时只更新状态并完成作业，不调用 LLM。
3. 到期候选标记为 `triggered`，调用共享主动评估器；评估决定先冻结在 job 结果中。
4. `completeProactiveSourceJob()` 在同一租约事务内重新检查人格屏蔽、来源状态、静默／预算／去重；`send=false` 标记候选 `consumed`，`send=true` 插入助手消息并标记 `consumed`。
5. 过期、取消、屏蔽、日预算或重复投递都产生结构化 `result_json.skipped`，不创建用户可见消息。
6. 人格删除沿用现有依赖顺序，先删 pending jobs 和 pending events（消息 provenance 由 `ON DELETE SET NULL` 解除），再删 conversations/messages 与其他 persona-private rows。

若待定事件评估在有限重试后仍失败，job 进入 `failed`，pending event 从 `triggered` 转为 `cancelled` 并记录 `evaluation_failed`；只有仍可重试的 job 才保留 `triggered`。

## 7. 兼容性与回滚

- 旧生活事件作业仍通过 `proactive_message` 路径处理；payload 没有 `pendingEventId` 时按 `eventId` 兼容。
- 旧消息没有 `proactive_pending_event_id` 时读取为 null。
- 新 migration 只追加表、列和索引，不修改已应用 migration。
- 若新 marker 解析或 pending job 失败，普通聊天正文仍可落库；待定事件失败不应阻断聊天 SSE。
- 通过回滚代码可以停止创建新 pending marker；已存在的 pending rows/job 可安全标记过期或由兼容 worker 完成，不删除历史聊天消息。

## 8. 观测与测试边界

- 诊断只返回 persona-scoped 的待定事件／job 摘要、生命周期和跳过原因；不返回完整 prompt、API key 或原始敏感上下文。
- 单元测试覆盖 marker 解析、时间／长度／重复校验、生命周期转换、结构化主动决定、冻结决定重试和来源幂等。
- 集成测试覆盖聊天 marker → pending row → due job → LLM decision → message，以及生活事件 → proactive job；覆盖屏蔽、活跃聊天、过期、重启／过期 lease 和第二次完成。
