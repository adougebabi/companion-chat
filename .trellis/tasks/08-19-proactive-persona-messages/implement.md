# 实施计划：人格主动私聊与待定事件

## 前置门禁

- [ ] 用户审阅 `prd.md`、`design.md` 和本清单，明确允许进入执行阶段。
- [ ] 运行 `python3 ./.trellis/scripts/task.py start .trellis/tasks/08-19-proactive-persona-messages`，将任务状态切换为 `in_progress`。
- [ ] 执行 `trellis-before-dev`：读取 backend spec 详细规则，并在修改前搜索所有受影响的 marker、job 类型、消息 provenance 字段和删除路径。

## 实施步骤

### 1. SQLite migration 与领域 helper

- [x] 新增 migration v8：创建 `companion_pending_events`、due／persona 索引，给 `companion_messages` 增加 `proactive_pending_event_id` 及索引。
- [x] 更新 `deletePersona()` 的依赖删除顺序，确保先删除 pending jobs/events，再删除 conversations/messages；两条 pending provenance 外键使用 `ON DELETE SET NULL`，不阻塞清理。
- [x] 添加结构化状态常量、时间窗口和生命周期 helper；所有 helper 使用 persona-scoped 查询和短事务。
- [x] 添加 `normalizePendingEventCall()`、`extractPendingEventIntent()`、`createPendingEvent()`、`pendingEventShape()` 等单一 owner，拒绝服务器从自然语言补全时间或主题。

### 2. 聊天能力 marker

- [x] 在系统能力层新增 `<pending-event>` 契约，明确调用条件、严格 JSON schema、绝对时间、过期边界、重复登记和“不要在普通闲聊中调用”的规则。
- [x] 在 `streamPersonaChat()` 的完成阶段解析并移除 pending marker；合法 marker 创建 pending row/job，非法 marker只记录安全诊断，不阻断普通助手消息。
- [x] 保持媒体 `<media-intent>` 行为不变；确认 pending marker 与媒体 marker 同时出现时不会破坏任一解析结果。
- [x] 检查 SSE 流和当前 client 的 done/message 兼容性；新增 bounded marker redactor，最终持久化消息不含 marker。

### 3. 主动评估与冻结决定

- [x] 抽取共享主动评估器，支持 life event 与 pending event 两种 source，使用当前人格上下文和最近 18 条消息（待定事件额外附带冻结事实）。
- [x] 定义并校验结构化输出 `{schemaVersion, send, reason, message}`；`send=false` 不接受可见文案，`send=true` 限制 90 个中文字符并走 `appendUserVisibleAssistantReply()`。
- [x] 首次评估在租约内把决定冻结到 job `result_json.decision`；重试优先复用冻结决定，不再次调用 LLM，并设置 60 秒调用超时以保持在默认 lease 内。
- [x] 重构现有 `completeProactiveMessageJob()` 为共享 source settlement，保留现有屏蔽、活跃度／静默、日预算、租约和按来源幂等校验。

### 4. pending_event worker

- [x] 增加 `runPendingEventJob()` 并接入 `runMediaJob()` 分发。
- [x] 作业到期前不执行；超过 `expiresAt`、候选状态已终结或人格不存在时安全完成并写结构化 skip 结果。
- [x] 到期时允许即使用户最近 10 分钟仍在聊天；把当前人格 prompt、冻结候选和最近 18 条消息交给共享主动评估器，由人格自行决定如何介入或不发送。
- [x] `send=true` 投递成功后把 pending event 标记 `consumed` 并关联消息；`send=false` 也标记 `consumed`，防止无限重试。
- [x] 服务重启、过期 lease、LLM 失败和重复 settlement 继续遵守现有 job worker 规则；终端评估失败会将 pending event 标记 `cancelled`。

### 5. 生活事件触发接入

- [x] 在生活事件主动排队前增加“最近用户消息不足 10 分钟则跳过主动路径”的服务端判断；仍然持久化事件事实。
- [x] 保留显式 `proactive` 生产入口和现有 event type eligibility；让非活跃事件进入共享结构化评估，而非无条件使用 fallback 文案。
- [x] 确保同一 life event 只会有一个有效主动投递来源，旧 payload 仍可兼容。

### 6. 诊断与测试

- [x] 为 debug/lifecycle 摘要增加 persona-scoped pending event 和 pending job 信息（不暴露 prompt 或敏感字段）。
- [x] 增加 marker 合法／非法／缺时间／过期／重复／同时媒体 marker 测试，并用 bounded redactor 防止流式泄漏。
- [x] 增加 pending event 生命周期、due job、上下文评估、冻结决定重试、过期、屏蔽、活跃聊天介入、重复完成和 persona 删除测试。
- [x] 增加生活事件在活跃聊天与非活跃聊天下的生产路径测试。

## 验证命令

### 静态与单元验证

```bash
npm test
node --check server.js
```

### 数据库／迁移验证

```bash
DATA_DIR="$(mktemp -d)" DATABASE_PATH="$DATA_DIR/companion.sqlite" node --input-type=module -e "import './server.js'; console.log('migration-ok')"
```

### 服务烟测

```bash
DATA_DIR="$(mktemp -d)" PORT=4178 npm start
curl -sS http://localhost:4178/api/health
```

需要在临时 `DATA_DIR` 中验证：

- 普通聊天仍返回 `token`、`done`、`error` SSE 事件；
- 带合法 `<pending-event>` 的模型回复只持久化可见正文，并创建一个 pending row + durable job；
- 到期 worker 读取最近聊天上下文，只调用一次结构化评估；
- `send=false`、过期、屏蔽、重复 settlement 都不写入主动消息；
- 非活跃生活事件可进入主动评估，活跃生活事件不会排队主动路径。

## 风险与回滚点

- **Marker 解析风险**：先保持现有媒体 parser 不变，pending parser 独立；解析失败只丢弃能力调用，不丢聊天正文。
- **SSE 泄漏风险**：如果流式 token 在客户端短暂显示 marker，补充 bounded marker redaction；不能改变 `done.message` 兼容字段。
- **来源幂等风险**：任何写消息路径都必须带 `proactive_event_id` 或 `proactive_pending_event_id`，并在租约事务中再次查询。
- **冻结结果风险**：评估决定必须在同一有效 lease 下先持久化，重试不得从 job payload 重新调用 LLM。
- **迁移风险**：只新增 v8，不修改已有 migration；临时数据库先跑 migration，再执行测试。
- **回滚**：停止新 marker 生成和 pending job 创建；保留 v8 表与历史 rows，使用安全过期／完成迁移，不删除已有用户消息。

## 完成前检查

- [x] `prd.md`、`design.md`、`implement.md` 与实现中的 event/job/message 字段一致。
- [x] backend spec 的数据库、错误处理、debug observability、quality checklist 全部通过。
- [x] 全库搜索确认 `proactive_event_id`、`proactive_message`、`pending_event`、`<media-intent>` 和 SSE `done` 的生产者／消费者没有遗漏。
- [x] 运行 Trellis quality check，记录失败项与修复结果。
- [ ] 更新必要的 backend spec，提交前确认工作树只包含本任务相关变更。
