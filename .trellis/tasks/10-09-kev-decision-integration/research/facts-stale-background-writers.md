

## 2026-10-10 重复 current_facts_stale 与后台 Agent 插队

现场只读确认 turn_dcd23c98-5a81-47f0-b994-c1a1e4cab636 14:12:22.567—14:13:52.994，单轮无native Tool，settlement expected facts_gen_27724 actual27752。14:13:53启动GoalEvaluation在错误之后，不可作为本次根因。natural assistant INSERT被generation trigger豁免，输入inbox/userMessage在快照之前，不是自身receipt漏衔接。现有generation行只有计数/时间，无法事后精确归因这28个版本。

确定缺口两类：1. semantic Intention每消费processed fact只更新trigger_cursor_sequence，仍被fluctlight_intentions任意UPDATE触发facts推进；同值consolidated summary UPDATE也非必要事实变更。2. ProcessConversationSummaryIntent/processRuntimeSummary、ProcessConversationDailyMemoryIntent没有logical lease，独立Temporal Activity可以在chat持lease期间快照/LLM/提交；daily-memory真正写Memory authority。dispatcher优先级不阻止已启动Agent。不能仅凭代码缺口断言现场28次就是这些写入。

实施边界：复用现有per-Fluctlight durable logical lease补后台summary/daily-memory两个Core入口，覆盖snapshot→LLM→commit，保留同owner重入、不同Fluc并发、lease fencing/取消，不移动handleTurn即时接受新输入/supersede事务。新输入应立即终止旧turn，而不是被旧Agent锁挡住。Metadata cursor只在所有其余语义字段不变时跳过facts bump；真实Intention status/trigger/body/revision变化仍推进。Summary真实内容/来源/状态/日期分类变化保持现有CAS，仅避免完全同值consolidation UPDATE。inner-state已有独立revision但仍是事实权威，本轮不移除它的generation保护。

新增下一additive migration（0059，前head0058）在历史installer和0058之后执行：cursor-only排除、generation每次实际bump的有界来源journal（Fluc/generation/table/op/entityId可选/txid/time，禁止存业务payload；每Fluc最多256或512行），同事务记录并裁剪。现有one-arg bump API兼容，direct/owner/child/message/fact/schedule/fluctlight trigger准确记录来源；保持0058 embedding trigger移除及head-rerun不恢复旧过滤。发生currentFactsMismatch时把expected→actual区间来源按table/op/count放现有Owner诊断payload，bounded read，日志失败不覆盖原CAS错误，窗口不完整要明确不可全归因。没有第二Agent loop、自动忽略锁冲突或盲目重跑已提交业务。

验证：真实入口受控并发测试先RED：chat持同Fluc lease时后台summary/daily不得执行snapshot/Provider/commit；释放后执行；同owner重入与不同owner不阻塞；取消/lost lease保留fence。PG真实generation tests验证cursor-only及同值consolidation不推进、真实语义/来源更改推进、原CAS仍拒绝真实变化；empty→0059、0058→0059、head-rerun；journal同事务rollback/owner scope/retention与诊断区间。无隔离DB按用户约定SKIP，不安装DB、不写正式数据。纯入口与SQL契约/有界诊断解析测试应本地可运行。真实生产来源未追溯之处如实报告，升级须应用新migration。
