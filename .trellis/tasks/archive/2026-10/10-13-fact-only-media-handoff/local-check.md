# 本地检查（2026-10-13）

已实现草稿 needs_action、component refs、真实 Capture/Kev 生图包装、durable waiting、独立表/Temporal 队列/dispatcher、受限原生续接、单一正文/附件事务发布、source/cancel/supersede/主动权限 fence。0062 additive migration 不改0061。

定向 `go test -race`：ReplyMedia refs/pending/publication correlation、ReplyDraft parser、FactOnlyRegistry、ReplyLogicalIdentity、Temporal ReplyContinuation bounded retry 通过。PG publication waiting/cancel 两个子用例因 GO_CORE_TEST_DATABASE_URL 未设置而 SKIP，不记为数据库闭环通过。真实模型/媒体服务/生产回归按用户约定由用户执行。

独立只读子代理 review 检出 correlation 双前缀及 Temporal started→start crash gap，已修复。原认知 final 失败采用明确终止未发布草稿策略，保留已提交消息，不自动绕过认知合同恢复。

本地消息与附件同事务，因此不存在文本已commit但同一消息附件未commit的半条消息；重试复用已生成组件与ready草稿。当前系统没有外部逐组件delivery ack，本轮仍只报告visible_output_committed。
