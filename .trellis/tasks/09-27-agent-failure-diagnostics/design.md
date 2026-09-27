# 设计

在 Go Core 的 Owner 诊断读模型中投影 `agent_runs` 的正式模型/Tool循环终态，以稳定 `run_id` / correlation 与已有物理 model-run、`adk.tool.*` 事件关联。`agent_runs` 是该循环的replay fence；最终契约、发布和结算属于后续调用者边界，失败时以 `agent.run.termination` 独立记录。`diagnostic_model_runs` 仍只表达单次 Provider 调用；UI 分层显示，不改任一原始事实。若旧 Agent run 没有可直接查询的 correlation，显示未知，不按时间猜。

新 Owner-only 投影只返回 agent、run/correlation、开始/结束时间、状态、失败阶段、稳定代码及经现有 typed redactor 清理且限长的 safe cause。失败阶段/代码由执行边界结构化记录，不从任意 `error_detail` 文本用正则猜。Tool 摘要仍只暴露 call ID、capability、status、safe code；不输出 Tool arguments、原始 `error_detail`、reasoning 或 provider secret。终止事件作为补充，不能覆盖权威 `agent_runs`。

验证真实 ADK Tool callback 的 `model_call_id` 传递。如果 callback 不继承物理 Generate 的子 context，在 run-scoped 执行状态中显式保存本轮 model-call identity，不能用“最近一条模型行”按时间猜。生命周期失败写入在已取消 ctx 下使用独立短时诊断 context，遵守 best-effort/health warning 规则。

通过 Go Core Owner API → Go browser boundary 显式字段映射 → generated browser client → `control-center` store → `DiagnosticsView` 增加逻辑运行区。页面按 correlation 展开本轮物理模型、Tool 与 Agent 终态；筛选不再依赖全局最近 20 条系统事件。旧行缺数据时显示“关联未知（旧记录）”。

新增数据库字段/索引（若需要）应 additive；回退 UI 后旧 Model Runs 查询仍可用，`agent_runs` replay fence 不受影响。预计修改 `agent_run_record.go`、诊断读写、必要迁移、Core/Browser API、OpenAPI/生成客户端、`control-center.ts` 与 `DiagnosticsView.vue`；具体文件实施前复核。
