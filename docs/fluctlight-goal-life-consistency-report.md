# 摇光目标驱动生活与事实一致性实施报告

更新：2026-10-04。分支 `codex/goal-life-consistency`，基线 `528bb97`。

代码与确定性闭环已实施，真实服务联合验收尚未完成。任务保持
`in_progress`，本轮源码已提交，未推送、未部署、未对生产库执行修复。
本报告中的“确定性通过”指真实隔离 PostgreSQL、正式 Tool 或原生 Eino/ADK
配合脚本 Provider 的可重复验证，不能替代真实模型行为或真实图片验收。

## 实際定位与修改

| 症状 | 确认的代码原因/修改 | 正式入口 |
| --- | --- | --- |
| 目标只在人格描述里、下一轮难以推进 | 将已有Goal/Intention持久权威加入required Runtime；execution按已有Activity派生；共享NULL profile保持共享，native goal_ref可复用目标 | goal_execution.go、intention_capabilities.go、provider_context.go |
| 异地纠正后旧同楼断言回来 | 增加qualified Actor属性事实与revision/source依赖；过滤Memory、Active、Resident、Self、overlay，源消息edit/delete撤回资格；晚Reflection/summary事务CAS | actor_facts.go、reflection_runtime_v2.go、runtime_summary.go |
| 定时检查当真人发话/自然醒 | system trigger显式区分目标与speaker；typed sleep在领域门禁；20次睡眠检查no-op，不产生生活经历 | sleep_cycle.go、wakeup.go、life_behavior.go |
| 不存在衣物进入实拍 | 获取/穿着分离；来源核验；冻结Life snapshot，闭合style plan；最终worker重渲染并校验workflow条件/参考来源 | inventory_source.go、current_capture.go、media.go |
| 聊天原文与历史摘要重复增长 | sole累计摘要与覆盖游标，最多40条批次/4条尾部；required摘要替换覆盖raw；每次物理调用计完整wire预算 | runtime_summary.go、prompt_context_assembler.go、eino_model_runtime.go |
| 诊断旧失败排在新成功前 | 后端先union/全局固定时间与id倒序再分页，NULLS LAST与PG snapshot membership；前端不按失败重排 | diagnostic_page.go、agent_run_diagnostics.go、DiagnosticsView.vue |
| 八小时时间混淆 | 公共instant formatter，明确reference_timezone/as_of，未知用户zone不推断；业务Clock与审计时钟分开 | internal/instant、instant.go、message_time.go |

用户原现场未提供完整购买Tool trace，因此不能追认原现场曾经成功购买或换装。
本轮根因结论依据入口代码与隔离复现，不把用户观察直接等同于已确认的数据库事件。

## 复用能力、新增契约与旧路径

保留formal Agent/Task registry、Eino原生Loop、独立ExecuteTool、receipt ledger、
领域事务/outbox与Temporal。Goal/Intention、Schedule/Event/Activity、wardrobe、
Summary formal Agent、Memory治理和诊断存储均复用现有权威。

新增/实质扩展：`actor.inspect`、`actor.fact.record`、`item.use`，普通库存类型和
套装购买，当前实拍style plan与最终workflow guard，proactive topic/purpose，
累计摘要/cursor，修复CLI及诊断page envelope。受影响入口包括Main/WakeUp/
Reflection/native cognition/daily review、购买日程/Activity、MediaPrompt/worker。
正式独立Tool adapter固定矩阵现在35项，原有项保留；严格live runner清单未缩减。

旧累计producer `settleConversationSummary` 与固定块选择器移出production，仅作为
历史回归fixture保留在 `_test.go`；历史episode显式 `memory.recall` 继续可用。
Runtime只读取累计摘要。失败自由文本不能提交baseline编译人格；图片不回写生活事实。

## 数据、时间和配置

新head `0048_proactive_topics`，迁移依次为0043 Actor facts、0044 inventory usage、
0045 runtime summary、0046 diagnostic pagination、0047 history repair、0048 proactive topics。
迁移测试包含空库→head、相邻head升级/reapply；源撤回SQL增量进入新迁移，未改已发布历史SQL。

- 公共瞬时：`2026-10-03T11:26:18.000+00:00`；LLM历史统一reference zone。
- Storage/cursor保留DB真实精度；user原话、prompt/audit snapshot不重写。
- DST本地缺失钟点拒绝；重复钟点选择较早瞬时；显式offset输入优先。
- `product.summary.interval_seconds` 300–600，默认300；`max_runes` 512–4096，默认2048。
- `product.autonomy.topic_suppression_seconds` 300–604800，默认43200。
- 物理输入预算复用现有Provider binding/context/max-input/output-reserve与安全余量；
  required overflow明确失败。summary显式请求`enable_thinking=false`，诊断声明provider支持/实际模式未验证。

Actor correction有一项保守限制：模型生成产物关联该次调用全部可见Actor事实，
不声称精确到每candidate；无关事实纠正也可能排除产物，需要重新生成。
叠加项失效会使旧Working Persona source/hash不匹配并失败关闭，需要现有受控编译流程重编译。
当前reference workflow验证也较保守，库存/衣着版本变化可能拒绝视觉上仍可用的旧参考。

## 成功、失败与最终输入证据

完整确定性链路日志（原始产物已清理，见精简验收记录）
保存boots与brush两个子场景。每条JSON包含Goal/Intention、scheduled receipt、Activity、
acquisition Event/item IDs、独立wear/use receipt、capture snapshot与final workflow。
[短靴链](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/chain-0.json)和
[画笔链](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/chain-1.json)可独立复核。

链路断言：查询为空仍能建立描述Goal；未来未执行无item；Clock推进启动；未到最短
耗时结果accepted；实际完成后item入库且wear/use不变；相同请求replay；随后独立穿着/
使用；最终conditioning只能来自冻结authority。失败购买测试断言无库存、无意图完成；
已提交Tool后final错误与重启不得重复mutation。详见life_activity_capabilities_test.go和
精简验收记录（原始日志已清理）。该失败证据不是实际部署模型timeout实验。

[实际worker传输捕获](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/final-comfy.json)
是正式MediaPrompt→worker→`/prompt`的真实HTTP request body（使用受控Transport）。
其中白衬衫/银项链来自冻结着装，愿望短靴未进入；renderer返回503后状态不变。
model-clothing/workflow-clothing两种恶意覆盖提交计数均为0。成功链生成的workflow
与这个worker submit测试为互补证据，尚无同一live run贯穿Provider/Comfy/S3。

Actor纠正重载、source edit/delete、Active/Resident/Self依赖过滤、晚snapshot拒绝、
overlay重载均有测试。20 sleeping cycles断言model调用0、assistant0、shopping0、
新增cognition/reflection evidence0；confirmed wake只在业务到期时成为current Event。

## 同数据集输入预算

[完整分项trace](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/budget.json)
使用相同40条消息、相同系统协议/人格/Tool/schema/current input，摘要覆盖1–36，保留37–40。
摘要32 runes，重要纠正“用户在国外、具体时区未知”和未完成购买保留。

| 估算项 | before | after |
| --- | ---: | ---: |
| 完整wire estimated input tokens | 8703 | 2032 |
| Recent | 7631 | 764 |
| Summary | 0 | 62 |
| System（含protocol/persona） | 671 | 671 |
| Runtime time view | 248 | 248 |
| Current input | 51 | 51 |
| Tool schema | 114 | 114 |
| Response schema | 28 | 28 |
| Wire bytes | 13988 | 2739 |

估算方法为UTF-8/runes保守公式加image allowance，count_mode=estimated。
section subtotal不是总wire精确和；protocol/persona/runtime.*是父项内明细。
实际部署模型usage/tokenizer对比仍未获得，不能将8703/2032描述成精确token数。
多轮native Tool loop以及超大Tool结果的物理调用门禁有独立测试。

[五轮物理请求预算](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/physical-budget.json)
记录run_id=`turn:native-budget-turn-1`，每轮估算输入42297、43866、44866、45681、46141；
max_input=49152，output/safety各4096。Tool结果从0增长到2684 tokens，五轮均独立检查。
累计计费输入估算222851不等于单轮context占用；这组也不是真实Provider usage。

## 诊断后端与浏览器

后端测试覆盖same instant/ns精度、NULL尾部、new success/old failure、union先排序、
status更新不退页、晚到backdated insert不进入旧snapshot、filter/cursor绑定/异常拒绝。
Core→BFF snake/camel envelope→generated clients→Web同步。

浏览器在本地production dist静态服务+明确fixture API运行；独立检查Agent/model页、
load-more保持new success先于old failure、刷新保留`agent-runs`、timezone offset。
发现并修复partial old page被后台poll抹掉；增加可执行测试及实际再次检查。
390px viewport和scrollWidth均390，console error日志为空。
fixture未运行真实授权/服务链；页面原有默认分组提示因fixture不提供创建接口，
不把它记为真实业务错误或完整浏览器E2E通过。

浏览器截图已按用户要求删除；检查结果保留于 browser-qa-result.json。

## 清理CLI实际调用与生产范围

全部执行在可丢弃库`codex_goal_life_cli_20261004`，合成owner/fluctlight/item清单明确。
fixture（原始产物已清理，见精简验收记录）、
manifest（原始产物已清理，见精简验收记录）、
[dry-run](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/repair-cli-dry-run.json)。

```sh
# CORE_GO_DATABASE_URL由操作员设置为已确认的目标库，不把凭据写进命令日志。
go -C apps/core-go run ./cmd/history-pollution-repair --owner OWNER --reason REASON --manifest manifest.json
# 人工审阅exact IDs、before/source与digest后才应用：
go -C apps/core-go run ./cmd/history-pollution-repair --owner OWNER --reason REASON --manifest manifest.json --apply --expected-plan-digest DIGEST
# 重复apply回放稳定batch，回滚必须没有后续变更：
go -C apps/core-go run ./cmd/history-pollution-repair --owner OWNER --reason REASON --rollback-batch BATCH_ID
```

实际batch `history_repair_a5a05ec3244207bba796207d27e45170`：apply applied/replayed=false，
apply replay applied/true，rollback rolled_back/false，rollback replay rolled_back/true。
最终输出JSON保留；第一次apply发现typed nil map导致CLI输出null并跳过操作，已修复后重跑。
程序不会扫描/清空未知生产库，没有`--all`；生产manifest尚需Owner按真实来源审阅。
periodic-only负例另验证mixed真实来源与confirmed合法wake不得清理。

## 全部38场景状态

“部分通过”保留具体不足；“真实行为阻塞”不计验收PASS。
表中代码路径在`apps/core-go/internal/core/`，证据在任务`evidence/`。

| 编号 | 状态 | 测试/证据 | 断言及限制 |
| --- | --- | --- | --- |
| A01 | 确定性通过 | goal_life_chain_test.go | 初始化共享关系目标进入持久化与下一步投影 |
| A02 | 真实行为阻塞 | 精简验收记录（原始日志已清理） | 模型服务拒绝连接；未证明自然关系探索语义 |
| A03 | 部分通过 | proactive_topics_test.go / sleep_cycle_test.go | 重复主题与睡眠门禁通过；明确拒绝的长期模型行为待验证 |
| A04 | 部分通过 | reflection_runtime_v2.go / evolution_persistence_test.go | 复盘来源与下一步权威有契约；错失机会的模型语义未实测 |
| B01 | 确定性通过 | actor_facts_test.go | 同楼纠正、常驻背景、来源派生退出当前事实 |
| B02 | 部分通过 | actor_facts_test.go / runtime_summary_test.go | 重载/源失效/晚提交/摘要纠正输入通过；多轮真实模型跨天待测 |
| B03 | 确定性通过 | actor_facts_test.go | change保留历史有效区间，correct明确不同 |
| B04 | 确定性通过 | actor_facts_test.go | 外主体/角色自述不升格事实；system无真人speaker |
| C01 | 确定性通过 | sleep_cycle_test.go | 20轮无model/assistant/activity/新反思证据 |
| C02 | 确定性通过 | sleep_cycle_test.go | confirmed wake到期后才生效 |
| C03 | 确定性通过 | goal_life_chain_test.go / schedule_tool_regression_test.go | 提前触发不执行；冲突保留旧状态 |
| C04 | 确定性通过 | schedule_tool_regression_test.go | 当前剩余段合法修改，历史/不可中断/执行中拒绝 |
| D01 | 确定性通过 | goal_life_chain_test.go | 缺物无需库存ID即可建Goal/Intention |
| D02 | 部分通过 | formal_tool_adapter_e2e_test.go | 独立查询失败有正式失败结果；失败后模型不重复买的语义待live |
| D03 | 确定性通过 | goal_life_chain_test.go | 未来日程链接真实目标/意图，提前检查不持有 |
| D04 | 确定性通过 | goal_life_chain_test.go | 推进Clock，正式due trigger启动购物并结算 |
| D05 | 确定性通过 | chain-0.json | 获取event/item IDs，Goal完成，未自动wear |
| D06 | 确定性通过 | life_activity_capabilities_test.go / 精简验收记录（原始日志已清理） | 失败无物品/无意图完成；重复成功尝试重用item |
| D07 | 确定性通过 | shopping_items.go / item_use_capability_test.go | 单件/套装/普通物品原子获取，独立使用 |
| D08 | 确定性通过 | scheduled_activity_closure_test.go / durable_turn_test.go | 固定尝试重放、重启及取消/改期权威回归 |
| E01 | 确定性通过 | current_capture_test.go | 持有未穿仍渲染权威旧衣物 |
| E02 | 确定性通过 | inventory_source.go / wardrobe_capabilities.go | 跨属主/无来源/失效物品失败，状态不变 |
| E03 | 确定性通过 | chain-0.json | 独立wear后snapshot与最终workflow含已持有物品 |
| E04 | 确定性通过 | current_capture_test.go | model clothing和workflow追加覆盖在submit前拒绝 |
| E05 | 部分通过 | final-comfy.json | 模拟renderer失败不反写事实；真实图片/S3/像素尚阻塞 |
| E06 | 确定性通过 | 精简验收记录（原始日志已清理） | 受控初始化合法，未溯源历史不补造 |
| F01 | 确定性通过 | runtime_summary_test.go | 300..600秒有新增触发，覆盖raw替换，无新增不重复 |
| F02 | 确定性通过 | runtime_summary_test.go | 生成中新消息carry-forward；写库失败不推进；删除重建 |
| F03 | 确定性通过 | prompt_context_assembler_test.go | 完整recent单元与物理Tool续接预算保护 |
| F04 | 部分通过 | budget.json / conversation_segment_test.go | 同数据集预算及跨天历史回归通过；长期真实对话语义待测 |
| F05 | 确定性通过 | prompt_context_assembler_test.go / turn_chain_budget_test.go | 每次物理调用计Tool结果/多模态；超限显式失败 |
| G01 | 确定性通过 | instant_test.go / internal/instant | Z归一数字偏移、UTC毫秒，unknown不猜 |
| G02 | 确定性通过 | instant_test.go / instant-display.test.mjs | 同瞬时投影、跨午夜、明确海外zone |
| G03 | 确定性通过 | instant_test.go | LA/Berlin缺失拒绝、重复选择较早瞬时 |
| G04 | 部分通过 | instant.go | opaque原始审计不重写；未知裸历史未批量迁移，需实际历史清单核查 |
| H01 | 确定性通过 | diagnostic_page_test.go | 全局固定时间/id DESC，null尾部，状态不置顶 |
| H02 | 确定性通过 | diagnostic_page_test.go / diagnostics.test.mjs | 快照多页/filter/cursor；浏览器append与partial-page暂停轮询 |
| I01 | 确定性通过 | repair-cli-apply.json / repair-cli-rollback.json | 实际CLI dry-run/apply/replay/rollback/replay；原始保留 |

## 执行检查与外部阻塞

检查命令在implement.md，结果摘要在任务evidence/。原始日志已按用户要求清理。`go test -race -json ./...`使用隔离PG，
`go vet ./...`、`go build ./...`，`pnpm generate/typecheck/test/build`，
`run-phase8-contract-gates.sh`、`git diff --check`均实际执行。
最新完整race日志：1641个test/subtest通过，0失败，24个外部/live测试跳过；
15个package通过、10个无测试package跳过。随后新增periodic repair负例、physical budget
日志断言及instant日志回归各自通过。浏览器客户端14、Web59测试通过；core-client
没有test脚本，记录generate/typecheck通过。数量亦保存于`validation-summary.json`。
原始与重复日志均已清理；完整race的统计和跳过名单保留于validation-summary.json。

严格live tools/agents/all确实运行过但没有通过。`100.80.75.9:8001`连接拒绝errno61；
配置模型为`qwen3.8-27b-abliterated-mtplx-optimized-speed`。ComfyUI8188可连，但缺
`FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE`验收配置，不能完成真实workflow/S3/像素链。
preflight原始日志已清理，阻塞结果保留于validation-summary.json与live-connectivity-final.json；本轮未擅自换服务或模型。

待完成：实际模型多轮/重复关系与纠正语义；实际模型usage预算对比；实际ComfyUI/S3
完整闭环及像素评价；按真实来源审阅生产repair清单。任务不归档，不声明T10完整通过。
在这些必需验收补齐前不提出“全部完成”或执行提交/推送。后续提交仍需按Trellis
流程给出具体分批提交方案供用户一次审阅。


## 2026-10-04 提交与测试产物清理

用户明确授权提交及删除部分测试数据。保留测试源码、必要合成链路结果与检查摘要，
删除临时seed SQL/manifest、fixture服务、截图、重复及原始运行日志。
[精简证据说明](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/README.md)。
真实模型与视觉验收继续待完成，任务保持in_progress。

源码提交：`d6636ef`（既有tracked改动）及`e86ab1f`（补齐新增源码、迁移与测试）。

## 2026-10-04 Wake-up 静默原因补充修复

用户提供的native ToolCall将内部周期检查说明放入conversation.reply.purpose，
text为no_op。当前源码原本已在最终publication边界拒绝精确no_op；仅有ToolCall
记录不能证明该值已实际送达，也没有本轮运行实例trace证明线上二进制版本。
本次确认并修复的是模型契约与诊断保存：共享reply描述不再暗示必须输出当前轮，
WakeUp policy/schema明确final response_intent承载内部静默原因；周期result持久化
该字段。对精确控制值返回稳定reply_control_value_invalid及明确纠正指引，并保留
typed domain error，避免被invalid_arguments覆盖。failed/rejected Tool尝试继续留审计，
但不再仅因有result就把静默周期改成completed/capability。

正式native Loop回归复现同类误调用：首次请求conversation.reply{text:no_op}，
领域拒绝且无消息写入；第二轮读取真实失败结果并返回no_op/response_intent；
周期no_op、原因保存、assistant消息数0，重放无额外Provider调用。独立普通/自主
Tool执行同时覆盖大小写与空白、noop/no-op/none；自然文本提及no_op仍可发送。
真正主动私聊及无消息生活活动的原有回归继续验证。没有新增诊断Tool或自然语言
关键词门禁；本次未部署，实际Provider的新行为仍需运行新版本后验证。

本次定向race验证128个test/subtest通过、0失败、4个live外部依赖跳过；
vet/build通过，独立只读核验无发现。本补充修复尚未提交或部署。


## 2026-10-04 补齐actor_user显式初始化与资料治理

前轮缺少用户资料的显式配置入口，聊天录入不能替代该需求。本次补齐可选顶层
actor_user.background，与core_persona并列；字段为name、occupation、background、
location_scope、location、timezone、relationship_distance、meeting_confirmed。
遗漏字段不作断言，null表示未知，false不丢失；时区须明确有效IANA，不取设备/摇光时区。
初始化规范化、JSON导入预览及激活链保留此字段，进入来源/激活digest，并在同一创建
事务写既有Actor事实。认证Owner是固定主体。
详情新增只读用户背景；编辑与治理提供独立表单。Owner PUT拥有版本CAS、批次事务、
correct/change区别与稳定请求重放；新0049_actor_user_background仅增加命令审计表，
当前事实仍只在actor_facts，后续聊天纠正不另存一套。

初始化相关44个test/subtest通过，6个DB依赖跳过（含3个本次集成用例）；BFF边界race
通过，浏览器客户端15/Web62测试通过，生成/typecheck/build/vet通过。实际浏览器以
production dist+syntheticAPI核对详情查看、治理保存反馈，390px无横向溢出、无console错误。
目前Docker未运行且Mac锁定，无法启动OrbStack；真实PG的激活、CAS、重放、批次回滚
及0049迁移验证待用户启动容器服务后继续。没有将这些skip或UI fixture计为数据库通过。
此补充改动尚未提交、部署或执行生产迁移。


## 2026-10-05 framing合法枚举与第一人称full_body恢复

用户明确指定枚举错误的恢复方式，本次仅针对current-capture的framing，不替换
普通media master的失败策略。合法枚举从schema生成到系统指令；unknown/缺失framing
恢复为full_body，effective capture=first_person/rear/hidden device。合法显式构图
继续保留。其他非法样式枚举、额外衣物/身体/物品字段、坏快照与workflow override
继续失败；不从自由文本pose中解析或容忍未经来源的服装。
prepared concept保留原capture与context_binding，另外保存capture_plan_fallback，
确保提交前cache重渲染一致，质量检查看到相同effective capture；新生成不继承旧标记。
真实隔离PG+脚本Provider/Comfy传输验证38个定向test/subtest通过（含此前未跑的
actor_user初始化/Owner CAS/重放/批次回滚）；更宽媒体/上下文/Prompt race回归
164通过、0失败、3 live skip，vet/build通过。已恢复数据库验证，0049迁移通过。
[精简结果与最终传输入参](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/capture-framing-fallback-result.json)。
Comfy传输为受控fixture，真实像素级全身/第一人称效果仍未验证。未提交或部署本次补丁。


## 2026-10-05 MediaPrompt规范化用途澄清

上一轮只恢复非法framing枚举，仍对Main提示与MediaPrompt结果做字面比较，导致
current_capture_framing_conflict。用户明确用途是模糊生图指令→准确标准照片信息。
本次将拍摄提示与领域事实分开：Main的capture/framing供模型解释；MediaPrompt
输出标准framing/pose/expression/lighting/style/capture相机关系，渲染与quality以
准备好的方案为准，不再存在字面framing conflict代码路径。原提示留审计，衣物/
身体/物品和工作流的程序事实核验继续保留，非法framing输出保留first_person/full_body兜底。

真实隔离PG+脚本传输测试166 pass、0 fail、3 live skip；vet/build通过。验证上游
portrait/selfie/front提示可规范成全身镜面自拍并到达最终传输，而非准备阶段失败；
相机/镜面/设备可见性和quality视图一致。
[结果与最终标准照片入参](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/media-prompt-normalization-result.json)。
没有宣称真实模型长期合规或真实像素效果通过，本补丁未提交/部署。


## 2026-10-05 衣柜物品中文表单

主动添加由默认JSON改为中文选择表单。类别与部位按UI常用预设联动；它们在领域
仍是开放名称，未添加假的闭合集。所有权owned/borrowed/unknown及可用状态
available/unavailable/lost使用中文选择，原stored状态按钮修正为unavailable。
普通物品kind=object，slot空且不穿着；Owner Add写入口复用既有库存权威与来源，
增加对象约束、同实例ID更新限定和失效穿着链接清理。正常添加不自动穿着/使用，
批量自定义JSON保留折叠高级入口。
验证：真实隔离PG wardrobe/inventory race23通过、0失败、0skip；客户端15、Web64
通过；typecheck/build/vet通过。浏览器production构建+syntheticAPI实测分类联动、
中文状态、提交反馈及对象无槽位；390px无横向溢出、无console errors。
[精简结果](../.trellis/tasks/10-03-yaoguang-goal-life-consistency/evidence/wardrobe-form-result.json)。
本次未提交或部署，未对生产数据执行操作。


## 2026-10-06 换装拍照与场景日程对齐提示词

用户报告聊天宣称换另一套衣服拍照却未调用换装 Tool，以及店内场景与书房工作日程
不一致；明确要求只调整提示词、不运行测试。变更边界为正式 Conversation、WakeUp、
DailyReview 的 operation_rules，新增共享 capabilityLifeConsistencyInstruction 并在
三个入口和其上下文刷新绑定中使用；不修改领域状态、Tool schema、权限或持久化。
指令要求服装不同时先 wardrobe.wear completed 并确认当前穿着，再生成照片和回复；
缺少 item IDs 时查询库存，不把购买/计划/失败当成穿上。照片按冻结快照描述，生成
accepted 不当作完成。场景冲突先核对有效 life_context/时间/活动；有效 Event 保持
权威。结束活动/返回计划通过获准活动结算与 scene_event，继续活动通过获准日程
查询与编辑/重排处理，保留历史及可中断边界；消费真实结果和刷新状态后再描述。
WakeUp 的“静默无需 Tool”明确限于静默本身，允许必要的生活状态处理。
本轮未运行测试、构建、模型或媒体回归，只做源码审阅和 git diff --check；提示词
是否使实际模型稳定调用 Tool 尚未验证。本轮未提交、部署或修改生产数据。


## 2026-10-06 店内借用试穿与partial换装修复

用户确认复用borrowed所有权并要求实施。trace明确蓝衬衫有owned/available ID，
partial失败来自选中upper_body同时remove_slots；原第二套店内衣物缺少正式记录。
最小实现新增wardrobe.borrow/wardrobe.return正式事务Tool，复用既有confirmed
wardrobe_gain/wardrobe_unavailable Event效果、库存来源、Owner/autonomy授权、
wardrobe revision CAS及Tool ledger；批量最多8件，同事务/outbox/receipt，不自动
穿着、不完成购买。借用返回真实ID，归还仅允许可用borrowed wearable并解除穿着、
设unavailable；历史所有权/来源保留。已结束库存Event不接管活动场景，无新迁移。
工具说明和共享提示词明确店内借用→wear→照片、归还及独立恢复自有穿着；partial
自动替换槽位，冲突详情直接指导删除重叠remove_slots，不升级full。
补齐真实领域隔离PG回归用例（未执行）；本轮仅源码核对、gofmt、build、vet和
差异检查，不启动测试数据库或媒体服务。没有新增行为/live通过证据。
未提交、部署或操作生产数据；总任务真实模型联合验收仍未完成。


## 2026-10-06 场景前置与提示词权威/结构优化

用户澄清模型未切换场景，在家叙述试穿不存在的店内服装。所贴prompt已含借用流程，
缺口是前置地点/实际状态和冲突恢复；不是规则完全缺失。源码确认成功Tool会markDirty，
下一物理请求沿绑定projection重读当前状态，未发现需要新增刷新通道的依据。
原composer过滤ProviderContextAuthorityRule为冗余，但短协议没有完整领域权威；且
core_persona > developing_self > current_state被写作全局优先级，长规则换行被压平。
本次改为身份行为/领域事实分权；single/multi运行协议实际嵌入共享权威一次，明确
worn_items及wearing查询、有效Event和历史/摘要ending_state边界；摘要输出视图增加
historical_conversation标签。主聊天/周期/日审规则按职责、步骤、失败结束分段，
renderer保留换行。店内链明确地点→获准移动scene_event及刷新→实际借用→wear→拍照；
不能在家为旧台词制造借用，不能拿自有不同衣服冒充目标款。属性不同则查询/承认未确认，
禁止无证据发错图/缓存/换回解释；无新条件不重复失败调用，说明阻碍并结束本轮。
修改只涉及提示词/Provider输出视图，不修改Tool领域校验/场景权威或加硬循环次数限制。
补出站system与摘要历史标记回归用例，未执行测试或live服务；build/vet与diff检查
单独记为静态证据。本次未提交或部署，模型遵循性尚待真实使用观察。


## 2026-10-06 运行时恢复与SQL真实回归

用户报告SQL42804、working_memory_required_budget_exceeded、工具结果预算超限、
life_context_stale、decision_influence_0_ref_unknown及request_cancelled。
借用SQL增加显式timestamptz；真实隔离PG进一步复现缺少Event replay-ready字段导致
提交P0001，补齐真实id/revision/status/expected/resulting_context_revision/replayed，
未改触发器或域校验。class42 SQL错误返回非重试失败及无提交说明；事务/来源保持。
WorkingMemory必需片段优先，分区可在总输入预算内借用容量；物理续轮超限时清理
出站副本中的旧推理及完整可选历史轮，保留system/Runtime/当前输入/全部Tool对，
原始Eino/audit不变。最新必需结果仍过大保持拒绝，记录压缩前后估算。
发布CAS失败改life_context_stale普通反馈并markDirty，下一物理轮重读后重组织回复。
真实PG验证旧“在家”不发布、新“在商店”只发布一次。完整形状但未知ref提前进入
已有一次tool-free final repair，不删除引用或重新执行动作；codec允许集与刷新后
实际索引一致，修复模型预算不带未发送的Tool schema。取消优先正确分类，保留取消，
不自动重启或更改已有终态DB契约。
本轮实际执行隔离PG及脚本HTTP native-loop定向race回归，最终结果见精简evidence；
build/vet/diff检查另记。没有实际Qwen长期行为或媒体像素验收，没有生产数据操作。
临时数据库/日志清理，保留精简结果；未提交或部署。


### 补充：stale回复恢复后的提交锚点

定向检查发现仅markDirty可让新回复发送，但原AuthorityAtRunStart仍使最终receipt
链比较失败。新增仅服务器可写的AfterRecoveryCallID/StaleReplyCallID：实际stale
reply失败后、下一物理轮确实重读状态才使用新锚点；边界覆盖重读前已观察的所有
调用，之前的effects/outcomes/audit保留，后续receipt继续CAS。普通刷新不改锚点，
无真实失败/伪造边界拒绝，之后外部变化仍拒绝。真实PG补验先wear成功→外部场景
改变→旧reply失败→刷新→新reply一次发布→提交链成功；旧wear仍有效，普通旧锚点
及伪造恢复标记均拒绝。无整体Agent重启或重复Tool执行。

最终本轮定向race回归51个test/subtest通过、0失败、0skip；build/vet/diff通过。
精简结果保存在任务evidence/runtime-recovery-result.json；任务临时PG/卷与原始日志已清理。
