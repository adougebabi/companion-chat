# M4 管理闭环与原始总验收

起点提交 `39068f2aaf225a5615a8906895bf4223ef03d1a0`，分支 `codex/fluctlight-meromero-plugin-humanization`。继承 M1～M3 报告和已完成证据。四处用户草稿保持原校验值，验证使用干净 Git 导出与明确的本单元覆盖文件。

| 单元 | 原始条目 | 唯一目标 | 状态 | 证据 |
|---|---|---|---|---|
| M4-01 | T11 14.3 开发调试权限 | 原始诊断读取和全文搜索需要 Operator 显式开启及 Owner 身份 | IMPLEMENTED，等待提交后复验 | 本目录 meromero-m4-evidence/debug-* |

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
- 配置包全部通过，`debug-config.log`，退出 0。React 诊断页面 6 项通过，无 SKIP。Web typecheck 退出 0。正式提交后统一留存最终命令与结果。
- 只读独立代码复核未发现当前单元阻断问题。复核者本机命令未配置测试库而 SKIP，未计入本单元数据库验收。上述隔离库日志由主执行者真实执行。

### 边界

本单元只收口原始诊断的读取授权。诊断写入/留存、完整 Trace 关联、通用隔离重放及全部真实页面到运行消费仍属于 T11 待办。既有正文凭据脱敏只覆盖已定义格式，保留原限制。M4 与父 T11/T12 保持未完成。普通聊天模型、Prompt、采样和工具合同未修改。
