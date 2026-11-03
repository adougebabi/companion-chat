# 技术设计：交互与 LLM 请求调度优化

## 1. 边界与目标

本改造覆盖 `apps/core-go` Provider/诊断/设置、`apps/gateway-go` 浏览器边界、`packages/browser-client` 合同与 `apps/web` 聊天/设置/诊断界面。Temporal 工作流仍负责 durable intent 的生命周期；新增的 Provider 队列负责所有模型 HTTP 调用的并发、优先级和可观察状态。两者不互相替代。

队列在每个 Go 进程内运行（API 进程和 Worker 进程各有一个 Provider 队列），并通过 PostgreSQL 的诊断记录提供跨进程、跨刷新的一致观察结果。生成式和 Embedding 使用独立队列/并发槽，避免向量化挤占聊天回复。若未来需要跨进程严格全局公平调度，可在此边界替换为专用队列服务，不改变 Provider 调用合同。

## 2. Provider 角色与场景

### 2.1 两个绑定目标

- `generic_llm`：所有生成式请求的唯一绑定目标。
- `embedding`：Embedding 请求的唯一绑定目标。

Provider 的公共方法仍接收业务调用方传入的场景角色，以保持领域代码可读；`assignment` 将所有非 embedding 场景映射到 `generic_llm`。迁移时从既有业务角色中优先复制 `action_realization`，其次 `cognitive_assessment`、`reflection`、`initialization`、`media_prompt` 的首个可用绑定生成 `generic_llm`；旧行暂时保留以便回滚/审计，但设置 API/UI 只返回两个新角色。若 `generic_llm` 尚未存在，assignment 按同一优先级读取旧行作为兼容回退，并在诊断中标记实际绑定为 `generic_llm`；不会跨到 embedding。

### 2.2 触发场景元数据

通过 context-scoped `ProviderScenario`（默认由业务角色推断，特殊调用显式覆盖）传入诊断和队列：

| 场景 | 触发位置 | 优先级 |
| --- | --- | ---: |
| `reply` 回复生成（含自治动作的可见回复） | `StreamText`/`Text` realization | 100 |
| `cognitive_assessment` 认知判断（含日评、计划） | conversation/autonomy/schedule | 90 |
| `native_cognition` 原生事件认知 | native life/cognition fact | 90 |
| `media_prompt` 媒体提示词 | media intent | 80 |
| `reflection` 反思 | reflection workflow | 70 |
| `wake_up` 唤醒 | wake-up workflow | 70 |
| `initialization` 初始化 | creation analysis | 60 |
| `embedding` 向量化 | memory embedding/retrieval | embedding queue FIFO |

所有 Provider 调用都在入队前生成 correlation/request ID；诊断 `role` 为 `generic_llm` 或 `embedding`，新增 `scenario` 保存上述真实触发场景。

## 3. 队列执行模型

新增 `provider_queue.go`：

- `ProviderQueue` 用互斥锁、条件变量和优先级堆保存待执行任务；堆排序为 priority DESC、enqueue sequence ASC，保证同优先级 FIFO。
- 生成式队列和 Embedding 队列各自有 `maxConcurrency`，默认分别为 2 和 1，安全范围 1–8。队列 worker 数量固定为安全上限，运行时只允许当前运行数不超过设置值。
- `Submit(ctx, class, scenario, fn)` 在入队前写入 `diagnostic_model_runs` 的 `queued` 行；取得槽位后原子更新为 `running` 并设置 `started_at`，执行函数后更新 `completed`/`failed`/`cancelled`/`timeout` 与 `completed_at`。
- 调用方 context 在排队时取消会从堆中标记取消并更新诊断；执行中取消通过已有 HTTP request context 传播，释放槽位后唤醒下一个任务。
- 队列不吞掉业务错误；Provider 的现有结构化/流式/Embedding 解析和领域重试规则保持不变。
- 并发设置每次新任务读取 `runtime_settings` 的 `llm.queue` 值（缺省回退默认值），因此 Settings 保存后无需重启；`UpdateSettings` 校验未知字段、数字类型和范围。

## 4. 诊断持久化与 API

`diagnostic_model_runs` 增加兼容列：`binding_role`（默认由旧 role 推导）、`scenario`、`priority`、`queue_status`（或复用 status）、`queued_at`、`started_at`、`completed_at`。为避免 queued/running/completed 产生多行，记录 ID 从 role/binding/scenario/correlation/prompt 派生且不包含状态，状态更新使用同一行 upsert。

Core 查询返回新字段；BFF `browserDiagnosticModelRun` 明确映射 camelCase；浏览器客户端类型同步更新。`status` 对外值采用稳定英文枚举（`queued`, `running`, `completed`, `failed`, `cancelled`, `timeout`），Vue 负责显示中文“排队中/执行中/已完成/失败/已取消/超时”。Prompt/Response 继续经过现有脱敏逻辑，排队记录 response 为空。

## 5. 聊天流修复

- `ChatView.send` 在确认提交非空文本后立即 `draft.value = ""`，失败时依赖 retryTurn，不将已提交内容重新放回输入框。
- composer 改为 grid/flex 单行结构：textarea `min-width:0`，操作区固定宽度；移动断点只缩小间距和按钮，保留触摸目标。
- conversations store 对 NDJSON 使用 `TextDecoder` 的流式解码和最终 flush，要求收到 `completed` 才刷新服务端消息页；缺少 terminal 或 sequence/turn 校验失败时清除 assistant 草稿并保留可重试状态。每个 token 触发响应式文本更新并保持自动滚动。
- BFF/Core 继续发送每帧换行和单调 sequence；队列等待阶段不发送伪 token，HTTP 连接仍可取消。

## 6. 反思/唤醒与恢复

代码注释和诊断场景明确：唤醒产生一次内部生命事实（attention/thought/desire/agency），并创建稳定的 reflection intent；反思消费 cognition inbox 的 processed evidence window，应用候选记忆/自我/关系更新。唤醒不是反思的别名。

复核并补强 `wake_up.current` 的 terminal reconciliation：active/paused Fluctlight 的 failed/completed workflow 重新置为 retry；稳定 intent/cycle ID 和 `cognition_wakeups` 唯一键保证重复调度只返回既有结果，不重复事实、动作或 reflection intent。新增单元测试覆盖 Provider 失败、Temporal 终态和重复 cycle。

## 7. 兼容性、回滚与风险

- 迁移只使用 `CREATE/ALTER ... IF NOT EXISTS` 和幂等数据复制，不删除旧角色行；回滚代码时旧行仍能被旧版本读取。
- 队列是内存调度、诊断是持久化观察；进程崩溃后 queued/running 记录由启动恢复/定时清理标记为 failed 或 retry，不能永久显示执行中。实现阶段要选择明确的 stale timeout，并加入测试。
- 所有新增列必须同时更新 migration schema、Core 查询、BFF DTO、browser-client 生成脚本和前端类型；避免单边字段漂移。
