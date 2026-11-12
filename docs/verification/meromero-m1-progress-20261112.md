# M1 基础可靠性：当前进度

当前里程碑：M1；开始基线 `988c7cc3b7166dd736d00dde45ba81295256f023`。
状态口径：只有正式路径的实现、对应验收和直接回归均有证据才标 `VERIFIED`；本表不是总体百分比。

| 子任务 | 原始需求 / 唯一目标 | 前置条件 | 状态 | 改动、验证、证据与阻塞 |
| --- | --- | --- | --- | --- |
| M1-01 | T04/T06：direct 媒体完成声明必须与所指任务的真实交付一致；普通交流及有依据的历史表达可用 | 既有 R0 红测 | VERIFIED | 提交 `3da8fefc228240084252f4e07e1aa55718b43425`。提交后干净源码、隔离 PostgreSQL 和受控 Provider 正式路径矩阵 `meromero-m1-evidence/m1-01-postcommit-matrix.log` 通过；同一提交两轮真实 MeroMero 检查器 `m1-01-live-postcommit.log` 全部合同通过，均退出 0、无 SKIP。实际真实模型服务为本机 CLI 26.8.11，图形应用显示 26.11.2；不代表真实生图或整条聊天链验收。旧显存不足、连接失败及回归红测均保留。 |
| M1-02 | T06：当前和历史媒体引用的作用域、任务身份与交付状态核验 | 并入 M1-01 | VERIFIED | 与 M1-01 同一提交和正式路径矩阵验证已提交消息附件、素材与引用、会话/权限作用域、同轮工具回执、历史任务匹配及事务重查；无附件的 queued/ready/failed 不视为交付。旧消息后补附件的确切交付时间标为未知。测试资产为隔离夹具，不代表真实渲染。 |
| M1-03 | T06：工具拒绝、终止错误、重复调用及发布边界 | 按实际调用链核对 | BLOCKED | M1-03a 已验收；同 ID 重放缺陷按用户要求登记后暂缓，其他策略拒绝和终止错误未作完整验收。此状态是用户指定的非阻断暂缓，不是环境故障，也不能据此声称该项全部通过。 |
| M1-03a | T06：正式私聊中同一无权限工具改变参数和调用 ID 的重复尝试有界，随后正常回复 | M1-01 已完成；不覆盖四处原有草稿 | VERIFIED | 首次提交 `c6d80502` 后发现第三轮拒绝与合法工具同批可能误拦，修正提交 `5cf41aa955fbfabd032440a099875fc745fcfd89`；最新提交的隔离库整合回归 `meromero-m1-evidence/m1-integrated-postcommit.log` 退出 0、无 SKIP。三轮按物理模型响应计数，同批合法工具仍可执行。此前人工组合自治授权和 direct 主链的实验不代表实际 `HandleTurn`；测试与失败日志保留，实验代码未纳入生产。 |
| M1-03b | T06：同一无权限工具跨不同模型响应重复相同调用 ID 时有界结束，且保留真实回执幂等 | M1-03a | BLOCKED | 用户明确要求非阻断问题只登记、不修复，AEL `meromero-m1-03b-replayed-denial` 已登记并暂缓；这不是主链环境故障。正式 `HandleTurn` 红测、实验代码和日志保存在 `meromero-m1-evidence/m1-03b-*`；实验性生产改动已从工作树撤出，不声称此缺陷已修复。 |
| M1-04 | T02：主链必要模型快照和旧任务兼容策略 | 既有路由/快照实现 | VERIFIED | 继承 `recovery-batch-01-review.md` 的真实 variant 覆盖、隔离和撤销；M1-04a/04b 分别验证旧草稿与新 direct turn 的恢复策略。旧媒体等保留现有 `legacy_unfrozen` 当前选模，旧衣柜、Diary、Studio 缺可靠快照时按各自已有合同显式失败；不能保证旧模型。未迁移默认主链、逐请求持久诊断和其他 T02 原验收仍未完成。 |
| M1-04a | T02：私聊续接有快照时恢复原绑定、旧记录无快照时采用当前绑定并标明原模型未知 | 既有私聊续接及隔离数据库 | VERIFIED | 提交 `dce89ce816aaaf2a6c272028eae0088e7a818422`；提交后隔离库回归 `meromero-m1-evidence/m1-04a-postcommit.log` 退出 0、无 SKIP。受控 Provider 下观察到有快照续接 `main-a→main-a`，无快照旧记录为 `main-a→main-b` 且记录 `legacy_unfrozen`、未造快照。测试刻意在观察请求后返回 HTTP 400，未验证最终媒体交付。 |
| M1-04b | T02：新的 direct 私聊在恢复或重试同一逻辑 turn 时保持原模型绑定 | 既有模型路由和隔离数据库 | VERIFIED | 提交 `612966ec5eb9c742a7dc15b23e436ed39b9db816`；提交后 `m1-04b-postcommit-controlled.log` 退出 0、无 SKIP，真实 MeroMero direct 回归 `m1-04b-live-postcommit.log` 通过。受控重试为 `model-a→model-a`，新 turn 使用 `model-b`；旧记录标记 `legacy_unfrozen`，损坏快照不静默重绑。真实提交后样本为同一 MeroMero 的 3 次物理请求、0 次独立 Writer，前两次分别因缺 `text` 和引用无效被拒绝。 |
| M1-05 | T02/T04/T06：M1 集成验收、阻塞复核和交付 | 前述必需项完成或明确阻塞 | VERIFIED | 最新提交 `612966ec` 的干净源码整合回归 `meromero-m1-evidence/m1-final-integrated.log` 退出 0、无 SKIP；里程碑回执为 `M1-review-for-chatgpt.md`，结论为 PARTIAL，明确用户暂缓项及后续归属。 |

父 T02 的历史 `completed/verified` 不代表原始验收全部完成；M1 使用独立关联工作项保留原历史。M2–M4 本轮不实施。

M1-01 的完整矩阵首次重跑发现同批拒绝后的普通回复被新预筛选误送检查器；修正后上述最终矩阵通过。反例的红测、失败矩阵和三次真实调用日志保留在 `meromero-m1-evidence/`。这些受控 Provider、合成资产和数据库夹具不等于真实模型或真实生图。局部字符串预筛选和语义提取存在自然语言覆盖限制，不代表任意事实均可判真。
