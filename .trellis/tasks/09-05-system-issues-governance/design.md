# 技术设计：系统问题治理（第一阶段）

## 设计边界

本任务覆盖工作流盘点、Googleapis CSS、媒体诊断、Redis LLM 调度、Redis TTL 反思/唤醒五个工作项；不实现提示词构成重构。设计遵守当前 clean-start 架构：PostgreSQL 保存领域事实和诊断状态，Temporal 保存可恢复 workflow history，Redis 作为事件/低延迟调度层。

Redis 相关机制不能成为唯一事实来源：Redis Pub/Sub keyevent notification 不是 durable event，ZSET member 也不能独立表达 payload、lease、ack 或恢复状态。因此所有 Redis 操作都必须由稳定 ID、PG 状态校验、幂等键和补偿扫描兜底。

## 工作项 1：现状盘点

产出任务目录内的盘点记录（必要时补充 `research/`），至少包含：

- 11 个 Temporal workflow 的 symbol、intent type、task queue、activity、生产者和触发时序；
- Dispatcher、workflow management、BFF diagnostics 和 Vue Diagnostics 的观测/控制边界；
- Googleapis CSS 的导入点、字体声明不一致、Vite 构建产物和部署运行时请求；
- Provider queue、diagnostic model run、media intent/event、reflection/wakeup 的跨层关系；
- 5 个工作项之间的实际耦合和不应扩大范围的边界。

## 工作项 2：Googleapis CSS

先通过构建产物和静态/浏览器验证确认 `Geist` URL 是否为必要依赖。默认方案是移除运行时外部 CSS，并保留显式、可控的本地系统字体回退；只有确认仓库已具备合法字体资产时才采用自托管。不得在没有授权/资产的情况下下载或提交字体文件。

验证内容：生产构建、静态 server 加载、network 中无 `fonts.googleapis.com`/`fonts.gstatic.com` 请求、中文/英文 computed font-family 和 Diagnostics/Chat/Settings 关键布局。

## 工作项 3：媒体诊断分层

### 数据流

```text
media intent
  → media workflow/activity
  → media_prompt provider run（若需要 LLM 改写）
  → media_intents.provider_prompt
  → media.comfyui.prompt_submitted event
  → media asset
```

在不改变 provider prompt 语义的前提下，给 model-run 记录保留语义 `role` 与 `binding_role` 的区分，并为媒体阶段提供稳定的媒体 intent/correlation 关联。优先复用现有 `media_intents.id` 和 `media:<intentID>` correlation；若无法直接扩展现有表，则增加受控的 nullable 关联字段或查询投影，不把大 payload 放入 Redis。

Core 查询应提供独立的媒体提示词结果（最近 20 条由 SQL 的明确 `ORDER BY initiated_at DESC, id DESC` 保证），BFF 只做 DTO 映射，Vue 在诊断中心增加独立区块。普通 LLM model runs 保持现有列表和过滤能力。

媒体诊断必须继续执行 `redactDiagnostic`，不泄露 quality gate 的图片 data URL；Visual Identity 的确定性 seed prompt 通过实际提交事件或媒体记录可见，但不能伪造 `media_prompt` LLM run。

## 工作项 4：Redis Sorted Set LLM 调度

### 边界

只替换生成类 Provider queue 的跨进程等待/claim 层，不迁移 `platform_workflow_intents` dispatcher，不改变 Temporal task queue，也不把同步 Provider 结果改为异步 API。

### 推荐数据结构

- `fluctlight:llm:pending:<binding>`：ZSET，member 为稳定 model-run/job ID；score 由 priority 主序和 queued-at/FIFO 组成，具体编码须避免浮点碰撞。
- `fluctlight:llm:processing:<binding>`：ZSET，score 为 lease deadline，member 为同一稳定 ID。
- `fluctlight:llm:job:<id>`：Hash/String，仅保存可恢复引用（model-run ID、assignment、scenario、request metadata），不保存不可恢复的业务结果。
- `fluctlight:llm:cancelled`：可选短期索引或 job 状态字段，用于 claim 前快速跳过取消。

claim/ack/requeue 使用 Lua 或等价原子事务：

1. 按 priority、queued_at、稳定 ID 选择候选；
2. 校验 job/PG model-run 仍为 queued 且未取消；
3. 从 pending 移入 processing 并设置 lease；
4. provider 开始/结束时回写 PG diagnostic model run；
5. 成功 ack 删除 processing；失败/超时按退避回到 pending；
6. 定期扫描 processing lease 到期项并 requeue；Redis 不可用时保留 PG polling fallback。

必须区分 API 进程和 Worker 进程的 consumer identity，避免同一 job 被重复执行；PG 的 deterministic model-run ID、状态 CAS 和 provider request timeout 是最终幂等保护。

## 工作项 5：Redis TTL 反思/唤醒

### 触发模型

- 用户 turn 完成后，在同一事务确认已写入可追踪的 reflection due 状态/intent，再设置短 ID Redis key：`fluctlight:reflection:due:<stable-id>`，TTL 来自当前配置。
- 当前没有独立 `product.reflection` 配置；第一阶段读取并复用 `product.wakeup.interval_seconds` 作为反思 delay，避免凭空引入新的设置协议。
- wake-up 使用每个 Fluctlight/稳定 workflow 的 key：`fluctlight:wakeup:due:<fluctlight-id>`；过期事件只负责通知 worker 尽快检查 PG due 状态。
- listener 收到事件后执行带幂等锁/状态校验的 dispatcher nudge；真正的 workflow/intent 创建仍通过 PG transaction 和现有 stable workflow ID。
- wake-up 成功并确认下一周期后，再设置下一次 TTL；disabled/inactive、pause/cancel 和 terminal 状态不得继续续期。

### 可靠性

- Redis 配置增加 `notify-keyspace-events Ex`，并实现重连、订阅恢复和 listener shutdown。
- listener 不是 durable queue：Worker 启动时和固定 ticker 执行 due/lease 补偿扫描，覆盖断线、Redis 重启和近似过期时间。
- reflection 继续使用 evidence window、watermark、running claim、state revision CAS；不得按 Fluctlight 无条件吞并多个 source intent。
- 现有 Temporal `WakeUpWorkflow` 的长期 timer 迁移前必须有明确替代和回滚点；第一阶段优先将 Redis 作为触发提示/加速，不直接删除 Temporal recovery 语义。

## 跨层契约

- Core HTTP/OpenAPI、BFF DTO、Browser Client 类型和 Vue store/view 同步更新。
- owner-only diagnostics、redaction、retention 和 correlation filter 保持兼容。
- 新字段使用 nullable/向后兼容默认值；数据库 migration 必须可重复执行，旧记录可读。
- Redis key 名称、TTL 单位、ZSET score 编码和 job payload 版本化，避免隐式协议。

## 主要风险与回滚

- 外部字体移除导致字体度量变化：保留 system fallback，构建后做关键页截图/浏览器检查，可独立回滚 CSS。
- 媒体诊断扩展导致旧 UI/客户端不识别：新增字段保持可选，旧 model-runs API 不删除。
- Redis queue claim/lease 出错导致重复或丢任务：保留 PG 状态机、reconciliation 和 feature flag，先在测试/单实例启用。
- keyevent 丢失或近似过期：补偿扫描必须先于功能开关，任何 listener 失败不得阻塞现有 PG/Temporal dispatcher。
- 如果实现过程中发现必须让 Redis 取代 PG/Temporal 事实来源，则停止本任务实现并回到规划阶段重新确认范围。
