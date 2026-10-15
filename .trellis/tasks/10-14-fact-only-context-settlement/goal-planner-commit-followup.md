# F19 Planner 提交可修复参数与复核闭环

用户继续报告 apply 的 goal_planner.commit：context_review_required 被放入 dependencies 后误报 resource is not authorized；去掉后 needs_input，未 durable commit。沿用已批准实施任务及 HEAD 42a2d7c，不重做 F18。

## 证据与边界

- dependencies 是 Goal ID → prerequisite Goal IDs；context_review_required 是 Goal 布尔 hold，真正清除入口 reviewed_goal_ids 已实现。普通标记不能被当成外部资源或自动当作复核完成。
- 初始目标空 success_criteria 且 needs_reflection=true 合法；Planner 不支持修改既有 criteria。报告中的暗梗、照片、关系阈值为 Agent 建议，不写成用户标准。
- 第二次只有 disposition，缺原始调用及 warning_code，无法唯一确定。当前缺技术 fence 返回 planner_snapshot_required；领域裸 ErrInvalidArguments 返回 goal_plan_rejected，均丢失排查抓手。

## 最小修复范围

1. 私有 Planner 提交中识别精确的 context_review_required 表示误用，告警并跳过该伪 dependency。仅有伪标记的 entry 不替换原有真实依赖；不自动增加 reviewed_goal_ids、不清 hold。其他真实 Goal refs 仍完整授权检查与原子执行。
2. Core 为 Planner 初始请求已提供的 snapshot 保存可信 fence，在真实物理调用身份下补缺省技术字段。后续优先取已消费的更早 snapshot Tool receipt，同批新 query 不可借用；显式 stale 值不升级。无需模型重复抄写已知版本，不读取最新 DB 为旧意图换 fence。
3. 领域参数拒绝给具体代码和确定性改正信息；ADK 和 Planner final 保留安全 warning/disposition，使 handled 不能再成为无原因 needs_input。
4. 明确 Tool schema/指令：dependency 只接受真实 Goal IDs，复核专用字段 reviewed_goal_ids；复核既有目标不需要 changes/source/create/criteria。
5. 纯逻辑/真实 native 边界及临时 PG 验证：review-only 真正提交，伪 dependency 不清真实依赖/hold，foreign/profile/claim/stale/DB 故障不掩盖，幂等无重复写入。

不扩展 Planner 的目标修改权，不改 cadence/capacity，不访问用户生产 DB，不 retry/cancel 真实 workflow，不使用真实模型。只在任务独占 /tmp 中创建/清理测试 PG。
