---
name: ael-migrate-legacy
description: 基于 AEL inventory 和 plan，按批次只读审阅 Trellis 旧资料的分类、冲突与状态歧义，生成可审阅 proposal；不直接改数据库或切换旧流程。
---

从项目根目录使用仓库自带的 `./ael`。先读取 `./ael migrate scan --out REPORT` 与 `./ael migrate plan --inventory REPORT/inventory.json --out REPORT` 的产物。只选本次明确范围，优先活动任务及其引用，不一次注入全部历史。

读取原文，区分 spec、decision、research、reference、任务定义和旧结果。可只读核对范围内代码/测试，不能顺便修业务。保留旧字段、来源哈希和关键引文。历史 completed 不代表当前通过，in_progress 不代表存在活跃新 Agent。未知完成时间必须保留未知。

输出 proposal.json：`{"plan_id":"...","items":[{"source_id":"...","source_hash":"...","quote":"原文中确实存在的片段","action":"classify","kind":"spec|decision|research|reference","status":"current|historical|conflict","target_ref":"原plan的target","reason":"...","pending":"..."}]}`。任务歧义使用 action=task_status、kind=task，status 为 open/historical_completed/cancelled/conflict；未完成来源不能提升为历史完成。替代建议可加 superseded_by，使用明确的新知识 ID；程序拒绝断链、自引用和循环。每条必须可回指原文；不能包含 shell、任意目标路径或删除动作。置信度不能替代证据或授权。

运行 `./ael migrate validate --plan REPORT/plan.json --proposal proposal.json`。提交建议给用户审阅；确定性程序通过 `migrate apply` 落盘，Skill 自身不编辑运行库、不删除源资料、不批量改目标。源变更须重新计划。没有明确切换授权，只做只读扫描/预览，不 activate、不改真实 hooks。

明确的适用范围可用 paths 记录项目相对路径；禁止越界。需要解释旧术语冲突时可用 review_note 和 references（本计划其他来源 ID）附审阅说明，原文仍保留。当前规范继承不表示业务实现已重新验收。原始日志/JSONL/二进制作为历史产物保存，不能转换成通过证据。
