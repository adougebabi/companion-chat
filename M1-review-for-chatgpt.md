# M1 里程碑复核回执

## 结论与范围

**M1 状态：PARTIAL。** direct 媒体声明、已提交交付证据、指定范围的越权工具循环，以及私聊模型快照子项已有实现和验证。用户随后明确要求对偏离原任务的非阻断性问题只登记、不继续修复；同一无权限调用 ID 的跨轮重放已登记为缺陷并暂缓，因此不能把宽泛的“权限拒绝不会导致无意义循环”全部标为通过。该暂缓不是 Provider 或数据库环境故障，也不作为本轮继续扩展的理由。

本报告只覆盖 M1，不宣称 T02、T04、T06 或 MeroMero-first 全链完成。继承 [R0 核对](recovery-review.md)、[R1 历史回执](R1-model-override-review.md)、[恢复批次 01](recovery-batch-01-review.md) 和 [原验收台账](docs/verification/meromero-first-ledger-20261109.md)，不改写它们的历史结论。M1 当前视图见 [进度表](docs/verification/meromero-m1-progress-20261112.md)。

受检生产源码提交：`612966ec5eb9c742a7dc15b23e436ed39b9db816`；分支 `codex/fluctlight-meromero-plugin-humanization`。M1 开始基线为 `988c7cc3b7166dd736d00dde45ba81295256f023`。最终报告及证据若另有后续文档提交，不改变上述受检生产源码。

## 子项与原任务对应

| 子项 | 原要求与实际结果 | 状态 |
| --- | --- | --- |
| M1-01 | T04/T06：正式 direct 回复中，无可核对交付依据的照片完成声明不能发布；普通交流和有依据表达保留。提交 `3da8fefc`。 | VERIFIED，限所列场景 |
| M1-02 | T06：M1-01 内共同实现当前/历史交付引用的作用域、任务身份、消息附件、素材、引用及发布事务重查。 | VERIFIED，限 direct 交付证据 |
| M1-03a | T06：同一无权限工具以不同参数和调用 ID 跨模型响应重复时有界；同批合法工具不因尚未反馈的拒绝被拦。提交 `c6d80502`、修正提交 `5cf41aa9`。 | VERIFIED，限不同 ID 的范围拒绝 |
| M1-03b | 同一无权限调用 ID 或窗口关闭回执被跨轮反复重放时，仍可能无意义循环。AEL `meromero-m1-03b-replayed-denial` 已登记；用户要求作为非阻断问题暂缓。 | BLOCKED：用户指定暂缓 |
| M1-03 总项 | M1-03a 通过；M1-03b 及其他策略拒绝/终止错误未完整验收。 | 部分完成 |
| M1-04a | T02：旧私聊续接有 cognition 快照时恢复原绑定；模拟旧记录缺快照时使用当前绑定、记录 `legacy_unfrozen`，不补造原模型。提交 `dce89ce8`。 | VERIFIED，限该续接路径 |
| M1-04b | T02：新 direct 逻辑 turn 首次请求前冻结模型绑定，恢复或重试同一 turn 时继续使用；新 turn 使用新配置。提交 `612966ec`。 | VERIFIED，限所列场景 |
| M1-04 | 继承 R1 两轮真实 variant 覆盖/隔离/撤销证据，结合 M1-04a/04b 和既有旧任务策略。 | VERIFIED，限 M1 快照目标；不等于 T02 完成 |
| M1-05 | 最新提交的直接相关整合回归、真实请求和本回执。 | VERIFIED：回执与证据收口；M1 总体仍 PARTIAL |

原 T01～T12 的交付归属保持不变：T01 的既有入口盘点沿用原记录；T02 涉及本次 M1 快照和后续 M4 管理/总验收；T03、T05 及 T04/T06 的其余主链和插件工作归 M2；T07～T10 的拟人交流与生活系统归 M3；T11、T12 及剩余原始验收归 M4。本次没有重新盘点这些任务，也没有改变其原验收。

## 生产行为与兼容边界

- direct 发布准备阶段对涉及媒体交付的候选进行有界检查；Core 只接受当前会话、授权主体及具体任务可核对的已提交消息附件、素材和引用，并在发布事务中再次核验。单独的 queued、ready 或 failed 状态不能证明交付；若此前已有真实提交的附件，后来某次失败不会抹去那次真实交付。
- 检查器默认为会话文本模型，也允许现有 Agent/variant 覆盖；它提取声明和候选引用，真正的交付依据由 Core 判定。自然语言预筛选和语义提取均有覆盖边界，不能保证识别所有措辞或证明所有自然语言事实。旧消息后补附件时，无法可靠确定的交付时间标为未知。
- M1-03a 只针对 `capability_not_authorized` 的无效果回执，把同一工具改变参数的重复按物理模型响应轮次计数；正常可修复参数错误仍按参数区分。第三轮拒绝与合法工具同批时，合法工具仍可执行。同 ID 回执重放未包含在此通过结论中。
- 新 direct turn 的 cognition 绑定保存于 Core 运行元数据，含模型、端点引用及配置来源，明文密钥不进入快照；相同 turn 重试恢复该绑定，新 turn 读取当前配置。模式变化时已存在的 cognition 绑定仍恢复。快照缺失或损坏不能对新 turn 静默重绑；旧 turn 没有可靠快照时使用当前配置并记录 `legacy_unfrozen`，不能声称旧模型是什么。
- 旧媒体及部分 Goal/视觉路径继续既有 `legacy_unfrozen` 当前选模行为；旧衣柜准备、Diary、Studio 在无法证明所需快照时按各自现有合同显式失败或要求新尝试。相关实现相对 R0 受检提交未变，沿用 R0 证据；完整后台、真实视觉及迁移后主链恢复仍归后续验收。每次物理请求配置诊断目前仍是尽力记录，不能宣称 T02 的严格持久记录要求已全部满足。

## 验证、命令和证据

隔离数据库为本机回环专用 PostgreSQL；每个集成测试创建并清理独立测试库。生产业务库和真实用户内容未使用。受控 Provider/合成资产不等于真实模型、真实渲染或端到端交付。

| 验证 | 命令与受检对象 | 结果及证据 |
| --- | --- | --- |
| M1 最终整合 | 干净导出 `612966ec`；`go test ./internal/core -count=1 -run '<M1 相关测试表达式>' -v -timeout=10m`。完整表达式与隔离运行命令见日志首行。 | 退出 0、无 SKIP：[最终整合日志](docs/verification/meromero-m1-evidence/m1-final-integrated.log)。覆盖 direct 发布、交付作用域、同批、旧续接、模型快照、来源指纹及相关提示词合同。 |
| M1-01 真实检查器 | `go test ./internal/core -count=1 -run '^TestLiveDirectMediaDeliveryChecker$' -v -timeout=40m`；干净提交 `3da8fefc`。 | 两轮各执行无依据、否认、未来、匹配历史、无关历史五种情况，均通过、无 SKIP、退出 0：[提交后日志](docs/verification/meromero-m1-evidence/m1-01-live-postcommit.log)；[真实响应](docs/verification/meromero-m1-evidence/live-responses-05/)。请求与 Provider 返回模型标识均为 `llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4`。 |
| M1-03a | `go test ./internal/core -count=1 -run '<权限循环及直接回归表达式>' -v -timeout=10m`；提交 `5cf41aa9`，并纳入最新整合回归。 | 退出 0、无 SKIP：[提交后整合日志](docs/verification/meromero-m1-evidence/m1-integrated-postcommit.log)。真实 `HandleTurn` 使用受控 Provider，实际调用并消费了获授权的 `memory.recall`。 |
| M1-04a | `go test ./internal/core -count=1 -run '^(TestReplyContinuationKeepsOriginalCognitionModel|TestReplyContinuationLegacyCognitionUsesCurrentModelWithoutInventingSnapshot|TestReplyCognitionSnapshotKeepsAgentScopeAndLegacyBehavior|TestDirectMainReplyRetainsExistingDraftContract)$' -v -timeout=10m`。 | 提交后退出 0、无 SKIP：[日志](docs/verification/meromero-m1-evidence/m1-04a-postcommit.log)。受控 Provider 在观察续接模型后故意返回 HTTP 400，因此不证明最终媒体交付。 |
| M1-04b 受控 | `go test ./internal/core -count=1 -run '<direct 快照及发布回归表达式>' -v -timeout=10m`；提交 `612966ec`。 | 退出 0、无 SKIP：[提交后日志](docs/verification/meromero-m1-evidence/m1-04b-postcommit-controlled.log)。同一 turn `model-a→model-a`，新 turn `model-b`；验证旧记录、损坏快照、模式变化、领取冲突、来源指纹和凭据轮换边界。 |
| M1-04b 真实 direct | `go test ./internal/core -count=1 -run '^TestLiveExplicitDirectConversationDoesNotRequireASecondWriter$' -v -timeout=20m`；提交 `612966ec`。 | 退出 0、无 SKIP：[日志](docs/verification/meromero-m1-evidence/m1-04b-live-postcommit.log)、[脱敏请求与反馈摘要](docs/verification/meromero-m1-evidence/m1-04b-live-postcommit-summary.json)。该样本是同一 MeroMero 的 3 次物理请求、0 次独立 Writer；前两次工具调用分别缺少 `text`、引用格式无效，第三次完成一次 direct 认知结算。该测试没有捕获 Provider 响应的 model 字段，不以此声称后端加载实现已独立核验。 |

真实请求使用临时启动的 `mlx-serve` CLI 26.8.11，监听 `127.0.0.1:11234`，使用用户指定模型目录及原界面显示的 32K 上下文设置；图形应用显示版本为 26.11.2，二者版本不同。没有为通过测试改采样、Prompt 或 token 上限。此前真实检查器首次对否认语句识别错误，后续一次历史场景遇显存不足 HTTP 400，另一次遇回环连接拒绝；失败样本和后续通过样本均在证据目录保留。

## 失败、暂缓与范围检查

- R0 的 direct 零媒体调用红测曾发布空附件的“已发送”声明；修复前后证据见恢复报告及 M1-01 日志。M1 复核又发现短引用错绑、隐含指代漏检、普通讨论/提问误拦、旧消息后补附件时间误用等反例，均保留对应红测；受控最终矩阵通过。曾出现同批工具保护回归，修正后重跑通过。
- M1-03a 的首次实现遗漏“第三轮拒绝与合法工具同批”，红测后由 `5cf41aa9` 修正。期间曾错误地把自治授权与 direct 主链人工组合，不能代表现有 `HandleTurn`；实验测试和日志保留，但未将该实验代码纳入生产。
- M1-03b 的 [正式入口红测](docs/verification/meromero-m1-evidence/m1-03b-replay-red.log) 显示同 ID 拒绝可反复重放，直到受控 Provider 人为返回 400；窗口关闭回执也有相同条件性问题。用户指示后，实验生产改动已撤出，AEL 缺陷保留；[实验补丁说明](docs/verification/meromero-m1-evidence/m1-03b-deferred.patch.txt)及测试仅供以后复核，不是已修复证据。
- M1-04b 红测观察到同一 turn `model-a→model-b`；模式变化和旧领取者冲突各有红测及修正。凭据测试首次错误地假设“地址变化但凭据未轮换”必须失败；现有合同允许未轮换凭据继续使用原快照地址，随后改为验证地址变化且凭据轮换时不得把新凭据发往旧地址。失败与修正日志均保留。
- 未更改全局默认聊天模式、模型采样、token 限额、真实业务数据、AEL 实现、前端、Image/Moment 整体边界或其他里程碑。没有提交图片或 SVG；提交钩子正常运行。没有使用真实购买、换衣、对外发消息或真实生图。

## 任务状态、保全与后续依赖

AEL 历史父 T02 仍显示历史 `completed/verified`，该状态不代表原始 T02 全部验收完成；未删除历史记录或直接修改 AEL 数据库。M1 关联工作项已登记：M1-01、M1-04a、M1-04b 的 AEL 状态仍停在 verify，M1-03a 仍停在定义节点。当前工作区含现有未提交草稿，技术测试使用干净提交导出；为避免把不同输入的测试伪装成 AEL 工作区候选的 pass，本轮没有强行完成这些状态。实际局部结果以本报告中的提交和日志为准。

原有四处未暂存改动仍与 R0 的 `working-tree.patch` 一致：`agent_model_routing_test.go`、`media_capture_plan.go`、`media_prompt_branches.go`、`task_prompt_sources_test.go`。M1 测试使用干净提交导出，未把这些草稿混入受检源码。R0 持久备份继续保留。

如需回滚，应先保全工作区，按依赖逆序审阅并使用逐个 `git revert`，不要 reset 或覆盖上述草稿；M1-03a 的 `5cf41aa9` 依赖其前置 `c6d80502`。回退 `612966ec` 后旧代码可能不再保留新 direct turn 的重试模型快照，需先评估在途任务；本轮未执行回滚。

M2 仍需默认私聊迁移、完整 Image/Moment 插件、真实工具和媒体恢复/交付、流式与 UI 等原验收。M1-03b 按用户要求保留为非阻断缺陷，后续须明确其归属和验收处理；它没有被本报告写成已修复。
