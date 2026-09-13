# 静态证据与变更漂移

检查日期2026-09-08。路径相对仓库根目录，简写Go文件均在apps/core-go/internal/core，workflow/API/Web路径另注。行号是检查时锚点，优先按符号复核。
这些是此前检查发现，当前其他工作仍在修改相关代码；不保证每个缺陷在开工时仍存在。已解决项保留测试，不重复修。

| ID | 事实 | 锚点 |
| --- | --- | --- |
| E01 | runtime人格单独组装，动态块漏接，system持久值可能冲突 | provider_context.go:36/97，provider_prompt_composer.go:125/324，personality_runtime.go:177 |
| E02 | composer顶层白名单曾丢action_type/response_intent，真实后台producer有传 | provider_prompt_composer.go:345；autonomy.go/wakeup.go的realizationMessages |
| E03 | current_message+text重复；记忆预算rune+32首条可超；max_tokens为输出 | mutations.go:428；intelligence.go:537；provider.go:548 |
| E04 | analyzeCreation所有失败统一persona_invalid，不返correlation | apps/core-go/internal/httpapi/server.go:441；apps/web/src/stores/control-center.ts:77 |
| E05 | assignment在queued之前；非2xx/非JSON信息少；fallback仍success | provider.go:148/180/209/214/290/300 |
| E06 | 模型诊断输入digest稳定ID/upsert，写库错误忽略 | diagnostics.go:251/276/287/304 |
| E07 | 前端带筛选，Core export不读query，固定最近集合；用户现已排除导出，本条仅留证据，不列本批修复 | apps/web/src/stores/control-center.ts:185；apps/core-go/internal/httpapi/domain.go:605；operations.go:1443 |
| E08 | history仅event type，默认50 | apps/core-go/internal/workflow/runtime.go:114/127；workflow_runtime.go:33 |
| E09 | 单轮wake，Redis expiry推completed→retry，补种仅启动，scan不含completed | apps/core-go/internal/workflow/workflow.go:162/34/918；redis_triggers.go:42/91；apps/core-go/cmd/worker/main.go:73 |
| E10 | 真memory记录+异步embedding+最终prompt存在；召回先最近200 | memory_intelligence.go:82/392；workflow_ops.go的ProcessMemoryEmbeddingAt；intelligence.go:439；provider_prompt_composer.go:326 |
| E11 | 模型工具只有record并绑conversation，反思只record，人工revise/forget已有 | memory_intelligence.go:28/57/61；workflow_ops.go的applyReflectionCandidates；operations.go:476/528 |
| E12 | reflection visibility允许public，存储不允许；E2E紧接同会话问偏好 | provider_schemas.go的reflectionResponseSchema；memory_intelligence.go:18；scripts/e2e/persona-layers-real.mjs:449/458 |
| E13 | 普通聊天同次visible_text；仍有恢复intent及多反思来源 | mutations.go:617；cognition.go:250/385；apps/core-go/internal/workflow/workflow.go:962 |
| E14 | 队列使用diagnosticID，日志ID更改涉及执行关联 | provider_queue.go:255；provider_redis_queue.go:187/213 |

## 已有保护/其他工作必须保留

工具schema已从context去掉，已有平行身份/状态去重与当前历史去重。
工作区持续修reflection appraisal、RawMessage/bytes解码、sequence refs、Actor/关系工具注册与参数、wakeup/autonomy/schema等；不得用旧HEAD覆盖。
普通聊天单次认知已经存在；旧README与规范两阶段描述不是恢复旧行为的理由。
新规划文件在本轮写入期间数次不再存在于原任务目录，原因未确认；因此额外保存工作区外副本。不能断言是哪个人或进程删除。

## 未实测与后置风险

未读真实数据库/对话，未跑模型、token对比、故障注入、迁移或history replay；不能声称生产已复现。
媒体远端接受到本地记录job ID的窗口：media.go:133/144/165，恢复媒体前查上游恢复语义。
Worker维护串行：apps/core-go/cmd/worker/main.go:143，延迟影响待测。
群聊作者：apps/web/src/views/ChatView.vue:131，成员模型不是群聊闭环。
视觉自动触发：app.go的ensureVisualIdentityInitializationTx，图生图暂停不等于存量已停止。

## 目标/意图补充证据E15–E20

详见[目标推进诊断](goals-progression.md)第1节。已抽查引用被压缩、progress未写、24h过期读过滤、成功/失败结算不对称、UI平铺与计数、缺少目标推进验收。实际生产运行情况仍未知，不声称所有目标从未推进。

## 交接时基线刷新

最终交接HEAD为eeae15319b5709d4f782796989c313552ea6a391，分支master；git status --short为空。此前未提交改动已发生变化/进入新的提交，因此旧诊断不能当成这个HEAD的再次确认。交接只读取状态，未重新审查全部新代码。原项目任务目录当前为空，完整规划以外部备份为准；新会话恢复前先检查是否出现其他版本。
