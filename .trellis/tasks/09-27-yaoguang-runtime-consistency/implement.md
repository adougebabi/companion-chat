# 跨子任务执行计划

- [ ] 逐一审阅并激活拥有实现工作的五个子任务，不直接 `start` 父任务。
- [ ] 每个子任务按自己的 `implement.md` 实施、运行测试和 Trellis check；跨层契约在子任务内同步生成 client/DTO。
- [ ] Context 与 Tool 子任务完成后，联合验证 opaque target ref、更新后 Runtime Context 刷新和 Tool 失败恢复。
- [ ] Tool 子任务的通用参数错误分类先于 Visual Identity 审查子任务；联合重放本次 `observations` 类型错误，验证 ADK 有界纠正及恰好一次有效提交。
- [ ] Life 与 Context 子任务完成后，联合验证一次临时事件跨日程边界、静默后新投影、summary/raw history 在下一次对话的语义。
- [ ] 最终检查五个子任务全部 AC1–AC7，审阅工作区原有改动未被覆盖，记录明确未复现的原始现场错误。
- [ ] 父任务完成时再做一次 backend/frontend 全范围质量检查及 spec 收敛。

验证命令和逐文件回滚点写在各子任务 `implement.md`；父任务只做联合验收。
