# F19 提交失败诊断与复盘

## 证据

用户报告仅有自然语言摘要，缺实际第二次arguments和warning_code，不能宣称其唯一根因已确诊。两个干净只读探子定位了领域和native两层，主线程亲自读取修改代码。

- goal_set.go / applyGoalSetTx：dependencies实际为Goal→prerequisites。context_review_required字符串被当Goal ID查询，pgx.ErrNoRows映射ErrUnauthorized。真实PG复现相同错误。
- goal_planner.go / executeGoalPlannerCommitTx：缺版本返回planner_snapshot_required；领域ErrInvalidArguments原本常合并goal_plan_rejected→needs_input。实际PG的不完整order用例复现第二类opaque回执。
- reviewed_goal_ids专用清hold已完整实现；insertAgency允许空criteria+needs_reflection初始目标。基线PG正向提交已证明二者兼容。
- 初始Planner prompt真实包含snapshot，但bindGoalPlannerMetadata原本只认更早Tool query；漏抄已有初始版本会无意义停住。
- ADK diagnostics旧码只写空error_code，handled原因丢失；模型final可以将技术参数未提交误称policy block。

## Bug Analysis

1. Root cause：B跨层合同+D测试缺口+E隐式假设。状态标记与资源身份混用；已提供快照不被默认绑定识别；把handled当业务成功或policy拒绝都不正确。
2. 前次兜底不足：F18保证未完成不会假成功，但不能单凭改error为handled使参数正确；缺少成功执行/真实review-only native回归。已知第二次根因仍需用户真实warning_code，不能拿测试猜原始请求。
3. Prevention：精确metadata规范化不推断review，可信snapshot按消费先后绑定，具体typed代码/固定指导贯通ledger→native→diagnostic→final，PG断言真实效果及回滚，独立registry inventory防description超限使工具不注册。
4. Expansion：这里不重做49项审计、不增加Planner更新criteria操作、不设新调度预算；全工具handled诊断补warning/disposition以支持后续定位。
5. Knowledge：已同步goal-planner-contract和tool-availability-contract；本仓无spec模板目录，不创造插件模板。提交按已存在workflow3.4最终清单审阅执行。

## 修复效果边界

可以真正提交：review-only；混有伪标记而其余操作合法；省略已知技术fences。不能推断：目标语义仍有效、复核已做、原criteria已经满足。精确标记忽略不清hold；需模型依据实际详情显式reviewed_goal_ids。

保留goal_planner.commit实际效果前提：

| 条件 | 具体理由 | 替代行为 |
| --- | --- | --- |
| 非当前主体/profile的真实Goal或Actor | 不能修改其他主体的业务记录 | 实际拒绝；选当前已可见Goal，不扩大授权 |
| 伪造/过期private claim或suggestions模式写入 | 旧任务/只建议任务没有本次apply权 | 无领域写入；等待合法run，lease冲突有刷新回执 |
| 显式旧snapshot/事实版本 | 防止覆盖已变化的真实上下文 | handled awaiting_state，刷新后最多调整一次 |
| Owner保护的pause/cancel或配置写入 | Planner不具有Owner治理权 | 保留Owner目标；正常review-only仍可执行 |
| 无法确定的参数/循环依赖 | 无法安全制造目标事实或调整业务规则 | handled具体warning+指导，全事务回滚 |
| DB/网络/Provider/取消 | 基础设施失败不能冒充成功提交 | 保留真实原因/回滚；不加新自动预算 |

Evaluation30min、Planner真实空位与合法完成/日/Owner episode沿F15，不为修复新增自动模型调用。

## 独立审阅

干净只读代理planner_commit_independent_check发现P2：handled commit后query的真实失败会被final误改awaiting_facts。主线程已修复为存在任何failed/rejected/cancelled/timeout回执时保持真实原因；补前/后query failed用例。其余规范化/版本/幂等范围静态未发现问题。该审阅未运行测试；最终执行由主线程负责。

## 既有测试断言校正

TestGoalPlannerCommitToolStandaloneReplayConflictAndProtection在42a2d7c的独立源码快照也失败：它把同run第二条命令已提交冲突的handled/no-effect误判成Planner成功pause。此前真正的protected-goal未授权分支仍保留且通过。本轮只校正后半段为goal_plan_already_committed无效果回执，并额外查询原Goal仍active且owner_protected=true；不修改保护规则。基线执行见verification/goal-planner-commit-results.json。
