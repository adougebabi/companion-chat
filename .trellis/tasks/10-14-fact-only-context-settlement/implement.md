# 实施与验证计划

## 开始条件

- [x] 用户已批准创建任务与规划；分支 codex/fact-only-reply-vision，基线 d696c44。
- [x] 原文、源码研究、PRD、设计与 R01–R26 映射已保存。
- [x] 最终 PRD 已收敛复读，manifest 验证通过，独立规划一致性审阅 PASS（research/planning-review.md）。
- [x] 用户于 2026-10-14 回复“开始实施”，已 task.py start。
- [x] 已加载 trellis-before-dev 并读取具体代码/spec；payload modifier 已用 ctx7 官方文档及固定版本源码核对。

一个任务按四个顺序切片执行。切片对应逻辑提交建议，实际提交在质量检查后提出一次批量计划获授权执行，不提前 commit/push。

## 1. 证据、fixture 与调用图（T0）

- [x] 复核 research/call-chains.md 的入口、参数、来源和实际 wire/SDK/UI 区别。
- [x] 在现有 Go 测试目录建立脱敏 fixture/source manifest/缺失清单及不可变摘要；缺失原件不补造。
- [x] 冻结旧 cognition/reply 装配的受控 wire baseline，标 document_excerpt 与 synthetic。
- [x] 先补能揭示策略、时间/身份、缺 turn_id 窗口问题的行为测试；记录旧实现失败原因。
- [x] 建立 R01–R06/R11/R19/R20 的可复现基线，无真实发布或真实模型依赖。

回退：仅测试/fixture/文档；不改产品行为。

## 2. 身份、历史、事实及两处投影（T1–T5）

- [x] 局部共享身份/时间/真实历史 helper，元数据独立；当前输入一次、完整片段/连续 assistant/可查回溯。
- [x] 来源/认知类型/时间/scope 投影、严格去重与结构化差异；自由文本不裁定。
- [x] Goal 先权限/profile/target 筛选，再保留原条件与真实依赖，剔除历史策略；统计口径分开。
- [x] FactOnlyCognition 接入局部 view，去无关 SOP，保留完整认知合同和能力发现/切换规则。
- [x] ReplyContext 直接从结构源装配，完整人格/有效目标/必要状态与结果保留，不从整篇 prompt 正则裁章节。
- [x] 必要时补已存在来源/有效期只读字段，不迁移表、不写领域状态。
- [x] Scene/Surface 隔离 Moments/legacy/其他 Agent；工具后新状态/人格进入 Writer。
- [x] 完成 R01–R18/R23–R26 本地覆盖及 source map/消息/DB 不变断言。

回退：Fact-only 局部入口，不删旧 Composer 或改全局默认。高风险：provider_context、assembler、projection_refresh、共享 Writer。

## 3. 参数、阶段协议与结算（T6–T7）

- [x] 新 Fact-only schema 去 purpose，阻止正文/语气/嵌套或改名策略，保留真实 refs/draft_ref/topic_key。
- [x] 服务端稳定用途满足主动去重，旧 execution_scope 仅服务端恢复；测旧新草稿、缓存 replay、改参冲突。
- [x] 交付两份最终协议；post-commit 不再要求重发，合法可选查询继续。
- [x] 原带工具无 response_format、完整 schema、一次无工具修复继续；普通采样/Thinking 不改。
- [x] 实际 HTTP/SSE 增语法坏 JSON、repair 成功/失败/取消/refs，检查工具配对、有效格式及单次 effect。
- [x] 已补 isolatedCoreTestRepository 组合测试，断言 message/draft/receipt/inbox/replay；因数据库未配置实际 SKIP，不计持久化闭环通过。
- [x] Validation 不入历史、错误正文不替换、失败不重开发送。

回退：run-local schema/协议和原修复 hook；不能撤销 receipt 或重置 published。高风险：registry、媒体恢复、去重及 request policy。

## 4. 预算、诊断、报告（T8 / 全部 R）

- [x] 原诊断补 effective choice/format/sampling、phase/lineage/来源/选择原因，默认仅 allowlist 配置。
- [x] 模块估算与实际 usage 分开，逐请求统计；受保护超预算显式失败。
- [x] 生成旧新脱敏输入/过滤理由、最终协议/schema 说明、逐项报告；不能将快照生成当通过。
- [x] 提供无真实发布回放；上下文 A/B 固定工具/模型/采样，合同变更单独比较。
- [x] 更新必要 spec/原 fact-only 文档/回退步骤，不改发布渠道或 Actions。
- [x] 全任务范围审查已完成；用户回复“提交吧”批准四笔本地工作提交；实际结果以 git log 为准，归档/日志另行处理。

## 验证命令及执行边界

规划阶段未运行产品测试。实施阶段先定向验证，选择器以实际新增测试名称补齐，不能零匹配就记通过：

```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache \
  go -C apps/core-go test -race ./internal/core \
  -run 'Test(FactOnly|RecentPrompt|CompactMemories|WakeGoalProjection|ToolFinalSchemaSeparation|PhysicalModelRequestPolicy|ConversationPhysical|ReplyEvidence|ReplyMedia|VisionOptional)' -count=1
```

定向通过后，任务最后一次全范围检查执行一次：

```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./...
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -race ./internal/core ./internal/workflow
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go vet ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go build ./...
pnpm typecheck
pnpm test
pnpm build
git diff --check
```

gofmt 检查所有实际改动 Go 文件。仅公共 API/schema 真的改变才同步 OpenAPI/客户端并 pnpm generate；本方案不计划公共 API 变更。

P 用例只用明确配置的 disposable 测试 PostgreSQL，必须经 isolatedCoreTestRepository 创建唯一数据库/清理；不把未知 DB URL 带入全套旧测试。未配置 SKIP 且列待执行，不能记持久化通过。真实模型/ComfyUI/S3/Temporal/Redis 单列 NOT_RUN，本地脚本 Provider 不算真实效果。

逐命令记录结果、测试数和 SKIP，映射 acceptance-matrix.md。仅新修改、失败或未解风险才重复扩大测试。

## 实际执行记录

详见 local-check.md、acceptance-matrix.md 和 docs/fact-only-context-settlement-report.md。阶段 3.4 四笔本地工作提交已获用户明确批准，实际记录以 git log 为准。真实模型/环境 NOT_RUN，新增数据库场景 SKIP。
