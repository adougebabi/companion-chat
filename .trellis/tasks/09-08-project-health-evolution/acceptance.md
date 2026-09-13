# 验收矩阵

当前运行用例全部未执行。以下是拟定场景，不表示同名测试存在。A必须项与B基线分开报告。

## A必需

| ID | 对应 | 场景 | 断言/验证层 |
| --- | --- | --- | --- |
| T01 | A01 | Foundation A/runtime B | 最终请求当前主导B，记忆目标同profile，固定事实不改；请求捕获 |
| T02 | A01 | 本轮text重复且历史过去也有同文 | 本轮仅一份，不删过去真实消息；请求捕获 |
| T03 | A01 | legacy realization输入有context/action | 行动字段保留；必需缺失在Provider前失败 |
| T04 | A01/A06 | reflection map/RawMessage/bytes/appraisal | 事实不丢，引用稳定去重且不越窗口；真实DB形状 |
| T05 | C02 | 纯聊、memory_event、tool reply、重放 | 正常单次语义调用；已完成重放零调用；Embedding/反思另计 |
| T06 | A02 | 配置缺失、模型未调用 | correlation/配置原因可查，模型调用0次；跨层 |
| T07 | A02/A03 | HTTP429/500、非JSON、结构/领域失败 | stage区分、原因有界，失败不当全链成功；假HTTP+DB |
| T08 | A03 | sink满/DB坏/序列化错/ctx取消 | 业务结果不变，stderr不递归，无秘密；队列限制仍生效 |
| T09 | A03 | 同prompt同correlation两次尝试 | 两attempt，各自完整状态，不覆盖 |
| T10 | A03 | queued落库晚于terminal，停机 | 状态不回退，flush有界，丢失限制明确 |
| T11 | A04 | 从失败操作A打开诊断，库中混有B；配置/校验/部分成功夹具 | 自动只关联A；首屏显示阶段、已知原因、影响及下一步，至多一层展开见依据；无需下载或手填ID |
| T12 | A04 | 第80条workflow事件失败，自动重试后成功；另有取消/取代 | 首屏直接见最新失败与重试状态，成功后状态更新；历史按需展开保留证据；取消/取代不误报故障 |
| T13 | A04 | Temporal坏但events/model正常；超时原因未知；诊断库不可用 | 已有证据保留；partial/unknown与无记录区分，不猜根因；页面明确哪类数据读不到，不以看stderr替代页面验收 |
| T14 | A05 | Redis写失败、expiry丢失、重启 | 持久deadline保持，两个成功扫描周期内可派发，每cycle一次 |
| T15 | A05 | 新聊天延后后旧expiry迟到 | 不提前、不多加cycle；可控时钟+DB |
| T16 | A05 | 双Worker、start后回写前重启 | 单状态推进、稳定workflow、对账无重复 |
| T17 | A05 | Provider期间新聊天/取消/退休 | 旧结果不外显不覆盖，superseded可见 |
| T18 | A05 | 旧库多状态intent、活跃历史 | 迁移无复活/重复，replay或路由正确，回滚保留事实 |
| T19 | A06 | visibility允许值和public | schema/validator一致，权限不扩，合法memory可落库 |
| T20 | A06 | 合法候选memory，其余输入无答案 | 记忆独立入prompt，移除后答案来源消失，可关联ID |
| T21 | A06/A07 | 创建/索引失败/空召回/未注入 | 阶段独立可观测，Embedding失败不删事实 |
| T22 | A07 | 大tools/schema与usage缺失 | 全payload分项，字节≠token，usage缺失null |
| T23 | 全部 | 注入凭据/隐藏推理/他人记忆 | DB/stdout/页面与查询详情脱敏，所有权限边界有效 |
| T24 | A08 | 同名G/G2与不同profile、短ref重试及同批create | 正确回解目标/意图，未知/跨作用域/过期ref拒绝，同批新意图关联正确，G2不变 |
| T25 | A08 | progress等数值update、字段缺省、complete、CAS重放 | 合法值真实写回，缺省不重置，完成有证据且progress=1，数值变化有审计，重复不推进 |
| T26 | A08 | pending过期、paused上下文、取消后重试 | expired有生命周期记录，paused不执行，不能自动延期/复活；页面原因来自事实 |
| T27 | A08 | 冻结动作成功/终态失败/重试中错误 | 原goal/intention关联贯穿结果；成功与终态失败有反馈，瞬时错误不重复反思推进 |
| T28 | A08 | G含I1/I2，两次产物，I2先失败后显式重试成功 | 同一I1/G有证据从0→0.5，失败不误完成，最终G完成，重复结果无重复效果；可丢弃端到端 |
| T29 | A04/A08 | completed/paused/active混合和旧未关联行动 | 活跃计数正确，页面直见最近行动/进度依据/阻塞/下一步；未知不编造，无需导出 |

T20只证明合法候选传递与对照条件，不证明所有历史可召回，也不能以fake Provider输出证明模型质量。

## B基线：A交付验证入口和结果，B才交付能力

| ID | 归属 | 目标 | A报告 |
| --- | --- | --- | --- |
| M01 | B02 | >200条之外事实+中文改写召回 | 明确当前限制/失败，不悄悄skip改绿 |
| M02 | B03 | 纠错替代旧事实且历史保留 | record-only缺口归B，不能追加历史蒙混 |
| M03 | B02/B03 | 允许跨会话可用、私有不可泄露 | 旧权限保持，新scope未做就不宣称支持 |
| M04 | B01 | 全请求预算与必要信息保护 | A仅计量，不宣称硬预算已实现 |
| M05 | B04 | 合并反思不丢事实、成功无空workflow | A只记当前基线 |
| M06 | B02/B04 | 长程回忆/未知承认/行为连续 | 固定版本，多样本结果，不给虚构通过率 |
| M07 | B05 | 到期/event触发、qualified、失败重规划、持续聊天期间到期 | A只保证既有路径最小闭环；未有正式消费者/重规划策略时明确B05未完成 |

M类单独baseline报告，写expected current limitation、actual、证据、后续归属；B实施后才提升为必需通过。

## 真实记忆实验

1. 独立测试角色与会话写难以猜测的合成事实，记录memory ID/revision/source；使用memory_event与反思入口分别测。
2. 让原话退出近期12条，并捕获最终payload确认Foundation/summary/关系/目标/本轮问题也不含答案；不能只数发送轮数。
3. 同一短期输入对照提供/移除目标memory，检查候选→选中→prompt，再检查回答。测试控制用依赖注入或fixture，不加生产越权开关。
4. 固定Provider/model/参数，真实场景至少3个样本记录；确定性检索/合同每次必须通过，模型结果保留分布。
5. >200、自动修正、新scope归M01–M03。紧接原话答对不能代替长程验证。
6. 显式测试base URL和fixture命名空间；脚本会真实登录、调用模型、创建数据，只能在确认的可丢弃环境运行，不对生产角色注入。

## 证据落盘

实施阶段verification/包含environment.md（版本无密钥）、results.md（ID/pass/fail/skip/reason/路径）、payload-metrics.json、memory-baseline.json、workflow-recovery.md、migration-rollback.md。
缺环境跳过单列；exit0但关键测试skip不满足验收。假Provider、真实DB、真实Temporal、真实模型证据分开。
禁止.env、真实用户对话、访问令牌、原始隐藏推理进入证据。效果数字带样本数与方法。
本轮只做文档覆盖/链接/状态检查，没有运行上表用例。
