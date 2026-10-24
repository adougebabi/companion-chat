# v6.2-r5 连续发布与衣柜契约修复

基线 `3cf563be`。依据用户 r5 报告和 `r5-diag.tgz`，包 SHA-256：
`04c9f6133b5613a3af2b4b7a9caf4d8be02cf30f86da4149c2c3e822b73a17e1`。
本轮直接修复，不创建任务。保持初始化单次、无应用 Token 上限、
LLM 并发 1、准备环节串行；没有引入动态频率配置。

## 22 条动态的证据与根因

`moment-loop-runs.json` 共 67 条物理调用。原认知 correlation 有 45 次调用：
21 次原生 moment_publish、24 次 capability_catalog、0 次 conversation_reply；
另有交错的 21 次独立 Writer 记录，最后 1 次 Moment 请求属于新的回合。
报告观察到 22 条可见动态。这个导出未包含每个 Tool receipt / Moment ID，
不能仅由 Provider 调用逐条证明数据库已提交；用户观察与实际运行机制一致。

三个代码条件叠加造成重复：

1. 私聊结束规则要求真实 conversation.reply；Moment 的 completed 回执不能代替私聊消息。
2. Moment 成功后，原规则仍强制选择整个工具目录，没有把已完成发布切换为原回复的收尾。
3. 每个新模型轮次生成新 operation ID；同一来源再调用 Moment，会重新运行 Writer 并形成新的发布命令。

这是一条受请求取消/超时限制的反复发布路径，不能把 300 秒后的失败当成正常停止机制。

## 修复边界

- 同一私聊来源的隐式动态发布动作使用稳定的业务 operation identity，排除模型轮次和
  原生调用 ID。保留 Core 明确区分的 action、显式 Owner target 和自主任务自己的标识。
  新用户来源可以发布新动态。没有按正文相似度或时间窗口全局去重。
- 空、null、空数组的可选 evidence_refs 统一为无证据选择；非空参数不被吞掉。
  原有持久化 Tool ledger 在 Writer 前复用已提交结果；同一操作改变真实命令仍冲突。
- Moment 提交后，下一次私聊请求只保留实际 conversation.reply，用它收尾原回合。
  若该回复实际需要媒体动作，保留其动作目录并排除已满足的 Moment 发布。
  已接受 waiting 继续原异步续接；needs_action 本身不算回复完成。
- 若 Provider 无视收尾目录再次选择 Moment，在原生调用执行前结束为明确失败，
  不再发布第二条。真实发布回执仍保留，失败不会伪造私聊回复。
- 协议校验兼容注册表中真实的 underscore/dot Tool 名称映射。

## 衣柜证据与修复

原始 `wear_item_refs` 缺失错误与后续修复错误被串在同一错误链中，容易误以为
修复没有采用。逐条复放显示空数组一直存在：多份修复实际失败于额外的
evidence_refs / influences，或 clothing_type / key 等字段仍不合法。
其中一份合法空数组修复已通过 schema，最终失败于没有可执行方案。

- 通用 final repair 原文要求“空 evidence_refs 或 influences”，与衣柜的封闭 schema
  直接冲突。改为只返回 schema 允许的字段，保留有效原产物，补齐必填项。
- 衣柜任务文字明确四个顶层字段和衣物、方案的实际键名，以及生成时 wear_item_refs=[]、
  select_only 时 items/plans=[] 的阶段契约。仍由独立 wardrobe.wear 建立实际穿着。
- 说明已有授权是虚构初始库存；未知性别、尺码、身体测量保持未知，并非准备库存的前置条件。
  代码不生成通用衣柜兜底，不虚构身体属性，也不把空方案算成功。
- 分别保存超时、长度终止、字段契约错误、非法 JSON、未提出方案等原因，
  准备页读取原 failure_reason，避免全部显示“格式不完整”。

## 验证与限制

真实隔离 pgvector PostgreSQL + 正式 HandleTurn/Eino/Tool ledger/Writer/发布结算回归：

- Generate 与 Stream 下：一次 Moment、一次私聊回复；两次认知请求，分别运行对应 Writer。
- Provider 故意再次选择 Moment：一个 Moment/outbox、没有假回复、两次认知后明确失败。
- 同一来源跨新原生 ID、物理轮次及重建的运行上下文：不再次运行 Writer，返回同一已提交 Moment。
- 新来源可生成另一条动态；显式 action/target 与自主作用域不被隐式私聊规则覆盖。
- 保留未提交/accepted、needs_action、waiting 的差异。
- 真实衣柜初始缺字段 → 一次修复含空 wear_item_refs → 库存提交 → 另一次真实选衣/换装 → completed。
- 空方案保留失败且无新库存；本地复放 6 份原始衣柜修复证明空数组保留并准确区分 schema 失败。

上述相关测试及 race、Go vet/build、Web 399 项均通过；工作区类型检查、
客户端生成同步、生产构建通过。没有新增迁移或 HTTP 契约变更。
Go 全量仍有此前已记录的 6 项基线失败，见 r3/r4 记录。
附件原始模型内容只在本机诊断目录复放，没有提交私人内容。

没有真实调用用户模型或操作测试库之外的数据；没有删除已经产生的动态。
仍需同版 Web/API/Worker 复测“能发条动态吗”及初始衣柜完整准备。
此次修复没有证明所有模型延迟、地点语义或字段标签问题已解决，
也不由 finish_reason=length / 8192 推断限制来源。
