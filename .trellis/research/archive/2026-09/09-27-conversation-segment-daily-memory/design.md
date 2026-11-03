# 设计

## 两级来源模型

`conversation_messages` 仍为原始权威。阶段摘要是 source-bounded、可重建投影：沿用 ordered message refs/digest、immutable revision、Provider provenance，增加 Core-owned `started_at`、`ended_at`、冻结的 IANA timezone/local date及 `ending_state/open_threads/core_events` 的受约束模型输出。Provider仅收到该段原文和绝对时间；时间范围由 Core按 source消息计算。段窗口到最后完整 turn，静默 t0+10m debounce 触发；同一段来源身份决定 stable intent。新消息重排开放段的 timer，不改变已经封闭的 source window。异步 settle后继续 drain已达条件的 backlog。

当地日归并使用独立 `conversation.daily_memory` durable intent，冻结 conversation、Fluctlight、IANA timezone、local date、半开 UTC日界、阶段 Summary IDs/revisions/fingerprints。Turn按用户消息开始的本地日期归属，使跨午夜回复不拆半个 turn；持续聊天跨日时封闭前日最后完整 turn，必要的未完成部分留在下一窗口并显式标状态。日窗口按时区计算，不能固定24小时。旧固定块 Summary仅在整个来源窗口归属同一当地日时纳入过渡归并；跨日旧块要从原文重建日内段，不把整块误归某天；同一原文不被新阶段重复覆盖。

日记忆通过唯一的 `applyMemoryCommandTx` 创建 `episodic`类型，conversation/visibility保持原 scope；新增 `conversation_summary` frozen evidence kind与 source-link live/失效传播。模型只能归纳对话经历、结束与未完线索，不据此直接创建 semantic/relationship事实。Memory commit、source-link和阶段 `consolidated`状态在同一事务提交，或以可恢复的显式中间态保证“先成功落记忆再退出摘要”；保存 `consolidated_into_memory_id`。原文/阶段修正后失效旧日记忆 provenance，重算或追加 Memory revision，不让过时内容继续作为 verified。

Provider retrieval只选未归并且 source-valid的 active阶段摘要；日记忆进入原有授权 Memory检索，保留有界预算与内部 source-ref去重。已被日记忆覆盖的 raw若仍在近期窗口，避免同一次 Prompt无必要重复；摘要/日记忆因预算落选时 raw fallback仍可用。`historical_conversation`应带真正 source时段，而非仅 `completed_at`。

## 兼容与风险

对现有 `conversation_summaries` 做 additive列/状态迁移；旧 status/revision和 Temporal history仍可读取/重放。旧 `conversation.summary` workflow不得被新格式直接改变历史命令；新 intent或明确版本分支处理。新任务遵守同一 PostgreSQL权威+Temporal执行，不创建新 scheduler运行时。风险包括摘要事实污染、跨会话可见性、日界/DST、源更正失效及“先清后写”历史空洞；以上都进入数据库回归。

预计修改 `conversation_summary.go`、`provider_schemas.go`、`memory_provenance.go`/`memory_lifecycle.go`、prompt/working-memory投影、迁移、Workflow/Worker dispatch和对应测试。依赖 `09-27-wakeup-idle-cadence` 明确最后用户活动的 idle epoch；两个任务可共享时间事实但各自拥有独立 durable intent。需更新 Memory与Workflow规格中旧固定块/无 scheduler的合同。
