# Summary Workflow 规格摘录与本次变更

- `.trellis/spec/backend/fluctlight-workflow-contract.md:714-763`：现有 `conversation.summary` intent以 Fluctlight、conversation、inclusive source range、digest和有序 refs为稳定身份。Activity在Provider前后重读原文；旧Summary不是阶段摘要输入。它明确说“无调度器”，和用户要求静默触发、当地日归并相冲突，因此要用同一Temporal/PG运行时新增durable intents并更新合同，旧Workflow历史须保持可回放。
- 同文件 `:763-812`：重复identity重用；changed payload拒绝；source gap/digest drift不能提交；Provider/Worker失败保留Raw并重试。
- `.trellis/spec/backend/fluctlight-memory-contract.md:267-315`：Working Memory是有界读模型；Summary是可重建投影；Raw候选最多200，已入选Summary才对完整Raw turn去重，未入选时Raw fallback仍竞争预算。
- 同文件 `:385-413`：Memory revision来源绑定真实kind/ID/revision/fingerprint；来源失效时Memory不得继续当已验证。日记忆必须经现有Memory lifecycle，不能把每条聊天复制成长久记忆。
