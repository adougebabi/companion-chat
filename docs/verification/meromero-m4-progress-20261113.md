# M4 管理闭环与原始总验收

起点提交 `39068f2aaf225a5615a8906895bf4223ef03d1a0`，分支 `codex/fluctlight-meromero-plugin-humanization`。继承 M1～M3 报告和已完成证据。四处用户草稿保持原校验值，验证使用干净 Git 导出与明确的本单元覆盖文件。

| 单元 | 原始条目 | 唯一目标 | 状态 | 证据 |
|---|---|---|---|---|
| M4-01 | T11 14.3 开发调试权限 | 原始诊断读取和全文搜索需要 Operator 显式开启及 Owner 身份 | VERIFIED | 本目录 meromero-m4-evidence/debug-* |
| M4-03 | T11 14.3 诊断留存 | 证明已有周期清理删除过期/超量诊断并保留业务恢复 | IMPLEMENTED，待提交后复验 | diagnostic_retention_integration_test.go |
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
