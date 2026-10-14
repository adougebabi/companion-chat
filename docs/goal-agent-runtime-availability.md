# Goal Agent 频率与提交可用性修复

本轮在 `codex/fact-only-reply-vision`、基线 `ec1ed82` 上完成两项追加修复：Goal Evaluation 每实例按 30 分钟周期准入；Goal Planner 必须有真实空位。评估原生提交中的确定性问题由程序处理，持续无进展时正常收束，保留真实成功结果与待处理证据。

## 频率规则

| 路径 | 当前规则 |
| --- | --- |
| Goal Evaluation | 同一 Fluctlight 共用持久化周期；两次周期启动至少间隔 30 分钟，不按 Goal/profile 分别增加频率 |
| 新消息、证据、反思、Tool 请求 | 积累/合并证据，不提前下一周期 |
| pending、retry、冻结 snapshot、旧 SQL producer | 实际 claim 都重查 available_at 与同一周期；等待不消耗失败 attempt 或证据 |
| 无新事件 | 既有 repair 生命周期补持久 due 工作；相同事实 memo 可跳过无意义模型调用 |
| Goal Planner 满额 | 所有入口都不调用模型，包括 Owner、review、旧队列、重试和恢复；返回 capacity_full，保留事件 |
| 正式完成释放空位 | 精确完成事件获得一次补位 episode，可合并同批完成事件 |
| 每日规划 | 有空位且捕获到合法当日日规划/初始化事件时，本地自然日一次 |
| Actor/profile/关系/wish、暂停/取消/放弃 | 保留上下文，不独立创造立即补位或每日预算 |
| 自动 Planner 失败 | 同日/同一完成事件的实际准入预算保留；新完成、下一日或明确 Owner 新请求才有新机会，仍须有空位 |

周期和 episode 使用数据库时间、现有 lifecycle/Life 锁及独立 Agent 准入。30 分钟作用于一次评估周期；同一原生 Tool 循环内的多次物理模型请求属于同一轮。调度可以因排队/Worker 忙而晚于到期时间，不能早于持久化周期。程序不会持有 logical lease 睡眠等待下一轮。

Planner 以实例实际 active Goal 数量与 `max_active_goals` 比较；Stage/Commitment 不占目标名额。日规划不是日期变化自动获得预算，必须有可见、watermark 内、未消费的合法当日事件：`schedule_accepted_daily`、`daily_cadence_due` 或既有 boot/enable/recovery。旧日程不能冒充当日事件；普通 hint 可唤醒已存在的合法机会，但不能额外制造预算。

## 用户报告的提交问题

原先缺少根评估时持续 forced Tool，底层生产循环没有“连续无进展则收束”机制。非 retryable 的业务拒绝依然交回模型继续决策，因此外层五次 claim 上限无法结束卡在一轮内部的重复调用。恢复输入还丢失了 accepted submission 的类型，使 plan 和根评估难以区分。

| 原表现 | 程序处理 |
| --- | --- |
| 已 accepted 的 plan 换 payload 再提交，goal_submission_conflict | 复用原已提交回执，标明 replayed/ignored_payload_change，不重写原 digest、不做第二次变更 |
| 空/全 unknown judgment 搭配 no_change | 在受信 native adapter 中归一为 needs_evidence，保留 unknown；缺等待状态时补最小技术性等待，不增加 satisfied |
| criterion_ref 首尾空白、完全相同的重复 judgment | 在冻结目标/对象范围内确定性规范化、合并同义证据并去重 |
| 未知、跨 Goal、跨对象或相互矛盾的 criterion | 正常 handled/awaiting_evidence 回执，assessment_accepted=false、mutation_committed=false，给出合法 refs；原目标、进度、判断和证据不改 |
| 持续重放已 accepted plan 或错误调用，无新 accepted target | 连续三次 native 尝试无真实新进展后，关闭本轮 Tool 并正常收束 |
| Provider 无视空 Tool catalog，仍返回 ToolCall | Generate/Stream 都用专用操作性等待总结收束，违规调用不执行；不依赖模型配合终止 |

accepted 恢复输入稳定排序，显式提供 `capability_name`、`submission_kind=root/plan/object` 和冻结短引用，不会把 plan accepted 当成根评估已完成。

所有回执重放仍检查私有 claim、projection、live Goal revision 和源 fingerprint。程序兜底不放开普通聊天/WakeUp 的完成权限，也不放松真实证据、授权或 CAS。无证据的 satisfied、过期来源和跨实例操作继续由原规则处理；真实 Provider、数据库与取消错误保留原原因。

## 收束与排查字段

循环达到无进展状态后，在最终请求前按 claim CAS 写入 `cycle_disposition=awaiting_evidence`。最终请求只带原 summary grammar，移除 `tools`、`tool_choice` 和 `parallel_tool_calls`。原物理响应、ToolCall 与 usage 的诊断先记录，随后才执行程序适配；不会把程序总结冒充 Provider 的原始响应。

正常处理完这一轮时，结果可为：

```json
{
  "status": "succeeded",
  "disposition": "awaiting_evidence",
  "assessment_complete": false,
  "unresolved_goals": ["internal goal identifiers in Owner diagnostics"]
}
```

这是结果字段结构示意，不是生产采样。`succeeded` 表示这一轮已处理完；`assessment_complete=false` 与 `unresolved_goals` 明确仍有目标没有有效根评估。它不表示目标已完成。未解决的共享来源仍 pending，不生成对应成功 memo、完成事件或伪造的 native 提交。成功兄弟保持原提交；同一 workflow 重放直接返回已处理结果，不再重开模型循环。真实 stale authority 保留已有 fresh-snapshot 路径。

重复 accepted target 的回执有 `replayed`、`ignored_payload_change`、`new_mutation_committed=false` 和 `unresolved_goal_refs`。安全待处理回执有 `handled`、`disposition`、`assessment_accepted=false`、`mutation_committed=false`、`reason` 和 `allowed_criterion_refs`，可区分技术处理成功与目标评估/完成。

## 持久化与升级

新增 additive head `0063_goal_agent_cadence`，前一 head 为 `0062_reply_media`：

- `goal_agent_cadence` 保存 evaluation 下一周期及请求身份，按 Fluctlight 共享。
- `goal_planner_admissions` 保存 `daily:YYYY-MM-DD` 或精确 `event:seq` 的已准入记录，包含 run_ref/admitted_at；不会用全局水位吞掉未激活 private profile 的旧事件。
- Runner 在旧 released installers 之后安装新规则；0054/0055 等旧 SQL 未改写。升级保守继承已有尝试的 operational 时钟/预算；head 重跑不能重置它们。

此构建需沿项目既有迁移流程到 0063 才能使用。此次没有运行生产迁移、push、部署、操作真实 workflow 或改写业务数据。旧运行中的模型循环也没有被代理取消；运行环境换用修复构建后，需由用户按既有运行管理流程验收。

## 验证与边界

已实际执行：

- accepted kind 原实现 RED；修复后 GREEN。
- unknown/ref duplicate/changed-payload replay 的旧行为敏感性 RED；恢复后 GREEN。
- 禁用新无进展处理时，重复 plan/坏 criterion 原生 HTTP 场景超过第九次请求触发测试失败；新行为有限结束。
- 禁用不合作 Provider 的 ToolCallFallback 时，HTTP 场景同样达到上限失败；恢复后 Generate/SSE 正常收束，违规 ToolCall 不执行。
- 连续不同 target 有实际进展时不提前截断；真实 Provider503、取消及原工具/最终合同/无工具修复矩阵通过。
- 最终 `go test ./...`、Core/Workflow race、`go vet ./...`、`go build ./...` 通过；Go PASS 事件 1787、SKIP 事件 542，无 FAIL。计数包含父/子测试，不能当独立验收数量。
- `pnpm typecheck/test/build` 通过；Browser Client 21、Core Client 2、Web 105 项全通过。构建保留原有大 chunk 提示。

**PostgreSQL 场景尚无执行证据。** 测试 URL 未配置，本机常见路径没有 PostgreSQL/pgvector，Docker daemon 也未运行；没有安装或启动服务。周期边界、并发原子性、真实迁移 fresh/0062升级/head rerun、持久化重放/source 保留用例均明确 SKIP。部分父测试容器显示 PASS，不能算数据库内部用例通过。真实 Provider/Temporal/Redis/线上恢复均未执行。

完整命令、日志摘要和逐项状态见任务 `verification/goal-runtime-check-summary.json`。当前代码为本地未提交改动，等待审阅提交。
