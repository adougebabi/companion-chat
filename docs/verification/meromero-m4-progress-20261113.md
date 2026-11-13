# M4 管理闭环与原始总验收

起点提交 `39068f2aaf225a5615a8906895bf4223ef03d1a0`，分支 `codex/fluctlight-meromero-plugin-humanization`。继承 M1～M3 报告和已完成证据。四处用户草稿保持原校验值，验证使用干净 Git 导出与明确的本单元覆盖文件。

| 单元 | 原始条目 | 唯一目标 | 状态 | 证据 |
|---|---|---|---|---|
| M4-01 | T11 14.3 开发调试权限 | 原始诊断读取和全文搜索需要 Operator 显式开启及 Owner 身份 | VERIFIED | 本目录 meromero-m4-evidence/debug-* |
| M4-04 | T11 页面→运行消费 | 模型 variant 页面保存、重载与正式 reflection 请求消费及撤销恢复 | VERIFIED | management-react-http-twice.log |
| M4-03 | T11 14.3 诊断留存 | 证明已有周期清理删除过期/超量诊断并保留业务恢复 | VERIFIED | diagnostic_retention_integration_test.go |
| M4-02 | T11 14.4 默认只读重放 | 在单个已存 correlation 范围内逐条回放记录，保留失败和分页缺口 | VERIFIED | 正式调用详情及现有 diagnostic-records GET |

## M4-01

启动配置 `FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ` 默认 false。API 将其注入 App。开启后需 API 重启，Compose 需重建 Core 容器。该开关授权当前 Owner 读取完整诊断，运行时页面不能自行授予。非法布尔值拒绝启动。此次没有修改实际私有环境文件或运行实例。

Core 检查覆盖原始 records 列表/详情、旧 events/model/media/lifecycle/Kev、workflow payload inspection/status 和分页原始 history。全文搜索 q/qAny/q_any 在导出与清理计数查询之前也受同一权限约束。元数据白名单导出和已有 Agent 操作摘要保持可读。业务命令继续执行原授权，调试授权仅用于读取。

页面展示明确授权提示。新旧 Browser 及内部 HTTP 入口均需正确保留 403 与 diagnostic_debug_read_disabled。

### 已执行及失败保留

- 干净基线加新增测试的红测实际失败，原始读取默认放行。`debug-red.log`，退出 1。
- 初版 Core 14 项定向测试通过，无 SKIP。`debug-green.log`，退出 0。
- HTTP 首次测试发现旧适配丢失领域权限错误，返回 502/通用 403。首次修正又将已转换的错误二次包装为 502。两次样本分别保存在 `debug-http.log` 和 `debug-http-fix1.log`，退出 1。第二次局部修正后 HTTP 矩阵通过，`debug-http-fix2.log`，退出 0。
- 完整受影响 Core 测试集合首次有三个既有诊断读取断言未显式授权，`debug-core-regression.log`，退出 1。补齐三个测试的独立 App 授权后，相同集合 74 个顶层测试通过，`debug-core-regression-fix1.log`，退出 0，无 SKIP。
- HTTP/Browser 全包首轮由于干净导出缺少两份既有跨目录测试输入而失败，`debug-http-regression.log`。补充 creation-controller.mjs 与 browser-client/openapi.json 导出后相同包测试全部通过，`debug-http-regression-fixture1.log`，退出 0，无 SKIP。
- 配置包全部通过，`debug-config.log`，退出 0。React 诊断页面 6 项通过，无 SKIP。Web typecheck 退出 0。提交 `8ca56aa5` 后 74 项 Core、HTTP/Browser/配置全包与 API 编译通过，无 SKIP；React 6 项提交后复验通过，无 SKIP。命令及版本分别见 `debug-core-postcommit.log`、`debug-http-postcommit.log`、`debug-web-postcommit.log`。
- 只读独立代码复核未发现当前单元阻断问题。复核者本机命令未配置测试库而 SKIP，未计入本单元数据库验收。上述隔离库日志由主执行者真实执行。

### 边界

本单元只收口原始诊断的读取授权。诊断留存待独立验证、完整 Trace 关联、通用隔离重放及全部真实页面到运行消费仍属于 T11 待办。既有正文凭据脱敏只覆盖已定义格式，保留原限制。M4 与父 T11/T12 保持未完成。普通聊天模型、Prompt、采样和工具合同未修改。

M4-01 收口提交 `8ca56aa56abc9c7bd48ed7b71ab01f892e9df017`。下一单元按 T11 核实已有留存边界与只读重放。

## M4-02

本单元复用现有带调试授权的正式诊断 GET。调用详情可打开只读记录回放，按时间与来源/引用显示事件、物理模型、Agent result.Trace、回合、工作流链接、媒体、异步及决策记录。全部来源分别分页，时间相同不宣称精确执行顺序。各来源保持自己的创建成员快照，状态为读时值。保留读取错误和后续页提示，缺少输入/回执/发布证据不能推定完成。

该单元是已存记录的逐条查看。重新执行模型、工具、媒体恢复与工作流控制不属于回放行为。跨异步关联完整性、未存输入恢复和隔离重新推理仍待 T11 总验收。诊断写入涉及 Provider、工具事件与 Kev 直写，已定位为独立待办，本单元不修改其业务恢复账本。

### M4-02 已执行与失败

- 首次 typecheck 因 useSyncExternalStore 快照类型 unknown 失败，补显式会话类型后通过，分别见 `replay-typecheck.log` / `replay-typecheck-fix1.log`。
- 首轮 React 回放测试发现后续页插入较早记录时默认选中项漂移，`replay-web.log` 退出 1。按记录键固定首次选中后相同集合通过，`replay-web-fix1.log` 退出 0。补正式调用详情入口测试后 13 项通过，`replay-web-entry.log`，无 SKIP。
- 真实 Browser HTTP/隔离 PostgreSQL 对九类 GET 的读取测试通过，三个相同 call_id 的 requested/failed/completed 均保留。tool_executions、conversation_messages、media_intents、workflow_control_commands 行数读取前后相等。`replay-http.log` 退出 0，无 SKIP。受控数据与 nil runtime，不代表真实模型/Temporal执行或端到端浏览器。
- Web build 退出 0，生产图扫描通过。保留既有大于 500kB chunk 警告，无本单元打包优化。`replay-build.log`。
- 独立只读复核未发现指定范围阻断，复核者 React 13 项通过。四用户草稿保持原 SHA-256。

M4-02 收口于提交 `0c0e259b8a9f300ed936ef31d49bbf7b8e954bb0`。提交后 HTTP/Browser/配置包及 API 编译退出 0，React 13 项退出 0，无 SKIP，typecheck 退出 0。证据为 `replay-*-postcommit.log`。

## M4-03

原文没有要求新增默认关闭原始采集的政策。本项复用 Worker 已有每 15 分钟清理、30 天和每表最新 10,000 行目标，补验收，不增加 capture 开关、迁移或另一套存储。

隔离 PostgreSQL 中四类诊断表分别写入 10,002 条合成记录，默认 PruneDiagnostics 删除 8 条过期/超量数据，各表剩余 10,000 条，再次清理删除 0。agent_runs 的整行 JSON 在清理前后完全相同，业务恢复 Trace 保留。`retention.log` 退出 0，无 SKIP。实际验证为清理函数和持久化结果，Worker 的 15 分钟接线来自代码核查，未等待真实长周期。两次清理之间可暂时超过目标行数，不能宣称数据库始终有硬上限。原始采集与读取授权是独立事项，此处未改为新政策。

M4-03 提交 `43cb0753` 后同一隔离库验收通过，退出 0，无 SKIP，见 `retention-postcommit.log`。

M4-04 VERIFIED，对应 T11 页面配置→回读→下一次正式任务消费。复用真实 React 管理页、BrowserClient、正式 HTTP 与 PostgreSQL。Provider 是透明受控响应，合成 session 只属于测试，不计入真实模型或原生浏览器 E2E。

### M4-04 实现与失败保留

测试专用 React 场景使用正式 ManagementPage、BrowserClient 及 configuration provider，经真实回环 HTTP 服务保存 variant 覆盖、重新挂载页面回读，再由正式 BuildContextProjectionFor / RunReflectionProposalTask 请求受控 Provider 并 DecodeReflectionProposalV2 校验。撤销覆盖后重复页面回读与正式任务，观察默认模型恢复。本测试不改业务模型、Prompt、参数或生产配置。

初始夹具提取时 context 被提前取消；之后手工投影缺少身份和 reference index，改为正式上下文构建后发现 affect/profile habits/compiled persona 等初始化前置缺失。按现有生产前置补齐合成记录并用正式 BackfillWorkingPersonas 准备人格。最后一次失败已完成页面保存/重载，但正式 configuration facade 的通知计时器使子进程未退出，90秒被测试 context 结束。测试跟踪并清理该计时器后完整场景通过。所有失败分别保存在 `management-react-http*.log`，未使用跳过或放宽原合同。

首次完整通过 `management-react-http-timer.log`，退出 0、无 SKIP。后续在两个新隔离数据库、全新页面进程中按相同输入重复两次，全部通过，退出 0，无 SKIP。`management-react-http-twice.log` 记录各阶段正式请求模型、页面来源回读与配置版本。Provider 模型标识为 m4-primary/m4-override 的合成服务，session 解析器与 CSRF 传输为测试夹具。该结果属于 React+真实HTTP/数据库+受控Provider 跨层集成，不是本机真实模型或原生浏览器 E2E。

提交 f62a11ee 后，全 HTTP/Browser/配置包与 API 编译通过、无 SKIP。两轮提交后专项首轮通过、第二轮测试在异步 catalog 返回前查找 variant 选项而失败，management-postcommit.log 保留。补测试等待真实选项出现，生产页面与路由未修改，待新提交复验。

M4-04 在提交 aeb21237 的干净源码上两轮独立场景复验通过，退出 0，无 SKIP。`management-postcommit-wait1.log`。M4-05 开始补原生浏览器相同管理入口的保存→浏览器刷新→正式运行证据，使用本测试的隔离后端和生产 Web 构建。

## M4-05 原生浏览器模型管理

状态 VERIFIED。正式生产 Web 构建由一次性回环服务提供。控制的 session 及 CSRF 属于测试夹具，Provider 请求使用同一受控模型服务。

实际浏览器进入管理中心的模型用途，选择正式 reflection_proposal_v2，确认默认来源后执行正式任务。页面保存 m4-override，真实浏览器 reload 后重新打开该变体，读回任务变体来源，正式任务请求 m4-override 并通过合同。恢复继承并保存、再次 reload 后读回默认来源，正式任务请求 m4-primary 并通过合同。三个阶段及配置版本见 `management-native.log`，退出 0，无 SKIP。API 原回执见 `native-initial.json` / `native-override.json` / `native-restored.json` / `native-finished.json`。

浏览器首个临时标签在读取选项时失效，现有浏览器清单确认标签已消失；新建同一 IAB 标签并保持后完成操作，未重建数据库或覆盖任何失败样本。截图仅在仓库外 `/private/tmp/fluctlight-m4-native-override.jpg` 和 `/private/tmp/fluctlight-m4-native-restored.jpg`，不提交图片。

该原生浏览器证据对应受检生产提交 aeb21237 和本单元测试夹具覆盖文件，生产 Web 源相对构建未变。完整应用页面、真实 BrowserClient、正式 Handler、PostgreSQL 和 typed reflection 任务真实执行；Provider 响应与身份是受控夹具。未作为真实模型推理证据。

M4-05 的三个实际覆盖文件与提交 76dbc6e1 按 SHA-256 完全一致，生产实现相对浏览器受检版本未变。提交后 HTTP/Browser/配置全包及 API 编译通过，无 SKIP。详见 native-source-and-screenshots.txt 与 native-postcommit-regression.log。

M4-06 VERIFIED，补插件开关从真实 React 页面经 HTTP 保存、重新挂载回读，再由正式独立 Tool 消费同一配置。仅使用一次性库的合成文字动态，图片保持停用以验证拒绝，不调用外部生图。

### M4-06 实际结果

真实 React 配置页分别保存 plugins.domains 的 false/false（version 1）和 false/true（version 2），真实 HTTP 回读后重新挂载页面，开关状态与保存值一致。正式 ExecuteTool 在前一阶段拒绝 Image 与 Moment，后一阶段只允许 Moment。后者通过现有独立 Writer 生成纯文本并在隔离库持久化一条动态；图片 intent 为 0，其他正式 reflection 任务继续使用默认模型并通过合同。受控 Provider 不代表真实推理或渲染。日志 `plugin-management-http-contract1.log` 退出 0，无 SKIP。

首轮测试沿用了 legacy 的 text 参数，当前默认模式的独立 Moment 正式合同只接收 evidence_refs，故返回 handled/needs_input。测试已改用现行无正文参数，并给受控 Writer 返回纯文本，未切换默认模式或修改业务合同。失败保存在 `plugin-management-http.log`。另外测试断言的 moments 归属列名在静态核对时修正为 owner_fluctlight_id，首轮未执行到该计数断言。React 异步轮询已包入测试 act，原警告保留在首轮日志。

M4-06 提交 9f55287b 后，两次新隔离数据库、新页面进程的同一模型+插件场景串行复验通过，退出 0，无 SKIP，见 plugin-management-postcommit.log。

## M4-07 表达审核页面到新 direct 请求

状态 IMPLEMENTED，待提交后复验。真实 ExpressionLearningPanel / createSelfProvider / BrowserClient 经隔离 HTTP 开启学习与参考，从正式 AcceptTurn 的合成 Owner 消息提取候选、审核并设置触发词。重新挂载回读“使用中”后，正式 HandleTurn 的实际请求包含候选。页面停用并重新挂载回读“已停用”后，新 HandleTurn 请求不再含 expression_suggestions。两次均正常发布，已有单主 direct 合同通过；模型响应为受控原生 ToolCall，不代表真实模型自主模仿。

保留首轮 Go 复合字面量语法错误（测试未真正运行），修正后表单测试错误假设了 aria-label，按正式 label/htmlFor 读取字段。之后受控响应使用 Core capability 名 conversation.reply，实际物理请求公布的是 conversation_reply，导致原生最终合同失败；观察实际请求工具名后改按已公布 wire 名回复。两次失败及只读观察记录分别保存在 expression-management-http*.log。未修改正式工具 schema、校验或生产路由。完整场景在 expression-management-http-wire1.log 退出 0、无 SKIP。
