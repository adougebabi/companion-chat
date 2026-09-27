# 对话阶段摘要与日记忆归并

## Goal

静默阶段摘要、当地日归并、来源验证与历史连续性

## Confirmed facts

- 当前 Summary 是含精确原文 refs/digest与 revision的对话投影，按 24-message reserve、40-message chunk/20 assistant回合/约6000 tokens创建；没有静默 timer、日界归并或自动追赶全部 backlog。
- 当前长期 Memory的 frozen source不接受 conversation summary；来源存在不等于摘要语义正确。直接把 summary当永久 semantic/relationship事实会污染记忆。
- 今天已提交 raw候选扩容、Summary入选后按完整 turn去重及 raw fallback；本子任务只验证并衔接，不重复实现。证据见父任务 `research/confirmed-paths.md`。

## Requirements

- S1：最后用户活动后静默达到 10 分钟时，对至最后一个完整对话回合的未覆盖区间生成阶段摘要；新消息到来前的静默分界稳定可重放，不把旧摘要递归当阶段摘要输入。
- S2：每段展示实际消息的起止时间、核心事件/变化、结束状态与未完成线索；时间范围由 Core从来源消息确定，不把摘要 `completed_at` 当结束时刻。
- S3：当地日期更替后，把同一授权会话内前一天所有有效阶段摘要归并为一条可追溯的 `episodic` 日记忆；跨午夜的完整回合按冻结的本地日期政策归属，不跨权限范围合并。
- S4：日记忆成功提交与阶段摘要退出活动 Prompt必须原子或可恢复；原始消息更正/摘要版本变化时日记忆 provenance失效并可重建，不自动晋升为稳定偏好/关系事实。
- S5：保留长对话的覆盖连续性与可诊断预算裁剪，修复 Summary backlog在会话停止后不再追赶的问题；旧 Summary和历史数据兼容。

## Acceptance criteria

- [ ] 10分钟静默产生一次 source-bounded阶段摘要；静默期间重复 timer/Worker重放无重复；新聊天形成新的阶段，不重写已封闭阶段来源。
- [ ] 摘要含真实起止时间、阶段结束与未完线索；不把聊天计划写成已执行行为，不把 `completed_at` 或服务器UTC日当对话日期。
- [ ] 同会话一个当地日多个阶段归并为一条 episodic日记忆；成功后这些阶段不再作为 active summary重复注入，原文/阶段/日记忆来源链仍可查。
- [ ] 日归并失败时阶段摘要仍active；原文更正、摘要 invalidation/revision及重试不会留下已验证的过时日记忆或重复日记忆。
- [ ] 70+ 回合、跨午夜、DST及旧固定块场景的选中 Summary/日记忆/Raw覆盖和预算丢弃原因可检查；当前消息只发送一次。

## Constraints and out of scope

- 日记忆先按每个会话、每个摇光当地日归并，不跨会话自动合并；其他 typed Memory仍由现有记忆生命周期维护。
- 阶段摘要“清空”表示退出活动检索、保留来源/审计行，不物理删原文或投影记录。
- 不用新的队列运行时或 Go关键词规则总结聊天；不重做 `09-27-context-summary-continuity` 已完成的 wire/raw修复。
