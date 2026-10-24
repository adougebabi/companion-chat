# v6.2-r3 修复与复验记录

报告来源：用户《摇光-从零走查-v6.2-r3.md》，SHA-256
`1d6d84b1db297611c17a514521a574565d4be3ac14a51dabd8cca314783cb7c5`。

## 版本与范围

报告测试版本是 `4eed0858`，没有包含上一批独立分支修复 `670875d8`。
按用户要求，先在提示词优化提交基础上应用该修复，形成 `7e94f7c8`，
并将集成说明 `53728da2` 推送到 `codex/fluctlight-functional-recovery`。
本批 r3 修改继续在其后提交；原主工作区未改动。

用户明确保持单次初始化生成。请求仍不发送 `max_tokens` /
`max_completion_tokens`，不拆分或设置分段配额。
`finish_reason=length` 是已观察到的终止结果；仅凭报告不能确认
8192 的来源，本批不把原因直接归到模型通道。

## 问题处理矩阵

| 报告项 | 本批处理 | 证据与复验边界 |
| --- | --- | --- |
| 四用途全部必填 | 未配置用途可留空；部分填写时校验必要字段，向量不再阻挡保存 | 浏览器只保存服务、再只绑定 generic_llm / conversation_reply 均成功 |
| 未保存连接测试失败 | 明确保存后读取模型目录；不再请求不存在的新服务 ID，改动中的连接不使用旧目录结果 | 受控 HTTP 请求日志 + 页面反馈 |
| 配置刷新丢草稿 | Owner 范围内恢复普通连接字段和原 CAS 基线；密钥不持久化 | 字段/URL 凭据排除、基线过期测试和浏览器刷新 |
| 新服务旧凭据文案、后台式长表单 | 按已有凭据显示清除项；协议选择；可选用途和其他配置折叠；章节聚焦 | 浏览器 AX 状态和实际表单 |
| 初始化截断被报成连接不可用 | 保留 typed length 原因和安全 usage；跳过此类 final 修复，显示准确反馈 | Generate / Stream 受控 HTTP；大描述 Haiku 成功率仍需真实复测 |
| 整理等待无计时/不能停止 | 显示实际等待秒数；显式 AbortSignal；停止或离页后保留输入；返回/刷新回到可编辑步骤，旧响应不影响新尝试 | 浏览器分析请求与观察中断对应；停止/离页/恢复/晚响应回归 |
| JSON 坏文件/错误位置、重复错误、步骤焦点、空模板死路 | 中文语法位置提示、可编辑内容、去重字段错误、步骤聚焦滚动、发布模板入口/空选项禁用 | 客户端回归与浏览器 |
| 原激活/资料编译 503 长等待、关页草稿 | 已集成上一批持久化草稿、短事务受理/后台编译 | 隔离 PG；用户需对集成版本复测 |
| C1 工具重复调用 | 合法空参数保持有效；accepted waiting 进入原 continuation；相同草稿的重复 needs_action 无进展时终止 | 原生 fresh call ID 的 1/2/3 次模型分支受控回归；报告实际回合仍需关联 trace 复验 |
| C2 换衣声称完成 / C3 相近动态 / 内部动作叙述 | 保留真实原生工具与发布契约，不把文字当执行，不按相似正文删记录 | 需要新版本下实际调用/回执和消息来源，未宣称已全部解决 |
| R5 日程 started 与反复生成 | 真实生成失败走既有两次 Activity 重试后显式失败；CAS 过期仍等待新快照 | SQL 无伪造日程 + Temporal 终态测试 |
| 串行后唤醒门槛 | 按已接受的初始日程放行，长期运行的 scheduler 无需 completed | 完整候选 SQL 回归 |
| C7 空日程无调整入口 | 空态提供原有日程安排/调整工作台入口，保留现有非覆盖安排契约 | 规范路由，无虚构条目 |
| C6 日记日期/无反馈 | 点击前显示明确日期；空日期可选；pending/running/no_content/failed 单独反馈 | 受控浏览器选 10-22、排队、无内容与读回；今天仍不能生成 |
| C8 摘要重复与记忆为零 | 片段摘要纳入逻辑租约，重新检查状态/epoch，完成后不重复修订；诊断区分片段/累计/每日归纳 | 并发真实 PG 一次模型调用；未自动生成记忆，真实 memory_event 仍需回合证据 |
| C9 通知 | 按角色分组计数，保留每条 notice 的阅读操作；预览只读取精确授权消息 | 5 条分组，读一条后剩 4；外部/外角色/晚响应不采用 |
| C10 诊断 | 场景映射包括 metrics；初始化无角色 ID 显示创建中；列表不再直接泄漏 JSON/表格/英文原文 | 原内容保留在详情；渲染回归 |
| 图片 no rows / 衣柜格式或方案失败 | 配置读取处给出明确图片服务缺失提示；穿搭格式/业务方案失败分别说明 | 缺配置真实 SQL；没有生成通用衣柜或把失败标就绪 |
| C11 旧库升级 | 增加可预览的人格画像维护说明及 README 入口，启动门禁保留 | [升级说明](../operations/persona-upgrade.md) |
| 首次设置 | 令牌来源、当前页面密码要求和确认密码 | 确认不一致不调用认证，保留精确密码文本 |
| 首聊耗时 | 保留并发 1；现有用户/回复优先级 100，高于初始化 60 | 已执行中的模型调用仍占槽位；未声称消除真实模型延迟 |

## 验证

- 工作区生成、类型检查、生产构建、客户端测试及全 Web 回归按本批最终代码执行。
- Web 全量 **392 项通过**；工作区类型检查、生成同步、生产构建及独立 mock 构建通过。
- 本批 Go Generate/Stream、原生等待/无进展分支、SQL 持久化、初始化顺序与
  Temporal 重试测试通过；相关 Core race 检查通过。
- 生成客户端与生成模板同步，没有额外 HTTP body / OpenAPI 变更。
- Core Client 3 项、Browser Client 40 项通过；Go vet、Go 全量构建与
  本批修改文件 gofmt 检查通过。
- 隔离 PostgreSQL：`codex-v62-r3-retest-pg`，tmpfs / 25432；
  每个测试独立子库，没有修改用户其他容器。
- SQL 回归证明日程失败不被吞为 perpetual pending、同片段并发只生成一次、
  Visual Identity 缺配置给出明确原因；原有片段到每日记忆与源修正回归保留。
- Temporal 回归证明日程失败按两次 Activity 尝试停止，正常时区检查保持。
- 浏览器使用生产构建 + 受控 HTTP，配置保存、草稿恢复、目录读取、
  日记终态、通知单条阅读、分析中断及离页恢复均有实际请求和页面证据。
  这是交互/契约验证，不是用户真实模型成功的替代证明。

浏览器证据：
[日记无内容反馈](evidence/v62-r3/v62-r3-diary-no-content.png)、
[通知分组/来源预览](evidence/v62-r3/v62-r3-notification-groups.png)、
[停止后保留输入](evidence/v62-r3/v62-r3-analysis-cancelled.png)、
[离页返回保留输入](evidence/v62-r3/v62-r3-analysis-resumed.png)、
[390px 同源 iframe](evidence/v62-r3/v62-r3-diary-390px.png)。

390px 检查修复了新日期控件挤压标题与日历横向溢出。
它不是实际移动设备、跨浏览器或完整可访问性验收。

Go 全量仍有原基线 4 个 Core 测试与 2 个 browser 路由矩阵失败：
`TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground`、
`TestInitializationOwnerBackgroundKeepsStrictScalarTypes`、
`TestNoArgCatalogSchemaValidationAndCatalogParity`、
`TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase`、
`TestBrowserRouteMatrixMatchesOpenAPIArtifact`、`TestSurfaceRoutesMatchCoreAndMatrix`。
本批相关执行测试单独记录，
不将跳过 SQL 或基线失败写成全量通过。

## 建议优先真实复测

以最终提交对应的 Web / API / Worker 同版运行，数据库保持 `0101_serial_llm_queue`。
优先检查：单次大描述初始化、换厚外套、请求一条带图动态、明确要求记住一件事。
若仍失败，请在报告保留 turn/correlation、实际 native tool name/arguments normalization
前后大小与工具 result、发布消息/动态来源；无需提供密钥。
这些证据能区分未选择工具、参数协议不匹配、已受理异步等待及实际写入失败。
