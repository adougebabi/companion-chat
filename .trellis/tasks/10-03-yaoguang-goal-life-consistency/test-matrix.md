# A01–I01 验收映射

详见 [实施报告](../../../docs/fluctlight-goal-life-consistency-report.md)。

| 编号 | 状态 | 测试/证据 | 断言及限制 |
| --- | --- | --- | --- |
| A01 | 确定性通过 | goal_life_chain_test.go | 初始化共享关系目标进入持久化与下一步投影 |
| A02 | 真实行为阻塞 | validation-summary.json（原始日志已清理） | 模型服务拒绝连接；未证明自然关系探索语义 |
| A03 | 部分通过 | proactive_topics_test.go / sleep_cycle_test.go | 重复主题与睡眠门禁通过；明确拒绝的长期模型行为待验证 |
| A04 | 部分通过 | reflection_runtime_v2.go / evolution_persistence_test.go | 复盘来源与下一步权威有契约；错失机会的模型语义未实测 |
| B01 | 确定性通过 | actor_facts_test.go | 同楼纠正、常驻背景、来源派生退出当前事实 |
| B02 | 部分通过 | actor_facts_test.go / runtime_summary_test.go | 重载/源失效/晚提交/摘要纠正输入通过；多轮真实模型跨天待测 |
| B03 | 确定性通过 | actor_facts_test.go | change保留历史有效区间，correct明确不同 |
| B04 | 确定性通过 | actor_facts_test.go | 外主体/角色自述不升格事实；system无真人speaker |
| C01 | 确定性通过 | sleep_cycle_test.go | 20轮无model/assistant/activity/新反思证据 |
| C02 | 确定性通过 | sleep_cycle_test.go | confirmed wake到期后才生效 |
| C03 | 确定性通过 | goal_life_chain_test.go / schedule_tool_regression_test.go | 提前触发不执行；冲突保留旧状态 |
| C04 | 确定性通过 | schedule_tool_regression_test.go | 当前剩余段合法修改，历史/不可中断/执行中拒绝 |
| D01 | 确定性通过 | goal_life_chain_test.go | 缺物无需库存ID即可建Goal/Intention |
| D02 | 部分通过 | formal_tool_adapter_e2e_test.go | 独立查询失败有正式失败结果；失败后模型不重复买的语义待live |
| D03 | 确定性通过 | goal_life_chain_test.go | 未来日程链接真实目标/意图，提前检查不持有 |
| D04 | 确定性通过 | goal_life_chain_test.go | 推进Clock，正式due trigger启动购物并结算 |
| D05 | 确定性通过 | chain-0.json | 获取event/item IDs，Goal完成，未自动wear |
| D06 | 确定性通过 | life_activity_capabilities_test.go / validation-summary.json（原始日志已清理） | 失败无物品/无意图完成；重复成功尝试重用item |
| D07 | 确定性通过 | shopping_items.go / item_use_capability_test.go | 单件/套装/普通物品原子获取，独立使用 |
| D08 | 确定性通过 | scheduled_activity_closure_test.go / durable_turn_test.go | 固定尝试重放、重启及取消/改期权威回归 |
| E01 | 确定性通过 | current_capture_test.go | 持有未穿仍渲染权威旧衣物 |
| E02 | 确定性通过 | inventory_source.go / wardrobe_capabilities.go | 跨属主/无来源/失效物品失败，状态不变 |
| E03 | 确定性通过 | chain-0.json | 独立wear后snapshot与最终workflow含已持有物品 |
| E04 | 确定性通过 | current_capture_test.go | model clothing和workflow追加覆盖在submit前拒绝 |
| E05 | 部分通过 | final-comfy.json | 模拟renderer失败不反写事实；真实图片/S3/像素尚阻塞 |
| E06 | 确定性通过 | validation-summary.json（原始日志已清理） | 受控初始化合法，未溯源历史不补造 |
| F01 | 确定性通过 | runtime_summary_test.go | 300..600秒有新增触发，覆盖raw替换，无新增不重复 |
| F02 | 确定性通过 | runtime_summary_test.go | 生成中新消息carry-forward；写库失败不推进；删除重建 |
| F03 | 确定性通过 | prompt_context_assembler_test.go | 完整recent单元与物理Tool续接预算保护 |
| F04 | 部分通过 | budget.json / conversation_segment_test.go | 同数据集预算及跨天历史回归通过；长期真实对话语义待测 |
| F05 | 确定性通过 | prompt_context_assembler_test.go / turn_chain_budget_test.go | 每次物理调用计Tool结果/多模态；超限显式失败 |
| G01 | 确定性通过 | instant_test.go / internal/instant | Z归一数字偏移、UTC毫秒，unknown不猜 |
| G02 | 确定性通过 | instant_test.go / instant-display.test.mjs | 同瞬时投影、跨午夜、明确海外zone |
| G03 | 确定性通过 | instant_test.go | LA/Berlin缺失拒绝、重复选择较早瞬时 |
| G04 | 部分通过 | instant.go | opaque原始审计不重写；未知裸历史未批量迁移，需实际历史清单核查 |
| H01 | 确定性通过 | diagnostic_page_test.go | 全局固定时间/id DESC，null尾部，状态不置顶 |
| H02 | 确定性通过 | diagnostic_page_test.go / diagnostics.test.mjs | 快照多页/filter/cursor；浏览器append与partial-page暂停轮询 |
| I01 | 确定性通过 | repair-cli-apply.json / repair-cli-rollback.json | 实际CLI dry-run/apply/replay/rollback/replay；原始保留 |
