# Implementation

1. 抽取并测试 turn 接受事务，确保 API 只写用户消息/inbox/intent，Worker 唯一执行。
2. 实现服务端 turn 状态读取、观察流与鉴权取消；增加断开、取消、失败及幂等回归测试。
3. 历史 DTO/OpenAPI/browser-client 与 Pinia/UI 同步；刷新后以服务端状态恢复，传输错误只提示连接问题。
4. 补齐 WakeUp 异常终态重新安排和 `superseded` 监督修复；测试配置间隔语义。
5. 运行 Go/TypeScript 检查和涉及的数据库测试，更新 Trellis 规范，复核跨层链路。

Risk points: `agent_result_adapter.go` 接受/执行边界、Worker claim 状态、HTTP NDJSON 终态、历史分页和既有重试。每段完成后跑针对性测试；最终跑全量门禁。
