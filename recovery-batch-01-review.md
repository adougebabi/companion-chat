# 恢复批次 01 回执

本批只处理 A 的真实模型覆盖验证和 B 的 direct 媒体完成声明。历史现场与上次阻塞分别见 [R0 恢复核对](recovery-review.md) 和 [R1 回执](R1-model-override-review.md)；这两份报告保留其当时事实。

## 受检版本与改动

- 批次开始 HEAD：`7e2125d2edbadad2f537dd7cff12e6a08ccd2edb`。相对 R0 受检的 `a45c4a3d98be602ec4a430db47b9da0a9a39ae92`，相关生产实现无变化。
- A 新增测试 `apps/core-go/internal/core/t02_live_model_override_integration_test.go`，提交 `0bd08f2b`。真实测试使用批次开始提交的干净源码加该测试；受检测试文件 SHA-256 为 `ff34d9a1f7993caaf8398382f41918a07fd4dd9a6d8688c92fcea9af7c03afeb`。随后对整个 `apps/core-go` 目录与 `0bd08f2b` 的 Git 导出执行逐文件比较，内容一致。
- B 将零媒体依据反例整理为 `apps/core-go/internal/core/direct_media_delivery_claim_test.go`，提交 `52de51e7`。它是**尚未修复的红色回归测试**：在设置隔离 PostgreSQL 后运行会失败，不能把它当成修复通过。本批未修改业务生产代码。
- 原有四处未暂存文件与 R0 保存的补丁逐字节一致，没有纳入上述提交。没有重新建立 R0 备份，没有连接生产业务库或使用真实用户内容。

## A：两轮 A1/A2/A3 真实验证 — VERIFIED

本机 `127.0.0.1:11234/v1/models` 在本次运行前列出并确认以下 ID：

- M：`llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4`
- Q：`Huihui-Qwen3.8-27B-3.8bpw`

两轮各自创建并清理一个独立的本机 PostgreSQL 测试库；每阶段使用正式配置保存/回读，串行调用正式 `RunReflectionProposalTask` 和固定的无外部副作用对照任务 `runExpressionLearningTask`。反思任务使用既有完成检查、schema 检查及 `DecodeReflectionProposalV2`；对照任务使用既有来源和候选校验。下表的“返回模型”是 Provider 响应中的 `model` 字段，属于 Provider 自报，不能独立证明后端实际加载的模型实现。

| 独立轮次 | 阶段 | reflection 请求 / Provider 返回 | reflection 有效来源与版本 | 对照请求 / Provider 返回 | 对照来源与版本 | 两个最终任务合同 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | A1：无覆盖 | M / M | `generic_llm` / 1 | M / M | `conversation_reply` / 1 | 均通过 |
| 1 | A2：variant 覆盖 | Q / Q | `agent:reflection:reflection_proposal_v2` / 1 | M / M | `conversation_reply` / 1 | 均通过 |
| 1 | A3：撤销覆盖 | M / M | `generic_llm` / 1 | M / M | `conversation_reply` / 1 | 均通过 |
| 2 | A1：无覆盖 | M / M | `generic_llm` / 1 | M / M | `conversation_reply` / 1 | 均通过 |
| 2 | A2：variant 覆盖 | Q / Q | `agent:reflection:reflection_proposal_v2` / 1 | M / M | `conversation_reply` / 1 | 均通过 |
| 2 | A3：撤销覆盖 | M / M | `generic_llm` / 1 | M / M | `conversation_reply` / 1 | 均通过 |

两轮中 variant 的配置修订均为：A1 不存在、A2 为 1、A3 撤销后为 2；每阶段另行读取有效绑定及版本。共完成 12 个逻辑任务、13 次真实物理请求：第一轮 A2 的 reflection 有两次 Qwen 物理请求，均已分别记录，**不算两次独立场景**。日志中的 `contract=pass` 表示该逻辑任务的最终结果通过既有合同，不声称逐一验证了每个中间响应的业务合同。两个测试子场景均 PASS，无 SKIP；此结果只证明本项测试范围，不证明所有 Agent 或模型后端的内部加载实现。

## B：direct 零媒体依据声明 — BLOCKED

- **修复前红测：** 在隔离库、显式 `direct`、受控 Provider 的真实发布路径中，`TestDirectReplyDoesNotPublishUndeliveredPhotoClaim` 失败：`claims=1 media_calls=0 media_intents=0 turn_error=<nil>`。错误正文“我已经把照片发给你了。”作为空附件消息正式写入了测试会话。
- **已定位的正式代码：** `direct_main_reply.go:83` 的 Prepare 校验决策/引用后，经 `builtin_capabilities.go:240` 和 `tool_publication.go:72` 进入事务发布；它没有对该正文做媒体交付依据核验。现有 `reply_action_consistency.go:11` 只针对真实换衣失败回执，不能直接处理没有媒体调用的 direct 候选。
- **停止原因：** 现有 direct `conversation.reply` 的正文与认知决策中，没有把正文所指的历史媒体任务绑定到可核对的交付引用；主会话的投影也没有开放历史媒体结果引用（`provider_context.go:557`）。旧式交付证据和草稿式交付分别走 `media_delivery_evidence.go:5` 与 `reply_media.go:380` / `fact_only_reply_capability.go:137` 等不同路径。存在一张旧附件只能证明某个旧任务有交付，不能证明它就是候选正文所指的照片。
- 在现有接口下，仅凭任意历史附件放行会违反 B4；仅靠正则或同一模型自报一致也不足以建立正文与具体交付任务的关系。要可靠覆盖 B2/B4，需要为所指任务建立可验证的引用/事实投影及其作用域规则；这属于本批明确要求停止的新增领域边界。没有以猜测或一条正则冒充修复。
- **生产修复、修复后测试、B2–B5 回归：未执行。** 没有切换默认聊天模式，也未修改 Prompt 系统、回退政策、插件、Temporal 或 UI。B 不能标记为通过；已提交红测在缺数据库变量时会被既有测试夹具跳过，使用隔离库时按上述方式失败。

## 命令、结果与证据

| 项目 | 命令 / 环境 | 结果 |
| --- | --- | --- |
| A 测试发现 | `go test ./internal/core -list '^TestLiveT02VariantOverrideIsolation$'`，干净源码副本 | 首次编译因测试自身 `int` / `int64` 比较失败；修正测试类型后退出 0，确实列出测试。 |
| A 真实验收 | `go test ./internal/core -count=1 -run '^TestLiveT02VariantOverrideIsolation$' -v -timeout=40m`，本机真实 Provider + 仅回环隔离 PostgreSQL；模型调用串行 | 退出 0；2 个独立轮次 PASS，0 SKIP，约 269 秒。没有使用 Mock 模型响应。 |
| B 修复前红测 | `go test ./internal/core -count=1 -run '^TestDirectReplyDoesNotPublishUndeliveredPhotoClaim$' -v`，隔离 PostgreSQL + 受控 Provider | 退出 1，实际运行、0 SKIP；复现 1 条错误消息、0 媒体调用和 0 媒体 intent。 |
| B 修复后与直接回归 | 未执行 | B 在历史任务引用边界处按用户停止条件中止，没有生产修复可验收。 |

A 的完整日志、每次合成输入对应的 Provider 原始响应及首次编译失败，已保存至仓库外私有目录：
`/Users/vinson/Library/Application Support/Fluctlight/Recovery/R0-20261112-a45c4a3d-hlwhk6oa/batch-01-a-evidence/`。`sha256.json` 记录复制校验。原始响应是测试合成内容，不写入本报告或 Git。

B 红测日志及其 SHA-256 位于同一私有目录下的 `batch-01-b-evidence/`。R0 原始报告、原始日志和原备份未删除。

## 失败与处理记录

1. A 首次测试发现编译失败：版本号比较使用了不同整数类型；这是本测试自身错误，已在测试文件内修正，重编译通过。失败日志 `test-discovery.log` 和后续通过日志均已保留。
2. A 的两轮真实任务最终合同均通过；其中第一轮 A2 有两次同模型物理请求，具体原因未在本批确定，原始响应均保留，不将其隐藏或当作第二轮。
3. B 修复前红测按预期失败；没有为了通过而删除测试、放宽断言、强制拒绝所有回复或修改生产策略。识别到 B2/B4 的任务指代与真实交付引用缺口后按授权边界停止。

## 父任务状态

- **A：VERIFIED；B：BLOCKED。** A 只完成了 T02 的这一项真实覆盖与撤销证据；R1 的 AEL 场景记录仍处于待验收步骤，技术结果以上述实际运行日志为准。
- 父 T02 的历史 `completed / verified` 记录没有被重开或删除，且不能据此宣称原 T02 全部完成：迁移主链、旧任务快照策略、真实 Temporal/视觉以及其他入口仍有未完成或未验证项。
- T04 的默认单主链和 T06 的完整发布可靠性均未完成；B 的红测仍失败。没有推进其他任务。
