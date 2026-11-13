# M3 复核报告

## 1. 交付状态

本轮按用户授权暂缓 M2 剩余真实模型/产品媒体链路验收，进入 M3 原 T07～T10 范围。M3 当前状态为 **PARTIAL**。已完成下列独立代码行为和本地确定性验证，真实模型自然交流、自动判断和完整 UI/多端/媒体场景保留未验收状态。

本轮开始分支为 `codex/fluctlight-meromero-plugin-humanization`，开始 HEAD 为 `0a53dd91a03eedc48e18b7ba227832edd0e5f3b1`。最终生产实现受检提交为 `fda3abc6c9cbd5d2df37a8e1d9dca1b17a24ff65`。后续 `27159e40c5b89fd6f15519938fae1de114bf29ab` 只补齐两项既有 Goal 测试的首次准入前置，该提交又通过相关 6 项复验。报告与任务记录提交位于上述实现之后。

T03 先前已验证证据继承。M2 的阻塞历史见 `docs/verification/meromero-m2-review-20261113.md`。R0/R1 既有报告保留其当时事实，本轮进度和失败过程保存在 `docs/verification/meromero-m3-progress-20261113.md`。

## 2. 原任务与本轮结果

| 原任务 | 已落地的本轮结果 | 本轮可复核状态 | 剩余验收 |
|---|---|---|---|
| T07 节奏/WakeUp | 原持久时钟内连续无事退避、消息/事实/手工触发重置、Goal/日程约束、重放及 Redis 恢复 | 确定性本地验收通过 | 真实配置模型自主主动/静默正反场景、其余多端流式组合 |
| T08 表达学习 | 当前对象/当前人格的正式 relationship.addressing 输入完整，表达参考明确结合真实关系与角色自主选择；继承已有候选审核/启停/撤销闭环 | 请求输入与 Core 闭环本地验收通过 | 真实模型关系含义称呼、广泛注入/日志排除语义、UI 到真实运行闭环 |
| T09 记忆治理 | legacy 来源隔离及 Owner 恢复、摘要/派生链失效、在途发布保护、两个实际死锁修复、Memory/ActiveMemory/Outcome 画像及人格来源传播和时效 | 原要求定向实现核对完成，来源/事务本地验收通过 | 真实模型自行识别玩笑/撤回/未知并选择治理操作、收到 stale 后的自然恢复；有限来源合同证据边界继续保留 |
| T10 领域保护 | 原有 Goal/获取/染发/关系接受/非法引用/持久化重放红测恢复实际准入，成功标准保留 | 关键领域受控回归通过 | 真实模型、真实渲染/交付及完整领域矩阵 |

原父任务与全链的完成状态继续由各自完整验收决定。本报告的本地通过范围明确覆盖 Core 数据、来源、事务和请求合同。

## 3. 独立节点与实现位置

| 节点 | 完整交付行为 | 主要实现提交 |
|---|---|---|
| M3-01 | 连续无事 WakeUp 退避及恢复 | `6a96cef6` |
| M3-02 | 真实关系称呼输入与自主参考约束 | `99dcd67f` |
| M3-03 | 可定位 legacy summary ref 的纠错屏障 | `ae9023ce` |
| M3-04 | direct/Writer 在途 revise/forget 发布栅栏证据 | `b0d47d68` |
| M3-05 | Goal 领域准入后关键回归恢复 | `4f9d6aba`、`9f50332f` |
| M3-06 | 无可解析来源旧记忆隔离与 Owner 恢复 | `a4b1bc8e` |
| M3-07 | Owner/Segment 和 generation/journal 两个实际锁环修复 | `f5fccd0b` |
| M3-08 | SelfClaim 的 Memory/摘要祖先来源传播 | `75f03c39` |
| M3-09 | overlay 当前/历史窗口/后继绝对值来源传播 | `7731b7da` |
| M3-10 | ActiveMemory/Outcome、直接/间接时效与统一业务时钟 | `fda3abc6` |
| M3-11 | 两项已登记 Goal 引用/恢复测试实际准入 | `27159e40` |

节点按原任务内部独立行为串行实施。详细命令、失败、修正和受检 overlay 清单保存在进度报告及对应日志中。

## 4. 正式链路与实现选择

### WakeUp

正式 `ProcessWakeUp → persistCommittedWakeUp → updateWakeUpNextDueTx` 使用同一 PostgreSQL intent 和 next_attempt_at。基础首次检查为十分钟，本轮选择连续无事的下一间隔为 20、40、60、60 分钟。未来日程/活动边界和 pending Goal review 时间约束下一次 due。真实消息、原生事实和手工触发重置退避。Redis key 为持久 due 的提示镜像，丢键恢复经过 due guard。

该间隔是本轮普通实现选择，原要求没有指定具体阶梯。测试记录了重放不增加 streak、失败/实际效果保留基础行为及 Goal/日程约束。

### 表达与关系

`factOnlyContextFacts` 保留正式 `role.addressing.preferred/self_reference`，沿用当前人格与当前对象选择。表达候选仍是可选资料，实际协议要求结合真实关系、已有称呼和当前人格自主选择。source/version、使用开关、撤销与发布复核沿用既有机制。

### 来源与画像

用户明确选择完全无法解析来源的旧记忆暂停自动注入，保留数据，待 Owner 重新确认来源后恢复。长期读取在候选 LIMIT 前要求 live support，混合有效来源按既有 partial 合同保留。ActiveMemory 检索、Resident 发布和旧缓存读取重新验证命令来源。

Owner ReviseMemory 重新确认后，旧记录的新 revision 进入检索及 direct/Writer 物理请求，历史 source links 和旧 revision 保留。ActiveMemory 恢复场景验证现有 Core lifecycle Confirm，Owner UI/API 一键恢复属于未覆盖的产品入口边界。

Reflection 已保存完整 context_snapshot/ReferenceIndex。本轮复用其中的实体、scope、revision、request digest 和已有递归 source authority，覆盖 SelfClaim、Owner source_valid、恢复旧快照、overlay 当前 accepted candidate、历史窗口与写前复核。后继绝对 after 值按原字段的有效 before 基线判定，独立字段与新有效基线保持可用。

原 T09 允许安全失效、更新或标记过时。本轮采用安全失效和原始历史回退，自动重建并重新启用已失效摘要没有被追加成完成前置。

### 并发与事实保留

两个正式双事务场景在修复前真实产生 PostgreSQL 40P01。修复为 Memory Owner 行锁 `FOR NO KEY UPDATE`，Segment 在 conversation/intent/summary/generation 前取得同一 Owner 行锁。权限、status、Memory revision、幂等及原事务提交边界均继续参与验收。

正式购票反例真实受理“我买了票”和“刚才是开玩笑”，Owner 改正后，memory.recall 同时保留两次历史发言并只返回购票未知的长期更正。

## 5. 受检环境、命令与结果

所有数据库场景使用本机专属隔离 PostgreSQL 容器 `fluctlight-meromero-test-9290b39b` 中随机新库。Redis 使用本轮独占容器 `fluctlight-m3-redis-5cf969fbf8`，所有测试结束后已按任务标签核对并清理。既有 PostgreSQL 容器继续保留。Provider 为受控 transport 或测试服务。模型及媒体证据按下表分类。

| 实际批次 | 受检提交 | 结果 | 证据 |
|---|---|---|---|
| 最终 Core 汇总 | `fda3abc6c9cbd5d2df37a8e1d9dca1b17a24ff65` | 206 个顶层 PASS，exit 0，0 SKIP | `docs/verification/meromero-m3-evidence/delivery-postcommit.log` |
| 同提交迁移/当前 Head 重跑 | 同上 | 1 PASS，exit 0，0 SKIP | `delivery-migration.log` |
| 最新 Goal 测试提交后复验 | `27159e40c5b89fd6f15519938fae1de114bf29ab` | 6 个顶层 PASS，exit 0，0 SKIP，其中 2 项是上述汇总外的补充 | `t10-final-postcommit.log` |

两个测试源码版本之间生产实现相同。Core 汇总选择清单在 `delivery-test-selection.txt`。每份日志头保留完整 `go test ./internal/core -count=1 -run <实际精确正则> -v -timeout=...` 命令、SOURCE_HEAD 和 OVERLAYS。最终提交级运行的 OVERLAYS 为空，实际代码来自干净 Git 导出。迁移使用 `go test ./internal/migrations -count=1 -run '^TestSummaryCorrectionGuardMigrationFromPreviousHeadAndRerun$' -v`。

这些证据属于真实 PostgreSQL/Redis 及受控 Provider 的合同/集成验证。此轮真实模型推理、真实 ComfyUI 产品交付及自然交流效果按用户授权暂缓。T10 中购物、染发为隔离虚拟业务，最终图像 workflow 为文本装配检查。

## 6. 全部失败与修正记录

| 类别 | 实际情况 | 保留证据 |
|---|---|---|
| 实现缺口 | 固定十分钟续排、丢 Redis key 提前释放 future due | `t07-red-corrected.log` |
| 起始基线已有 | Goal cadence 阻止旧领域 fixture 实际进入评估 | `t07-baseline-goal.log`、`t10-domain-red.log`、`t10-affected-baseline.log` |
| 实现缺口 | 首次全 legacy/unknown 摘要来源绕过纠错 | `t09-legacy-red.log` |
| 实现缺口 | Owner 与 Segment/Publication 两个正式 40P01 | `t09-lock-red-overlap.log` |
| 实现缺口 | Memory/summary 派生 SelfClaim、overlay、历史窗口和后继绝对值未失效 | `t09-profile-red-bound.log`、`t09-overlay-red-bound.log`、`t09-overlay-lineage.log` |
| 实现缺口 | ActiveMemory/Outcome 画像版本传播、ActorFact 到期 | `t09-profile-family-red.log`、`t09-actor-expiry-red.log` |
| 本轮局部错误 | Clock 字段、RawMessage 类型、prepared DDL、fixture revision、噪声必填字段、restore target、JSONB byte-level 重验和 habit receiver 均已修正 | 对应 red/green/validated/corrected 日志及进度报告逐项记录 |
| 统计更正 | T08 早期扩展实际 46 PASS、1 项私有录制 wire replay SKIP；此前“无 SKIP”摘要有误，已更正。最终 206+1 汇总及最新 Goal 复验都实际执行且无 SKIP | `t08-postcommit.log` 与进度报告更正段 |

独立只读复核曾指出同字段未采用候选误关联及间接依赖/clock 漏传，本轮增加对照并修正。失败样本持续保存，后续成功日志采用新文件名。

## 7. Git 与现场

所有实现和证据分批提交，当前实现检查点包括上表各节点。四处用户已有修改在本轮每次 SHA-256 核对中一致，保持最终未暂存状态。

| 文件 | SHA-256 |
|---|---|
| `agent_model_routing_test.go` | `d344d7a5d2aa75212391ea70c36148b63fdccf4e8955ed6c88c0c5ddcf4eb857` |
| `media_capture_plan.go` | `c0bfe6111a5e8ef6da497c5a6887fb48ccc271a18d9bfa96df21d2ee79fabb75` |
| `media_prompt_branches.go` | `f24c2101f051e82f73052d21a9c80b398d6473c0c10ec656afffe7114a764279` |
| `task_prompt_sources_test.go` | `b8a39afa9e3e6b740d60bd73fac1bceadcd972e4c54553e2c50ecbac1ca89fa7` |

路径均位于 `apps/core-go/internal/core/`。每次代码提交运行图片检查，提交工件为代码、测试、文本证据及任务记录。测试使用干净受检源码，四份草稿留在原工作区供用户继续处理。

## 8. 剩余边界与后续恢复

- 真实配置模型的主动/静默、关系含义称呼、广泛注入排除、自动理解纠错及自然恢复继续等待真实模型/用户验收。
- 原 T07 的多端流式组合及原 T08 的完整 UI 到运行验收仍待执行。
- 原 T10 的真实模型、真实渲染/产品交付和完整领域矩阵仍待验收，M2 延后的外部链路也保留原状态。
- 有限双事务和来源样例证明所列不变量，全仓任意交错及无限自然语言事实准确性保持其证据边界。

代码回滚以节点实现提交作为边界，在隔离审阅分支逐个反向 revert 并复跑对应日志中的场景。SQL 函数沿已有 Apply 重装，原业务/审计记录继续保留。当前工作区四份草稿应在回滚操作中继续保护。

本轮在 M3 复核边界交付，M4 按用户后续里程碑授权推进。
