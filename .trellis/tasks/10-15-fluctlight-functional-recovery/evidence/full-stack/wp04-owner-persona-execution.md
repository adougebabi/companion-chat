# WP-04 Owner 人格请求执行链（2026-10-16）

本切片实现 Owner 请求 → 已提交 Workflow intent/来源事实 → Worker 的既有 `RunPersistentSwitchTask` → 受控人格 writer → 持久 terminal 回读。复用 `owner_surface_records`、`platform_workflow_intents`、outbox 和 Working Persona；没有新表/迁移，没有调用外部 live 模型。

## 生产行为

- `POST /api/fluctlights/:id/persona-switch` 仍要求 Owner、声明目标、confirm、幂等身份、Foundation/runtime 双 CAS。相同目标同步 `completed/no_change`；有合法声明规则的不同目标返回 `202 pending`，包含 `request_id`、`status_url`、`source_fact_id`。
- 正式 Owner 请求事实为 `owner.persona_switch.requested` outbox，稳定 intent 为 `persona.owner_switch`，只在 lifecycle Worker 执行模型评估。API 不执行 Provider Activity。
- 评估可拒绝；Owner 确认不能替代时间/语义等自由条件。只接收命中 enabled persistent 声明、原 source/requested target 的结果。退休、双 CAS 变化、冷却、未声明/错误规则均不能产生切换。
- Core persona service 在同一短事务写运行态/切换 outbox/请求终态；原始有效身体、穿着和长期生活状态不重新初始化。公开 history/runtime 使用 Owner 原理由，模型评估理由和原始 Provider 错误不进入请求读模型。
- Worker 失败按 3 次持久计数重试；终态 failed/rejected/completed 可经 Owner request GET/list 回读。Workflow 提前失败或取消的 reconcile 会关闭 pending 请求；晚到结果和服务重建重放不重复写效果。Activity 有 bounded timeout/heartbeat；提交前同时检查持久取消状态。
- `owner_switch_targets` 返回可提交评估的声明目标（`rule_id,target_profile_id`），不是承诺立即切换。
- 浏览器生成类型：`BrowserPersonaSwitchRequest/Result/Terminal`；实际可调用 `BrowserClient.switchPersona()`、`ownerRequest<BrowserPersonaSwitchTerminal>()`。

## 已执行证据

1. 通过真实临时 PostgreSQL（每例独立创建/迁移/清理临时库），受控 Provider transport，生产 App consumer：

   `python3 /tmp/fluctlight-recovery-1015/envexec.py env GOCACHE=/tmp/fluctlight-go-cache go test -race ./internal/core -run 'TestPostgresOwnerPersonaSwitch|TestOwnerPersonaAssessmentTerminals' -count=1 -v`

   最终 PASS，18.282s。主成功用例覆盖同 payload 并发受理、无变化同步结果、变更幂等键冲突、异 Owner/未知目标/双 CAS admission、Worker 真实评估提交、服务重建/replay、GET 终态、一次切换 outbox 历史、cooldown、退休以及 body revision=7、wardrobe revision=8、blue hair、worn shirt 保持。11 个分支：keep、workflow_failure、late_workflow_failure、failure、recover、foundation_cas、runtime_cas、wrong_target、wrong_rule、disabled_rule、retired。

2. `GOCACHE=/tmp/fluctlight-go-cache go test ./internal/workflow -count=1` 通过，包含新 Workflow 的评估重试后 completed 和 rejected 终态、management registry。此为 Temporal SDK test environment，未声称真实 Temporal server/Worker restart 验收。

3. `node --import tsx --test test/*.test.ts`（packages/browser-client）29/29 通过；包括 index 实际导出 client 的双 CAS 命令、202 → typed Owner request terminal 回读。

4. browser-client `tsc --noEmit` 通过；两份 generator 已重跑；`git diff --check` 通过。persona 定向 Browser route 测试通过。`go vet ./internal/core ./internal/workflow ./internal/httpapi/browser` 通过。

## 仍需主任务统一核验

- 外部 live Provider 模型语义判断与真实 Temporal 部署/进程重启证据未运行。本切片的 Provider transport 受控，PG 真实，不以 controlled 代替 live。
- 全 browser 包检查发现共享 world 差异：`TestBrowserRouteMatrixMatchesOpenAPIArtifact` 的 GET `/api/fluctlights/fl-1/world/catalog/world-1` 缺 OpenAPI；`TestSurfaceRoutesMatchCoreAndMatrix` 的 Core matcher 缺 PUT `/api/fluctlights/fl-1/world`。已报告主线程；本切片不修改世界域。
- API/Worker 部署需重编以加载新 intent/Workflow；共享前端 provider 必须等 terminal completed 且读回 active profile 后显示生效。旧二进制无此消费者。
