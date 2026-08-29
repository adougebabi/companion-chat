# 实施计划：LLM 结构化回合协议与人格状态记忆

## 前置门禁

- [x] 用户审阅 `prd.md`、`design.md` 和本清单，确认混合协议、显式 `memory_event`、三项可扩展 drives 和“不暴露思维链”边界。
- [x] 运行 `python3 ./.trellis/scripts/task.py start .trellis/tasks/08-22-llm-structured-turn-protocol`，再进入实现阶段；当前规划阶段不执行代码改造。
- [x] 执行 `trellis-before-dev`，读取 backend/frontend 详细规范，并搜索所有 `tool_calls`、`<media-intent>`、`<pending-event>`、SSE `done`、memory、state 和 commit boundary 的生产者/消费者。
- [x] 确认当前工作树中的既有用户改动，不覆盖无关修改；所有跨层 contract 变更先更新 producer/consumer 清单。

## 实施步骤

### 1. 合同与 schema owner

- [x] 在 contracts/application 边界定义 `CompanionTurnResultV1`、control candidate、`memory_event` 和 allowlisted affect/drive event 类型。
- [x] 定义字段上限、数组数量、字符串长度、confidence 范围、枚举、source/persona 关联、版本和错误码。
- [x] 将 native tool calls、结构化 sidecar、旧 marker 和纯文本归一化到同一个内部结果；不新增 affect/memory 文本 marker。
- [x] 为 malformed JSON、未知 schemaVersion、未知字段、重复 capability 和 provider 不支持建立 fail-closed 语义。

### 2. Affect/drives domain 与持久化

- [x] 新增 affect snapshot/event SQLite migration，包含 `(persona_id, idempotency_key)` 唯一约束、persona/time/causation 索引和 migration ledger 验证。
- [x] 新增表级 repository：读取衰减后的 snapshot、追加事件、CAS 更新 snapshot、按 persona 查询审计记录。
- [x] 实现固定三项 drives：`social`、`exploration`、`rest`，pressure 范围 `0..1`；未知未来 key 可保留但不能未经注册参与策略。
- [x] 实现 persona baseline、weight、half-life 配置读取和安全默认值；固定 decay model version。
- [x] 实现 allowlisted event type 到 PAD/drives bounded delta 的 reducer；拒绝模型直接提交任意 delta。
- [x] 覆盖同一事务中的 event + snapshot、重复幂等、CAS 冲突、跨 persona 隔离、惰性衰减、重启和长离线场景。

### 3. Memory capability

- [x] 新增 `memory_event` capability schema、plan/apply flow、registry 注册和 persona ownership 校验。
- [x] 扩展 memory repository/service 支持受限 upsert 或安全重复忽略，并保留 source type/source ID/confidence/时间。
- [x] 让 assistant message、memory plan、affect plan 和其他 capability effect 复用现有 commit boundary；任何 apply 失败都整体回滚。
- [x] 只有显式 `memory_event` 才能从普通聊天写入长期 memory；`learned`/`memoryChanges` 只能表示实际提交成功的结果。
- [x] 增加重复调用、跨 persona、空值/超长值、错误 source、transaction rollback 和 persona 删除测试。

### 4. Provider/application/chat flow

- [x] 在 provider adapter 中保留可见 text/token streaming，同时收集 native structured calls 或 provider parsed control sidecar。
- [x] 更新 normalized completion，使 control payload 和 parse diagnostics 与旧 text completion 共存。
- [x] 在 chat flow 的 completion/presentation/commit 边界接入结构化校验和 plan 生成；control 未通过校验时不得执行副作用。
- [x] 保持旧 `content`、原生 tool call、legacy media/pending marker 和旧 text-only completion 的兼容路径。
- [x] 保证现有 `done.messages` 为权威集合、`done.message` 为第一条兼容别名，外层 SSE 事件名不变。
- [x] 为 provider 返回 object、malformed JSON、半截结构化流、工具参数非法、control 与文本同时存在等场景补齐测试。

### 5. Context and deterministic behavior policy

- [x] 从 context reader 读取 decayed affect/drives snapshot，并生成不暴露原始数值的 `replyPosture`。
- [x] 让 posture 影响回复姿态、休息延迟建议和 timeline 候选偏置，但不绕过静默、屏蔽、预算、lease 和 safety gate。
- [x] 将 timeline、deferred chat 和 debug inspector 接入结构化状态摘要/治理 reason code；proactive worker 继续复用已有最终 gate。
- [x] 普通 UI 只接收安全摘要；debug inspector 只显示 persona-scoped bounded diagnostics，不显示 prompt、凭据或隐藏推理。

### 6. Rollout and documentation

- [x] 增加结构化控制 feature flag/kill switch；关闭时保留 text-only 聊天和既有能力行为。
- [x] 更新 backend contract baseline、API/SSE 兼容说明、migration 说明和开发调试文档。
- [x] 记录新表、字段、effect、source 和删除路径；确认 persona 删除不会留下不可引用的 affect/memory rows。
- [x] 完成 PRD convergence pass，移除已解决问题和临时讨论段落；再运行质量检查。

## 验证命令

### 静态与单元验证

```bash
npm test
npm run typecheck
npm run build
node --check server.js
```

### 数据库与迁移验证

```bash
DATA_DIR="$(mktemp -d)" DATABASE_PATH="$DATA_DIR/companion.sqlite" node --input-type=module -e "import './server.js'; console.log('migration-ok')"
```

验证临时数据库中的 migration ledger、affect tables/indexes、默认 blueprint policy、重复 idempotency 和旧 schema 重开。

### Chat/SSE 兼容验证

- 旧 text JSON completion 仍产生 token/done。
- native tool call 和 structured control sidecar 归一化到同一 turn result。
- malformed control 只丢弃控制副作用，不产生半条 memory/affect/job。
- `done.messages`、`done.message`、旧客户端 parser 和断开/重试行为保持兼容。

### 状态/记忆集成验证

- 同一 persona、同一 idempotency key 只有一个 affect event 或 memory write。
- 不同 persona 可以使用相同 key 而不互相污染。
- event、snapshot、assistant message 和 effect 事务失败时全部回滚。
- PAD/drives decay 在精确边界、未来时间、跨重启、长离线和重复 checkpoint 下稳定。
- `memory_event` 只能由显式结构化调用触发，且提交结果可审计、可删除。

## 风险文件与回滚点

- Provider/contract 风险：`server/infrastructure/llm-provider.js`、`server/contracts/index.js`、`server/application/chat-production-adapter.js`。回滚点是关闭 structured control，只保留旧 text/tool/marker 归一化。
- Chat/SSE 风险：`server/application/chat-turn-flow.js`、`server/http/chat-turn-sse-adapter.js`、`web/src/composables/useChatStream.ts`。任何失败不得改变既有 terminal event 和 message alias。
- Persistence 风险：`server/runtime/startup.js`、新增 affect repository、`server/infrastructure/memory-repository.js`。只新增 migration，不修改旧历史 rows；失败时保留旧 schema 可启动。
- Commit/effect 风险：`server/infrastructure/conversation-commit-adapter.js`、capability registry/flow。必须先写可见 assistant facts，再在同一事务 apply accepted plans，重复 apply 依赖幂等。
- Policy/observability 风险：`server/runtime/runtime.js`、timeline/deferred/proactive flows、debug service。保持 persona scope、预算、屏蔽、静默和凭据脱敏。

## 完成前检查

- [ ] `prd.md`、`design.md`、`implement.md` 与最终 schema、migration、capability 名称一致。
- [ ] 全库搜索确认结构化 control、`memory_event`、affect/drives 字段、SSE alias 和删除路径没有遗漏的 producer/consumer。
- [ ] 运行 Trellis quality check，记录失败项和修复结果。
- [ ] 更新必要的 backend/frontend spec，确认工作树中只有本任务相关变更。
