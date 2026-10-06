# 执行计划与联合验收

## 当前门禁
- [x] 用户同意创建任务，状态 in_progress。
- [x] 附件完整快照、PRD 收敛、设计、研究和真实上下文清单已保存。
- [x] 用户在最终方案展示后明确回复“可以，开始实施”，已启动任务。
- [x] 产品代码、迁移和确定性业务测试已实施；真实服务验收仍待环境恢复。

## 顺序与检查点
1. **T00 基线取证**：从 entry-map 精确入口核对最新代码；固定正式 Agent/Task/Tool 清单；隔离数据复现症状，无法复现记录定位证据。保存当前 commit/digest、原始消息、Tool receipts、DB revisions、最终模型/图片请求；不把旧报告算本轮通过。核查 Actor facts 现有字段、非服装 item 表达、真实消息打扰策略与摘要配置。
2. **T01 时间统一**：App.Clock 串联业务服务和 SQL 生效查询；单一 instant formatter、参考时区和 as_of；真实审计时钟分开。验证 API/Tool/Prompt/Web 往返、unknown、跨午夜/DST 与高精度游标；不得破坏 Temporal history replay。
3. **T02/T03 基础约束**：来源可证 Actor fact 修订与小背景/查询接线，按来源联动 Memory/summary/Resident/Developing Self/generation。结构化睡眠、到期先结算、独立 Tool 生活权限；触发与真实 speaker 分开；主动冷却与合法当前段重排。以 B01—B04/C01—C04 为关口。
4. **T04 目标结果闭环**：初始化→Goal/Intention→各入口投影、正式 create/update/pause/complete/result/Reflection。关系案例验证合理等待和拒绝，购买案例验证目标不会只留人格描述。
5. **T05/T06 获取使用**：需求→查询 completeness→安排→due execution→原子 event/items/receipt/goal→独立 wear/use。补非服装与套装，按稳定业务 operation identity 做失败/超时/重启恢复，失败无 ghost item。最终 media 提交走权威拍摄快照、自由文本/参考图 fence，验证初始化来源与四种失败。
6. **T07 摘要与输入**：正式累计摘要替换旧 active episode 注入；5—10 分钟有新增/阈值触发、批次预算、atomic cursor/CAS；source 修正重建；保留完整 Tool 单元；每次物理模型调用分项预算，当前请求和必需事实不裁掉。
7. **T08 清理与唯一路径**：实现 scoped dry-run/apply/rollback 命令，隔离库污染夹具，重复执行/恢复/缓存和召回验证。仅清理可确认来源污染，未知项隔离。新链路接通后删除旧 producer/consumer/formatter/fallback，原始审计保留。正式生产 apply 命令交付但不自动执行。
8. **T09 诊断**：可在基础字段稳定后独立处理；后端 Model/Agent 固定时间+唯一键、union 后统一分页、高水位/filters cursor；Core/browser/OpenAPI/client/Web 同步，删除隐式状态/失败排序。H01/H02 实测，详情步骤正序。
9. **T10 联合验证**：下面全量检查与附件 A01—I01，两类执行测试并行存在，真实模型重复回归与 ComfyUI/S3 单独记录。不缩减既有固定矩阵，新增/修改 Tool/Agent 调用全部登记。
10. **收尾**：docs/fluctlight-goal-life-consistency-report.md 交付全部十项证据，更新领域/spec 的发现和唯一权威边界，再按项目流程提出批量 commit 计划供用户一次确认，完成后 journal/archive。

## 受影响模块与风险
- Core：Clock/ContextProjection/Actor事实、Memory/Resident/Developing Self、Summary、Goal/Intention、Schedule/Event/Activity、Tool 授权/receipt、effective life/media prompt/workflow、诊断 SQL。
- 数据：相邻 migration 编号按实际 head 分配，不修改已发布历史 migration；facts/source links、nonwearable items/usage、summary cursor、诊断分页必要字段均最小增量。
- Web/边界：httpapi/browser DTO 和 routes、OpenAPI、generated clients、Control Center store/DiagnosticsView/time formatter。
- 高风险：SQL now 与业务 Clock 分裂；纠正后的晚到 Reflection/summary worker；日程版本变更下的旧活动；Tool 已提交后 Agent 失败；多版本 media 快照混合；未知来源清理误伤。每项有独立失败夹具和 CAS/recovery 断言。
- 子代理只用于只读探索/核验；主代理亲读将修改的代码，负责最终验证，不向子代理分派写代码。

## 验证命令
完整命令沿仓库 scripts，精确新增用例在实现时填入 test-matrix.md（映射全部 A01—I01 和固定既有矩阵）。

```sh
go -C apps/core-go test -race ./...
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
./infra/acceptance/run-phase8-contract-gates.sh
./infra/acceptance/run-go-live-provider-smoke.sh --suite tools
./infra/acceptance/run-go-live-provider-smoke.sh --suite agents
./infra/acceptance/run-go-live-provider-smoke.sh --suite all
```

PG 测试使用 GO_CORE_TEST_DATABASE_URL 对应可丢弃实例及 per-test 随机数据库，禁止用户业务库。controlled HTTP/script model 只用于确定性原生 Loop，工具和 persistence 不整套 mock。live suite 要 Provider URL/模型，真实视觉另要 ComfyUI/workflow/S3 隔离配置，从已有允许配置安全加载，不输出凭据。缺依赖/零匹配/SKIP/BLOCKED/失败必须记未通过，不自动降低 suite 或声称完整 E2E。

## 必交证据
- 与原始症状逐项对应的 before/after 及代码锚点。
- 黑色短靴及至少一个非服装物品：无库存→需求→目标/日程→提前触发未持有→推进 Clock→合法活动→实际获取 event/item IDs/goal→仍未 wear/use→独立 wear/use→拍摄 snapshot/version→最终 workflow payload。
- 失败、事务失败、超时重试与重启：同业务 attempt 只一次获取，失败仍可合法重试，套装不隐式部分成功。
- 异地纠正后的总结/反思/重启/召回、明确后来回国；20 次睡眠周期静默和合法起床。
- 同一数据集完整 system/persona/runtime/background/goals/summary/messages/tools/results/schema 的分项 Tokens、count_mode、usage、摘要覆盖范围和多轮累计输入。
- 两列表 backend 精度排序/分页、frontend 过滤/刷新，以及详情正序。
- repair dry-run、apply、再次 apply、rollback、原始数据保留及状态变化日志。
- 实际模型 ID/配置/重复次数/失败样例，controlled 与 live、模拟 media 与真实像素分别说明。

## 规划收敛结果
目标/范围/边界/验收已有附件基线；没有需要用户补充的产品问题。数据库具体最小迁移结构、工具对外命名、真实环境可用性是 T00 技术核对与执行 preflight，不能据此改变已承诺行为。最终方案审批后才能进入实现；若实施发现必须改变目标或安全边界，回规划。

## 2026-10-04 Wake-up 静默原因路由修复

用户报告模型将周期检查说明放进 conversation.reply.purpose，并发送 text=no_op。
最小边界：复用 final response_intent 保存不对外发布的原因；修改正式 reply 描述/
字段说明及 Wake-up policy；领域 publication 对精确控制值返回可纠正的稳定错误；
Wake-up 将 failed Tool audit 与真正completed/accepted效果区分，保存静默诊断。
预期修改 capability_prompt_policy.go、provider_schemas.go、tool_contract.go、
tool_publication.go、agent_result_adapter.go 及定向测试/spec。
不新增诊断Tool、不解析purpose自然语言、不凭final no_op撤销真正已提交的自然回复，
不改变日程/睡眠/库存权威。验证直调Tool拒绝无消息、清醒周期误调用→失败结果→
final no_op保存原因及真实主动私聊仍可送达。

## 2026-10-04 用户背景显式初始化与设置入口

用户纠正聊天不应成为初始化入口。本次增加可选顶层actor_user.background（称呼、
职业、简单背景、所在地概况/具体地点、明确IANA时区、双方距离、见面确认）；
遗漏键不作断言，null明确未知。创建JSON分析/可编辑预览/激活透传，激活事务写入
现有actor_facts，主体绑定认证Owner，不把背景放进摇光core_persona。
详情只读展示、编辑与治理表单修改；新Owner PUT以current_facts_revision CAS，
明确correct/change、reason与idempotency key，批次事务及0049命令审计只用于恢复，
不建立第二事实权威。初始化digest包括actor_user；旧JSON未提供则保持旧语义。
核验Core创建/重放/跨Owner/CAS/原子失败/聊天纠正共享来源，BFF/客户端字段保留、
null/false、前端草稿与实例隔离及界面。未部署、未运行未知生产迁移。

## 2026-10-05 当前实拍framing枚举与第一人称全身兜底

用户明确授权：告诉MediaPrompt合法枚举；构图枚举无效时用first_person/full_body。
在原schema之外把同一枚举源写进系统指令；解析后只对无效/缺失framing启用兜底，
已知别名继续归一，合法显式构图仍保留。保留其他字段验证、额外字段和快照/衣物/
所用物品/workflow拒绝；不把姿态自由文本当作安全枚举错误吞掉。
CapturePlan保存合规full_body，CaptureFallback随冻结concept持久化（原capture留审计）；
渲染及质量视图读取同一effective capture。质量重试的新生成清除旧兜底再评估，
防止缓存prompt与重渲染冲突。普通无当前快照的media master不新增fallback。
测试：合法值全部进入指令，未知值/空值/类型错→first_person/full_body，持久化重渲染
一致、quality视图一致、原快照不变、额外衣物/坏穿着/非法pose继续拒绝；真实隔离PG
Worker捕获最终Comfy payload并验证保存标记（传输为script fixture，不声明真实图片）。

## 2026-10-05 用户澄清MediaPrompt是规范化阶段

current_capture_framing_conflict来自将Main的capture/framing与MediaPrompt输出当作
两份同级约束做字面比较。用户明确用途是模糊指令→准确标准照片信息，本次取消该
比较：Main拍摄字段是意图提示；MediaPrompt选择framing并产出标准capture对象
（模式/相机/角度/镜面/设备可见性）。正文指令明确语义解释，已保存标准plan成为
渲染及quality的唯一拍摄描述；原始提示留审计。旧5字段plan保留兼容，未知几何
提示不直接拼接或导致硬失败。身体/衣物/物品/来源/workflow事实核验保持原职责，
framing无法规范化仍按已授权first_person/full_body兜底。实测上游portrait+前置selfie
提示→全身mirror_selfie标准输出可提交；原快照/上游审计不变。

## 2026-10-05 衣柜物品中文选择表单

用户要求category/slot/ownership/availability改为下拉中文展示。ownership与availability
是后端枚举；category/slot开放名称，提供常用中文预设并联动兼容部位，不新增伪枚举。
普通物品没有穿着槽位；修补Owner Add入口支持item_kind=object/slot空并拒绝worn。
正常添加不换装/使用，批量自定义JSON留折叠高级入口，状态按钮stored改合法unavailable。
 typed payload复用现有API，失败保留描述，实例变化重置草稿；清单同步中文标签。
验证前端选项/错组合/状态/对象不穿着，隔离PG添加，界面实测和窄屏布局。


## 2026-10-06 借用试穿实施边界

缺口：商店试穿衣物没有模型可调用的借用登记/归还入口，聊天不能产生wear所需ID；
同槽位替换被模型同时填remove_slots导致冲突。复用已存在的borrowed字段和确认
衣柜Event效果写入。新增wardrobe_borrow_capability.go及定向用例，builtin注册；
wardrobe_capabilities.go补说明与可纠正反馈，capability_prompt_policy.go补顺序。
无独立试穿库存、无迁移、无自动穿着/恢复、无购买旁路。来源/Event、批量事务、
CAS、重放、归还边界通过代码与规范审阅；本轮不执行测试，build/vet单独记证据。


## 2026-10-06 场景前置提示词修正边界

用户反馈当前仍在家却叙述试穿店内衣服。调整provider_prompt_composer及共享prompt
领域权威，替代全局人格>当前事实优先级，实际保留完整权威一次并保留规则分段；
capability_prompt_policy补地点/移动→借用→穿着→照片及冲突恢复/结束本轮；
provider_context仅为摘要输出添加历史时间语义。更新出站prompt定向用例及spec，
不改持久化、能力schema、场景授权或原生循环；沿此前要求不运行测试/live服务。


## 2026-10-06 运行时兜底边界

用户新增多种实际运行错误，本次不再是提示词调整：修正借用SQL及commit契约，
补安全容量分配/出站历史压缩、stale reply刷新恢复、final引用提前校验及取消分类。
复用现有native loop、一次无Tool final repair、领域CAS、事务/receipt/outbox与总预算。
不提高窗口、不截坏Tool对、不丢来源、不自动重跑业务动作、不改变生产数据。
关键SQL和发布用例使用任务新建可丢弃PG，模型修复/预算用原生Loop脚本传输；
这些是确定性运行证据，不计为真实模型或媒体联合验收。
