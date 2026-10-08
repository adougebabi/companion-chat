# B 最新实施检查点（2026-10-07）

用户已批准完整 G00–G13 计划；不需要再次审批。整体仍在实施中，B 不应仅因下列测试通过而归档。

已补齐并核验：
- 正式普通对话 → 真实消息 journal → 共享 Evaluation → 原表达 Goal 完成，无 Intention/Attempt。
- 关系确认显式使用 relationship authority；实际双方消息、参与范围校验。评估快照冻结 relationship ID/revision，提交不能覆盖 Provider 期间的新编辑。
- 开放信号/引用/拒绝不完成；实际接受完成并使用共享关系 writer。脚本 Provider E2E；不属于真实 Provider 语义通过。
- 可选 followup 无残留动机/重复/无效定义独立审计拒绝，不回滚原完成。
- 空评估使用 Life→request 相同锁序与 claim revision fence。
- 父 Goal 恢复重新检查 Commitment 结束窗口及 blocker；不盲目 active。
- Stage 策略调整不无条件变更标准版本。
- >6 Goal 分批保留 source 处理水位，真实证据不会在首批后丢失。
- 模型输入使用 compactGoalEvaluationInput，完整 CAS 快照仍保留 DB；没有提高输入预算。
- 购物/染发/Reflection 旧测试已适配正式共享评估。
- inventory item source 读取增加既有 inventorySourceVerifiedSQL 验证。

最近扩大隔离 PG + race 回归：父 research/b-layer-full-compact.jsonl，72 PASS events、0 FAIL/SKIP；62 叶子 PASS（明细 b-latest-validation.json）。这不是整个仓库/真实模型/Worker 联合验收。随后 C 投影改动单独 c-generic-dialogue-projection.jsonl 11 PASS events、0 FAIL/SKIP。

仍需 B 门禁收口：
- 关联 candidate/rejected 可审计完整性及领域 source trigger rollback/late-event coverage。
- Intention Stage/Commitment 链接字段及省略 StageID 时一致性，独立 Tool/原生 loop 受影响矩阵。
- 同批标准调整/对象评估、阶段 skip/冗余承诺、机会解除/过期 Review。
- spec 同步、独立核查结论落盘、提交；不得把 C/D 全部当作完成。

C 已 start，Core Owner service/internal HTTP/通用投影正在实现，0052_goal_governance 已开始（尚未全部接线）。Review schema 当前仅建表，未形成 producer/consumer，不能宣称 Review 完成。
测试容器 fluctlight-goal-test-7ef70c25；秘密 URL 仅 /tmp/fluctlight-goal-test-runtime/runtime.json；通过 run.py wrapper 使用。B/C 未提交；不改生产库。
