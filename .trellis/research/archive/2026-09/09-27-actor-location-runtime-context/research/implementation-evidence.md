# 实施与验证记录（2026-09-28）

## 已实现

- Provider `current_state` 带 `subject=actor_self`；运行协议、AuthorityRule与 `scene_event` Tool描述明确用户第一人称地点不等于摇光当前地点。去掉普通场景Tool无法表达的 `context_override.explicit`要求；图片覆盖仍沿用该字段。
- Conversation/Wake-up/Takeover 的最近结果过滤已由当前状态/已发布回复吸收的完成态 `affect_event`、`conversation.reply`和空聚合行；重复失败只保留最新，成功后的旧失败不再冒充当前，待完成外部结果保留时间。Reflection等其他surface维持自己的投影。
- Model-facing reply/affect回执缩短；同轮已发布回复只在**本次ADK trace有对应目标消息ID**的续调 outbound Prompt中排除。新的独立重试仍能看到已提交回复，Core refreshed Projection完整保留原消息。
- Provider外貌去除未知字段、revision/captured元数据，保留已知值、显式cleared状态及穿着Tool目标；内建零压力drive省略，自定义typed drive保留描述。对话/wake-up的current_state/self_actor/current_speaker为第一层必需事实，超限显式失败。
- cognitive runtime、life-world、memory规格已同步Actor主体、outcome选择、Tool回执和critical预算合同。

## 已验证

- `go -C apps/core-go test ./internal/core ./internal/ai/... -count=1`、`go -C apps/core-go vet ./internal/core ./internal/ai/...`、`git diff --check`通过。
- 隔离PostgreSQL定向回归：场景Event影响下一Projection、真实两轮conversation.reply Tool adapter、wake-up affect+reply、双地点Provider wire和outcome/appearance/drive/critical-budget fixtures通过。
- 独立只读复核指出最初按turn_id移除同轮回复会误伤新重试，以及零压力custom drive不该省略；均已改为trace精确消息ID及built-in-only省略，并补回归。

## 联合验收限制

- 真实模型是否仍误判中文地点主语需要用户允许的现场/受控真实Provider验证；当前测试证明Prompt主体标记、Tool契约、状态传播与Core权限，不声称模型语义零失误。
- 父任务最终统一跑完整数据库套件并做实际Runtime Context大小对比；本子任务尚未单独提交或归档。
