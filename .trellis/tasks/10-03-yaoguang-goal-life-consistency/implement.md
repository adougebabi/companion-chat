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
