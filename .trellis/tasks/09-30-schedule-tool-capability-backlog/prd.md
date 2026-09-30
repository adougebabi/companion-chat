# 日程工具、能力需求池与 Agent 运行错误修复

## Goal

让摇光能按需查询、创建和修改日程，并在缺少能力时给 Owner 留下可处理的需求；减少用户报告的 Tool、最终输出和回复发布失败，保留日程、证据引用及消息发布的安全边界。

## Background and confirmed facts

- `intention.schedule` 已注册，负责新增未来可执行意图及关联日程；`schedule.replan` 只接受自然语言 `intent` 并整体重排。模型侧没有独立日程查询或单项编辑/取消工具。`apps/core-go/internal/core/scheduled_activity_capability.go:13,28-49`；`schedule_capability.go:13-40`；`phase8_contract_matrix_test.go:21-38`。
- `schedule_replan_intention_link_unknown` 来自规划器输出的 `intention_id` 无法从当前 accepted schedule 找到有效 `action_plan`。没有本次模型输出，具体是新造/改写 ID 还是旧数据缺 plan 尚未确认。`apps/core-go/internal/core/scheduled_life_action.go:72-92`。
- `capability.request`、数据库与治理页“能力需求池”已有显式提交链路。未知工具调用直接返回 `capability_not_found`，不会自动建需求；实例卡片进入治理页时没有加载该列表。`apps/core-go/internal/core/capability_requests.go:13-44,169-180`；`adk_conversation_runtime.go:100-108`；`apps/web/src/views/InstancesView.vue:94-101`。
- `decision_influence_0_ref_invalid` 和 `_1_ref_invalid` 是 influences 数组对应位置的 `ref` 格式无效；`provider_context_ref_alias_unknown` 表示模型给了外形合法但本次 run 未注册的短引用。缺失 `appraisal.evidence_refs` 在 Provider schema gate 被拒绝，虽然后续 normalize 层会补空数组。`context_reference.go:445-486`；`provider_ref_codec.go:136-150`；`provider_schemas.go:119-130`；`cognition_growth.go:18-59`。
- `structured_response_invalid_json` 表示最终正式 Content 不是完整 JSON object；目前没有最终输出修复重试，且外层分类落到通用 `agent_run_failed`。`provider.go:596-623`；`eino_model_runtime.go:951-968`；`agent_run_record.go:35-73`。
- `uq_conversation_messages_turn_kind` 限定同一 conversation/turn/kind 只有一条消息。Tool ledger 按 OperationID 幂等；同一 turn 的不同 `conversation.reply` 调用会得到不同 OperationID，publisher 没有按 turn/kind 复用已提交回复，于是第二次会在 INSERT 时撞唯一索引。`migrations/runner.go:2270-2272`；`adk_conversation_runtime.go:147-165`；`tool_publication.go:82-145`。

## Requirements

1. 提供模型可调用的日程按需查询，返回可定位的事项及必要状态，不依赖把整份日程常驻提示词。
2. 支持新增未来事项，以及对指定事项改时间、改内容、取消；保留 immutable schedule version、已完成历史、当前/未来边界、CAS 和 linked Intention/action plan 约束。
3. 修复 `intention.schedule` 在 planner 发明或改写旧 `intention_id` 时的失败体验；遇到真实无效历史数据时给出明确诊断，不静默执行或丢失旧意图。
4. 缺少 tool/capability 的需求应进入 Owner 可见的“能力需求池”，包含可实现所需的用途、理由、期望契约和来源；工具缺失不应被报告为动作成功。治理页各入口均能加载并区分空、加载中、失败。
5. 对列出的最终输出、引用和结构化 JSON 错误建立可复现的修复与清晰诊断；允许有界纠错或重试，但不得凭空制造证据、接受未知引用或重放已经提交的 Tool 副作用。
6. `conversation.reply` 在同一 turn 的重复/并发调用应以确定性结果结束，不把数据库唯一冲突暴露为用户失败，也不产生两条 assistant 消息。

## Acceptance criteria

- [x] 模型能按需查询日程；查询结果能用于定位并改时间、改内容或取消指定事项，完成后再次查询可见新状态。
- [x] `intention.schedule` 与日程编辑在未知 planner link、丢 link、旧数据缺 plan、CAS 冲突和正在执行的 linked activity 上有明确且安全的结果；原 accepted version 在拒绝时保持不变。
- [x] 缺失能力请求在数据库和能力需求池可见，包含来源和实现所需信息；从两个治理入口进入均加载正确，空/失败状态可区分。
- [x] 所列 `decision_influence_*_ref_invalid`、`provider_context_ref_alias_unknown`、`appraisal.evidence_refs` 缺失和 `structured_response_invalid_json` 场景有自动化回归；可纠错场景恢复，无法安全纠错时返回具体错误且不伪造引用。
- [x] 同 turn 重复 `conversation.reply` 的串行、并发与重试场景至多保存一条 assistant 消息，并返回稳定 receipt 或明确的业务冲突，而非 SQL 唯一键错误。
- [x] 相关 Go、浏览器与集成检查通过；现有未提交用户改动不被覆盖。

## Out of scope

- 将整个日程重新常驻每次提示词。
- 放宽未知引用、未经授权的日程/Intention 修改、已完成历史改写或已提交 Tool 副作用重放。
- 顺带重做能力需求池的所有历史聚合、分页和审核状态机；仅修复本次请求所需的捕获与可见性。

## Risks and deferred evidence

- 真实模型输出与数据库状态尚未取得。实施时优先用诊断中的 final Content、planner proposal 和对应 accepted schedule 判定每次事故的具体来源；静态代码只证明触发条件。
- Provider 是否真正支持 strict JSON Schema 尚未从实际 endpoint 验证。若模型能力不达标，需在配置/preflight 与运行错误中明确呈现，不以弱解析伪装成功。
