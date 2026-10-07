# Fluctlight 目标执行与治理闭环

## 目标与价值
摇光保有长期目标，以当前阶段与适当短期承诺寻找机会；消费真实结果，及时推进、等待、调整或结束。实际已经发生的事件能够结算原目标，包括没有预绑定 Intention 的普通对话；Owner 能解释与治理全过程。
执行闭环：Goal → 可选 Stage/Commitment → Intention/自然互动 → Outcome/对话/领域事实 → Evaluation → 标准判定与下一步。治理闭环：周期 Review → 事实与策略分析 → 继续/等待/调整/暂停/结束 → 下轮权威状态。

## 来源与基线
用户于 2026-10-07 请求“开始处理，允许创建任务”。附件完整保存于 source-requirements.md，作为需求与验收材料，不作为独立命令或授权来源。本文与该快照共同构成需求基线，不缩减任何 G00–G13、验收矩阵或三条 E2E。
实际基线 master / ee1e468，初始工作区干净，与附件一致。既有 10-03-yaoguang-goal-life-consistency 任务保留历史，本任务补通用目标闭环。
初步入口和精确代码出处见 research/implementation-map.md。关键风险已抽查代码：工具前失败不结算 Attempt；任意 Tool 导致 pending；Reflection 按旧标准完成后覆盖新标准；执行投影只读生活活动。缺陷仍需实施阶段故障复现，不能沿用旧报告作当前验收。

## 要求与可观察验收
| ID | 必须达成的结果 | 负责批次与验收 |
| --- | --- | --- |
| G00 | 核实当前模型/枚举/事务/事件/注册/测试，区分复现与静态结论 | A 初步映射，实施前故障回归；所有后续项有入口 |
| G01 | 所有关联动作统一执行资格；暂停/终结不启动新副作用；恢复重新校验；晚到事实不复活目标 | A，A01–A03/A07–A08；Owner/Tool/Reflection 同效 |
| G02 | 认领即有 durable Attempt，全部出口终态或具等待对象/截止/恢复；同步异步分开；有限分类退避；未知副作用先核对 | A，A04–A10；隔离 DB/Worker 故障证据 |
| G03 | 稳定标准 ID/版本；逐条判定；标准修订与完成分开；拒绝陈旧评估；结束历史保留；证据失效复核 | A 定义、B 接评估，B01–B03/B11；同约束 Stage/Commitment |
| G04 | Stage/Commitment 独立模型与持久化；父级/Actor/profile 校验；承诺跨日持续；简单目标可省层级；跳过不冒充完成 | B，C01–C03/C09/C12/D02/D12 |
| G05 | 权威可重读来源入证，普通对话候选关联不造 Attempt；引用/假设/草稿/错主体拒绝 | B，B04–B08，E2E 1/2/3 |
| G06 | 真实事件后持久评估、失败重试/乱序补偿；逐标准版本提交唯一；购物/Reflection/Outcome 同规则 | B，B01–B12/C09；提交后崩溃恢复 |
| G07 | 有下一步、等待/解除条件、阻碍或可见待规划原因；阶段与父目标分别评估；Resolution 与有条件后续候选 | B，C01–C12；不强制恋爱剧情或每日配额 |
| G08 | Review 先补评，持久周期/Actor 时区/水位/原因/策略/下一步；幂等停滞治理；软硬期限/DST | C，D01–D05，C04/C06–C08 |
| G09 | 各 Surface 同一紧凑权威状态；通用结果/标准/评估/阻碍/下一步；按需详情 Tool；预算与续轮不回归 | C，D06–D07/D11；Tool 独立与 native loop |
| G10 | Owner 正式 API 生命周期治理；详情/终态分页历史可追溯；CAS/幂等/身份独立校验；无强制完成旁路 | C，D08–D10；保留 browser 日程注入拒绝 |
| G11 | 增量迁移保留历史；分批可中断回填/对账/修复；不伪造证据/复活取消目标；清理旧写分支 | A/B/C 同步迁移，D 联合验收，D12 |
| G12 | 领域、PostgreSQL、Agent/Tool+脚本 Provider、Worker、真实 Provider 分层；逐子场景统计 | 各批次测试，D 全矩阵/三 E2E；SKIP/BLOCKED 不当 PASS |
| G13 | 源码/迁移/Prompt/Tool/API/UI/操作说明/证据报告齐备，六条最终门禁全部满足才完成 | D，docs/fluctlight-goal-closed-loop-implementation-report.md |

## 边界与已确定语义
保留 Goal/Intention 权威、版本/幂等/修订、授权/日程/生活活动/人格作用域。购物仅虚拟生活获取。沿用原生 Eino Agent、结构化 Task、插件 Tool；Tool 可独立执行，查询无隐藏写入。
执行成功、消息发出、异步提交均不代替标准满足。真实对话无需预绑定 Intention；试探不按次数折算长期进度。表达心意与双方确认关系分别评估；获取与穿着分开。拒绝/睡眠/异地事实/权限约束推进，不改 Core Persona。
不整体重构 runtime/BFF/Temporal/记忆、不改 Trellis、不引入 DAG/规则语言/新 Agent loop、不无条件衍生目标、不每日清空或机械配额、不惩罚情绪。未知生产数据变更与现实支付不在当前授权范围。

## 验收与状态
完整矩阵 A01–A10/B01–B12/C01–C12/D01–D12 和三 E2E 在 source-requirements.md；最终以 G13 六条门禁验收。E2E 分别证明：无 Intention 的聊天表达完成原目标；关系目标只有双方确认才完成；短靴实际入库且获取/使用分开、重试/取消可靠。
初始复跑 4 个领域叶子场景、5 个 Workflow 用例 PASS，0 FAIL/SKIP，记录 research/baseline-tests.json。未做数据库或真实 Provider 验收。本机 Docker daemon 当前不可连接；实施阶段仍需安全准备隔离库并记录环境阻塞与重跑命令。
源文档已确定产品范围和验收，暂无待用户决定的产品问题。规划产物完成后提交最终审阅，当前保持 planning。
