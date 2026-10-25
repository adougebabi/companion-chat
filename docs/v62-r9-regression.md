# v6.2 r9 重复问题修复与验证

日期：2026-10-25。依据最新 r9 报告和脱敏诊断，在隔离 worktree 修复。
不创建任务，不修改另一 session 工作区，不导入用户诊断内容到仓库或业务库。

## 修复与根因

| 反馈 | 本轮处理 | 验证边界 |
| --- | --- | --- |
| 初始化/日程空正文，8192 length | false 之前经常被省略；现在 Generate、Stream、legacy 均显式 enable_thinking=false。空 length 分类为可理解的截断失败，保留原输入 | 真实模型完成一次初始化及完整全天日程；不声称已证明原网关的内部推理行为 |
| 失败日程无恢复入口 | 新增原任务恢复 API、生成客户端、创建进度和日程页按钮；保留原 workflow/payload/幂等身份 | PG 恢复/重放/跨 Owner 拒绝、浏览器按钮已测 |
| 工具名字错误终止整轮 | 保留 canonical 点号名字；真正未知名字在完整原生身份验证后返回 handled/no effect，不撤销已发生行动 | Generate/Stream 受控原生循环已测。点号本身并非非法 |
| 反复查动态却无回复 | 重复目录加载先调用真实只读 query，重复事实再收敛到 Writer；执行侧拒绝阶段外动作；accepted 行动及媒体续写重置旧停滞 | 本机模型实际读出原动态并回复，未新增动态；受控重复/媒体负向已测 |
| 诊断误导成冻结重试 | 删除诊断前 64 条消息限制，保留晚到 ToolCallID/name | 180+ 条消息回归；原 r9 输入实际在增长 |
| 购物长期不结算 | 独立活动同事务登记持久到期任务，旧活动补登记，不依赖初始日程或下一轮聊天；同来源延长只生效一次，not_before 不变 | PG 真实入库、Temporal 定时/暂停恢复、历史扫描不饥饿、重放已测 |
| 买到后未换装 | 仅开始时明确选择 wear_after_acquisition 才在真实入库后用实际 ID 执行 partial wear；真实回执持久化 | 本机模型和 PG 均验证原衣物保留，重放不重复入库/穿戴 |
| 未来约定不进日程 | promise.propose 使用当前真实 Owner 消息原文；原 Goal/Commitment/Intention 决定接受，再登记对应日期排程。分开未来计划登记与实际执行窗口校验 | PG 完整提议→接受→关联日程，未接受不排程、过时窗口不阻塞、不能提前执行 |
| 地点新定义“无差异”不能应用 | preview 区分 create/update/unchanged；create_required 的新记录允许确认；增加地点中文表单 | PG 实际新增地点，浏览器保存、预览和确认动作可用 |
| 保存后分类回空、默认1F、硬编码“她”、Studio占位 | 按实际提交/读回恢复分类；空楼层保持未知、隐藏空标签；新旧向导中性文案；Studio 显示实际资源和编辑入口 | 前端行为测试、生产构建、桌面/窄屏浏览器已测 |
| 记忆为空 | 诊断含 superseded/completed，并非全部 running；明确片段在 idle 后整理、正式每日记忆在日终后形成 | segment→daily memory、长 idle 连续片段和诊断终态数据库回归已测，不伪造即时记忆 |
| 午夜穿着待确认 | 保持既有跨上海午夜穿着契约，换装由真实 Tool 改变 | R6 午夜回归包含在专项中；报告中的怀疑不作为清空穿着授权 |

购物真实回归还暴露出租约校验的错误分类：REPEATABLE READ 事务末尾的租约锁定
可能与续租更新产生 40001。现在仅 missing/token 不符才算 lease lost；保留其余
数据库错误。仅对明确回滚的 40001 最多重试三次完整短事务，沿用原操作身份；
不重复 Provider 准备，不重试断网/未知提交结果或真正失租。确定性 PG 测试已过。
原失败日志丢失了 SQLSTATE，因此不能据此断言原那次必然就是 40001。

## 实际验证

- 前端：`pnpm generate`、`pnpm typecheck`、407 项测试、生产构建通过；正式包无 fixture 数据。
- Go：本轮与 R6–R8 相关 PG 专项 `-race` 通过；本轮最终 R9/Core/Workflow/Browser 路由专项 `-race` 通过；`go vet ./...`、`go build ./...` 和格式检查通过。
- 数据库：仅使用专用 `codex-v62-r9-pg`；新增数据来自测试 fixture，不修改用户实际角色或备份库。
- 浏览器：隔离生产构建，验证地点中文保存、新记录确认可用、物品分类保存读回、中性创建标题、日程恢复反馈、390px 布局；修正验收 fixture 的版本列表响应后新页面控制台无错误。
- 模型：本机 `huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX`，调用串行，无 max_tokens/max_completion_tokens。

| 真实模型案例 | 实际结果 | 整个测试耗时 |
| --- | --- | --- |
| 初始化与初始日程 | 初始化单次完整生成，资料校验通过；随后生成 12 段连续全天日程；共 2 次物理请求 | 178.71s |
| 片段总结及动态回忆 | 总结保留未购买事实；原生 moment.inspect 读取数据库原文，再由 Writer 回复；共 6 次物理请求（含总结/Writer） | 67.24s |
| 购物、到期结果、选定穿戴、重放 | 查实际目录、修正一次无效参数后真实开始；开始时未提前入库；到期正式结果入库并 partial wear；原衣物保留、重放无重复；共 8 次物理请求（含 Writer/结果模型） | 197.34s |

购物测试只控制时间流逝；真实结果模型、Core 入库与穿戴流程没有替换成成功桩。
Temporal 测试另验证原始 due 时间等待及暂停后必须恢复才可结算。上述本机耗时是
完整测试耗时，不是界面首字延迟，也不是对用户远端模型速度的保证。

对应入口：`r9_recovery_test.go`、`r9_progress_test.go`、`r9_lease_fence_test.go`、
`r9_chat_promise_test.go`、`r9_live_test.go`、`workflow/r9_activity_resolution_test.go`，
以及 `conversation_segment_test.go` 和 R6–R8 回归。

## 未伪装成全绿的检查

不带 PG 的全量 Go 检查仍有 4 项旧失败，已将修改前 `7c59f02c` 导出到隔离目录，
运行相同四项并确认同样失败：

- TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground
- TestInitializationOwnerBackgroundKeepsStrictScalarTypes
- TestNoArgCatalogSchemaValidationAndCatalogParity
- TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase

首次全量 PG 探索使用了未迁移的测试入口库，还遇到既有 fixture/旧断言及 10 分钟
默认超时，因此不计为通过。迁移专用入口库后，本轮所需专项、片段→每日记忆与
诊断终态测试重新通过；未宣称完整 PG 套件全绿。相关本机日志保留在
`/private/tmp/r9-*.log`，原始模型请求证据仅留本机，不提交仓库。

## 保持的约束

初始化不拆分；LLM 并发仍为 1；Kev 超时仍为 10 秒；不恢复任何应用输入/输出
token 上限；自主拒绝、真实失败和未取得/未接受/未穿戴状态均保留。
