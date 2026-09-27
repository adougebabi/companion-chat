# 跨子任务设计

父任务保存源要求与联合验收，不直接修改产品代码。Go Core继续拥有 Actor/Life Context、Memory、Workflow和 Agent 状态；Provider只判断语义并给出受约束输出；Browser仅展示Owner已授权读投影。避免用一个全局提示词或序列化开关掩盖来源、时序与诊断缺口。

## 子任务边界与顺序

1. `09-27-agent-failure-diagnostics`：R6，先让逻辑 Agent/Tool失败可定位，为后续回归提供证据。
2. `09-27-actor-location-runtime-context`：R1/R3，明确摇光/用户地点归属并删去已由当前事实吸收的 outcome/receipt重复。
3. `09-27-media-capture-prompt-format`：R4/R5，恢复媒体规格中的自摄与显式视角规则，减重媒体专用投影和多模态结构化文本。
4. `09-27-wakeup-idle-cadence`：R7，建立用户消息驱动的绝对 idle epoch和取消栅栏。
5. `09-27-conversation-segment-daily-memory`：R2，在同一 idle事实之上使用独立摘要/日记忆 durable intents。它依赖第4项的时间基点契约，但不能复用 Wake-up业务 cycle。

前3项各自可独立验收；Wake-up先于摘要日记忆，因后者需要明确“最后用户活动”与跨日时间语义。`09-27-context-summary-continuity`今天已提交的 raw fallback/wire去重是本任务基线，不归本父任务重复实现。现有 `09-27-yaoguang-runtime-consistency` 的其他子任务代码不由本任务覆盖。

## 数据与格式的共同约束

- 场景当前事实带 `actor_self`主体；用户地点是另一 Actor的语义事实，不写进摇光 Life Context。场景Tool仍需真实提交，不能仅靠文本声称地点变更。
- Raw消息、阶段摘要、日记忆依次是权威→可重建投影→持久 episodic Memory。每层保留 source identity/revision，日记忆成功且来源链接持久后阶段摘要才退出 active Prompt。跨会话不自动归并。
- Runtime Context的体积优化先减少重复事实、未知值与无用 outcome，再决定 TOON表格化；原生 Tool协议JSON不改。媒体 Provider文本单独使用紧凑 TOON，并保留冻结 concept的完整 Core副本。
- `agent_runs`与单次 `diagnostic_model_runs`保留各自真实状态；页面在同一 correlation下显示层级关系。安全错误信息沿用Owner-only/typed-redaction/限长合同，不进公开聊天。
- 计时采用最后一条已接收user消息作为 `t0`。Wake-up节点绝对相对 `t0`；阶段摘要静默 debounce同样引用该用户活动事实，但用独立稳定intent。当地日界按冻结的摇光有效IANA时区计算，不使用执行时日期。

## 兼容与回滚

数据库与API变更尽量 additive。旧摘要、旧 Agent诊断行、旧 Wake-up clock/Temporal history均需有明确兼容读取或版本分支。新阶段摘要或日记忆任一转换失败时旧 Raw/active Summary继续可用。各子任务单独测试与提交；父任务最后做跨子任务覆盖、取消竞态、预算、权限与浏览器展示联测。
