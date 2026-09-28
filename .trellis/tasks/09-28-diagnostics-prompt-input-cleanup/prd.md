# 诊断中心与工作流提示词入参修复

## Goal

让 Owner 能在诊断中心看清新记录的模型轮次、阶段和实际输入输出；让活跃 LLM 场景只接收任务所需的语义、合法引用和图片，避免内部 ID、持久化元数据及重复资料干扰排查和模型判断。

## Background

用户在新记录中看到“轮次未知（旧记录） · 阶段未知”，大量 `[REDACTED]`，以及 Visual Identity、计划生成、媒体质量和 wake-up 输入中的内部字段。第 2 点的 Provider 标识虽已出现但样式不理想，用户要求暂不处理。用户补充要求检查其他 Provider/LLM 场景的同类入参问题，并确定 `request_timeout` 是否可以先靠配置解决。

已确认的新记录误标来源：非 ADK physical run 没有 `model_call_id`，读取层只给有该 ID 的行推导轮次/阶段（`apps/core-go/internal/core/provider.go:329-372`；`diagnostic_model_runs_filter.go:88,113-125`；`apps/web/src/views/DiagnosticsView.vue:74-80`）。`[REDACTED]` 在入库前生成，旧值不可恢复（`diagnostics.go:261-319,505-514`）。多模态 Eino 诊断遗漏 `UserInputMultiContent`，可能与真实 wire 不符（`eino_model_runtime.go:890-906,1058-1085`）。三类模型正文目前经过 YAML/TOON 格式化，但诊断页面把整个消息容器序列化为 JSON；两者需要分别验证（`internal/ai/prompt/format.go:20-68`；`DiagnosticsView.vue:80,209`）。详细路径见 `research/code-paths.md` 和 `research/provider-audit.md`。

用户已明确选择：**诊断中仅隐藏图片数据，其他可用文本原样保留**。这会改变现行诊断规范对凭证、隐藏推理、原始 Tool 参数和初始化原文的保护规则（`.trellis/spec/backend/fluctlight-diagnostics-contract.md:54-55,81-92`）。本任务须在代码与规范中保持一致，并保持诊断接口的 Owner 授权边界。

`request_timeout` 是覆盖完整 Agent 模型/Tool 循环的一次性 deadline（`provider.go:342`），没有五轮固定上限。设置页空绑定默认 60 秒，后端/数据库回退 120 秒，初始化最低 600 秒；实际运行环境的持久化值目前不可读取。用户决定先使用现有配置，不在本任务改动计时逻辑或默认值。

## Requirements

- **R1 诊断关联。** 新 physical model-run 显示可靠的轮次与阶段，ADK 多轮仍逐次配对，中间 Tool 请求不误标最终回答；真正缺少足够信息的历史行可显示未知。沿用既有 `09-27-adk-diagnostics-display` 的 sequence/stage 语义。
- **R2 原文诊断。** 后续诊断的 Prompt、Response、Tool 参数、reasoning 和其他可用非图片文本按原值保存、查询、展示和导出；仅图片 payload 改为 `REDACTED_IMAGE_DATA`。初始化也采用同一规则，不再一条记录仅有摘要、另一条有原文。既有 Owner-only 授权不变。
- **R3 忠实可读。** 诊断能够区分消息容器与实际 message text，显示多模态文本及图片占位；Visual Identity、计划生成、媒体质量的最终模型正文不得是误嵌的 JSON 字符串。不能把 HTTP JSON envelope 或页面的 JSON 容器展示误判为模型正文。
- **R4 指定场景入参。** Visual Identity 移除 Fluctlight/session/media/asset 等 Core 绑定 ID、`background`/`background_story` 与重复 snapshot 字段，保留真实图片、视觉事实、review/feedback 和恢复语义。计划生成移除 `foundation:*`、`goal_initial_*` 等内部标识及 goal/intention/outcome 重复元数据，保留计划所需语义。媒体质量保留冻结 concept、候选图片与重试信息，但不带非视觉背景或原始 JSON 正文。Wake-up 去掉冗余嵌套 `schedule_ref` 和无需模型看到的 wake-up 内部 ID，保留日程语义。
- **R5 其他活跃 LLM 入参。** 审查全部生产可达的非 embedding LLM 场景及后续轮次 Tool 结果；清理已证实的无用 ID、revision、hash、renderer/CAS 配置和重复结构，重点包括 schedule replan、media prompt/scene image、persona compilation、reflection/current-life projection、virtual activity、媒体及生活类 Tool 结果。保留输出 schema 或后续 Tool 确实需要的 activity/item/intention/profile/rule ID 和 `kind:ctx_*` 引用。已证实干净的 summary/initialization 正文不做无谓重构，休眠的 takeover/旧 vision/patch 路径只记录审计结论。
- **R6 超时说明。** 给出设置项、默认值、总期限边界和针对多轮调用的可操作配置建议；本任务不修改 `request_timeout` 的实现、默认值或轮次上限。
- **R7 Provider 标识。** 记录 `endpoint_id`、`model_id` 的来源与当前展示，暂不改 UI 样式。

## Acceptance Criteria

- [ ] **R1:** 新的单次和多轮 model-run 在 Owner 页面按物理请求显示正确轮次、阶段、状态及配对 Prompt/Response；真正的历史缺字段行才显示未知，刷新后顺序稳定。
- [ ] **R2:** 新记录中非图片字段不再出现因诊断脱敏生成的 `[REDACTED]`；初始化和其他 physical row 一致，Tool 参数及可用 reasoning 可查看；图片 data URL、原始 base64 与多模态 `image_url`（含远程/签名地址）不入库或导出，只显示 `REDACTED_IMAGE_DATA`；非 Owner 仍不可读。
- [ ] **R3:** 捕获的实际 Provider HTTP body 与对应 model-run 诊断逐条核对；多模态文本不丢失，图片只在 Provider 请求中保留，诊断中为占位；页面能直接辨识每条消息正文。
- [ ] **R4:** 针对四类指定场景的请求级测试证明禁用字段/重复值不在最终模型输入中，必要语义、真实图片和 Visual Identity Tool/review 流程仍可工作。
- [ ] **R5:** 审计覆盖矩阵列出每个生产可达非 embedding 场景的输入边界和结论；已证实污染的活跃路径有请求级或 Tool-result 回归，Core 内部 durable receipt、CAS、引用校验和后续合法 Tool 选择保持有效。
- [ ] **R6:** 文档明确 60/120/600 秒来源、持久化值查询方法和“多轮共用一次 deadline”；本任务无超时实现改动。
- [ ] **R7:** Provider 绑定来源和当前字段映射有记录，展示样式无改动。

## Out of Scope

- Provider 标识的视觉样式及布局。
- `request_timeout` 的计时语义、默认值、轮次上限或 Temporal deadline 修改。
- 恢复已在入库前被替换的历史文本。
- 删除休眠 Formal Agent/旧 Visual Identity API、重做对话 response schema 或其他与本次入参污染无直接因果的重构。
