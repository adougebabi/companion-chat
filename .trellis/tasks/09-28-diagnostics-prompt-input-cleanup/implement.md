# 实施计划

本清单只在用户审阅 `prd.md` 与 `design.md` 并明确批准进入实施后执行。各阶段改动后运行相应窄测试，最后做跨场景全量核查。修改前先确认工作树仍无其他人的未归属改动。

`fluctlight-provider-contract.md` 与 `persona-layer-contract.md` 超过 Trellis 自动注入大小限制；涉及 Provider 和人格编译的实施/核验者须在对应阶段直接读取完整文件，不能依赖截断的注入内容。

## 1. 建立红灯反馈

- [x] 为新的非 ADK model-run 建立 PostgreSQL/受控 Provider 复现：新 `provider_attempt_id`、无 ADK event 的 completed/timeout 行当前返回空 sequence、unknown stage；真正空 metrics 的旧行仍 unknown。使用 `diagnostics_test.go:475` 附近的测试缝。
- [x] 为 `redactDiagnostic`、初始化 physical row、Tool arguments、reasoning、多模态 text+image 建立输入/入库/读取/导出对照测试；当前非图片字段应红灯，图片 marker 必须出现且无 base64。
- [x] 捕获 Visual Identity、schedule generation、media quality、wake-up 的最终 Provider HTTP body，而非断言中间 JSON；准备含具体内部 ID、背景、重复字段的 fixture，使目标断言在现代码下失败。

## 2. 修复诊断链

- [x] 沿用既有 model-run DTO，在持久化/读取层补齐非 ADK physical sequence 和状态阶段；不改变 ADK event 的 Tool/最终判断，不猜测真正旧行。
- [x] 将诊断写入、读出、事件、导出及初始化包装统一为图片专用替换。保留非图片原文；旧行不做伪恢复。增加 Owner/非 Owner 访问回归。
- [x] 让 `einoMessageRaw` 记录多模态 text/image part；图片在保存前替换。修改 DiagnosticsView 的 Prompt/Response 文字呈现以区分 role/content 与结构容器，不触碰 Provider 标识样式。核对 Browser BFF/client 映射，只有字段变化时才更新生成契约。
- [x] 运行诊断/初始化/多模态/Browser UI 窄测试，确认当前真实请求和 Owner 诊断一致。

## 3. 收敛用户指出的提示词

- [x] Visual Identity 建立 provider-facing 视觉/任务状态投影，移除 Core-owned ID、背景故事、重复字段；保留图片、审查反馈、Tool 纠错和 durable 恢复。检查 media concept 的后续使用没有误删服务端 ID。
- [x] Schedule generation 用专用 goal/intention/outcome/life 语义投影，移除 `foundation:*`、`goal_initial_*`、revision/digest 与同值字段；保持日程覆盖与结果持久化 evidence。
- [x] Media quality 及共享 media concept 使用视觉字段 allowlist，确认 text part 为可读 YAML/TOON、image part 实际发往 Provider；诊断只保留图片 marker。
- [x] Wake-up 删除嵌套 `schedule_ref` 和 Core-owned wake ID；保留 schedule fact 和可作为 `influences` 使用的合法 refs。更新现有“必须保留嵌套 ref”的反向测试。
- [x] 逐个运行最终 HTTP wire 与 Visual Identity/媒体/日程/Tool 流程回归。

## 4. 审计其他活跃 Provider 场景并修复已证实污染

- [ ] 以 `research/provider-audit.md` 的完整场景总账为覆盖表，分别锁定 persona compilation、schedule replan、media prompt/scene image、reflection/shared current-life、virtual activity、native/daily cognition 的最终 wire；已清洁的 initialization 正文、summary/segment/daily memory 只记录证据。
- [x] Persona compilation 去掉 Core-owned profile/rules metadata 和重复 personality/behavior，并校正专属任务指令；更新靠内部字段路由的测试 fixture。
- [x] Schedule replan 专用投影去掉重复 Schedule、无输出用途的 refs/CAS 字段；保留 linked `intention_id`，由 Core 注入 expected revision/completion boundary，并验证 stale CAS 继续拒绝。
- [x] Media prompt 去掉 life-context CAS/renderer adapter 数值与非视觉 snapshot；损坏 frozen concept 在 Provider I/O 前失败；ComfyUI renderer 仍收到完整权重和 frozen snapshot。
- [x] Shared current-life/reflection/virtual-activity 只删请求级测试证明无消费者的字段；保留 personality switch IDs、influence/evidence refs、active activity 与 worn-item 选择键。Reflection update、memory correction 和 Tool 链接仍能提交。
- [x] 为 model-facing Tool result 建立 capability matrix，只投影状态、错误和合法 chaining key；Core 原始 receipt 不变。覆盖 image、scene、presence、schedule、moment、habit、memory、life 和 intention 的下一轮请求。
- [x] 记录休眠 takeover/旧 vision/patch 与无独立 action-realization 调用的结论，不将其无生产路径计入修复验收。

## 5. 全范围质量与规范

- [ ] 对每个修改场景运行红灯测试转绿、原始复现转绿，并抽查最终 HTTP wire、下一轮 Tool message、诊断表记录、Owner 页面；用 `rg 'DEBUG-'` 确认无临时调试输出。
- [x] Go Core：`go -C apps/core-go test ./internal/core ./internal/httpapi/...`；按相关模块补 `-race`、`go vet`。若本地缺 PostgreSQL/Temporal/ComfyUI，明确标记未运行的真实服务测试，不以 fake 测试声称完成真实 E2E。
- [ ] Web/workspace：`pnpm --filter @fluctlight/web typecheck`、`pnpm --filter @fluctlight/web test`、`pnpm --filter @fluctlight/web build`，并按 `.trellis/spec/backend/quality-guidelines.md` 做生成契约与全 workspace 检查。可用时通过本地静态服务检查诊断页及窄屏文字展示。
- [x] 最后按 backend/frontend index 的 Quality Check 做跨层审查；同步 `.trellis/spec/backend/fluctlight-diagnostics-contract.md`、相关 Provider/媒体/生活规范中被新行为取代的断言，再进行 Trellis check。确认 Provider 标识 UI 与 timeout 代码无改动。

## 风险与回退点

- 诊断非图片原文会保留凭证和隐藏推理；仅限 Owner 的现有访问控制与保留期必须通过测试，导出内容应与页面一致。
- Provider-facing 投影与 Core durable receipt 分开；每一类 Prompt/Tool 修改是独立回退点，不回滚业务数据或改变 CAS/授权。
- 多轮诊断排序不能依赖不稳定时间戳或把失败行推断为最终回答。
- 本地无法读取现网 `generic_llm.timeout_seconds`；交付时提供查询方式及调大建议，但不擅自改动用户持久化设置。

## 2026-09-28 实施证据与环境缺口

- `go -C apps/core-go test ./... -count=1`、`go -C apps/core-go vet ./...`、针对 Prompt/Tool 回归的 `go -C apps/core-go test -race ./internal/core -run ... -count=1` 均通过。
- `pnpm typecheck`、`pnpm test`、`pnpm build` 均通过；`git diff --check` 和 Trellis `task.py validate` 通过。
- 诊断原文、图片占位、多模态 text part、指定场景语义投影、Tool-result 不泄露 Core ID 的窄测试均已在修改前观察失败、修改后通过。对话/计划/媒体等生产 HTTP wire 的现有 E2E 仍依赖真实 Provider 或 PostgreSQL 环境，不将 helper 测试当成真实服务验收。
- 实施中启动 OrbStack 并运行一次性 `pgvector/pgvector:pg16` 容器，先迁移基库到当前 head，再以 `GO_CORE_TEST_DATABASE_URL` 跑完整 `go -C apps/core-go test ./internal/core -count=1`：通过。物理轮次、Visual Identity 上轮评审续传、计划重排、schedule generation/wake-up/media quality 最终 Provider HTTP wire，以及诊断写入→Owner 查询→导出测试均在隔离 PostgreSQL 上通过。媒体质量测试还对照同一次真实图片请求与 physical model-run 诊断，确认图片仅在后者成为占位。临时容器已停止并自动删除。
- 真实外部 Provider、MinIO/ComfyUI 与 Temporal 环境未配置；受控 Provider HTTP 捕获和 PostgreSQL 集成验证不等于真实模型/媒体 E2E。Owner 诊断页面未连接真实运行实例做手工浏览器验收。
