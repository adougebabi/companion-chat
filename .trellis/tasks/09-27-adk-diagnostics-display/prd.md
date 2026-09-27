# ADK 多轮诊断展示

## Goal

让诊断中心把一次 ADK 运行中的多次“认知判断”解释为有序的模型请求与 Tool 往返，而不误导用户把中间 Response 当最终回答。

## Confirmed facts

- 每次物理 Eino Generate/Stream 都有独立 model-run row 和递增 sequence，共用父 correlation（`apps/core-go/internal/core/eino_model_runtime.go:72-164`）。
- Owner Model Runs API 当前仅返回小投影，前端按 `scenario` 平铺展示相同标题和各自 Prompt/Response（`apps/core-go/internal/core/diagnostic_model_runs_filter.go:31-74`、`apps/web/src/views/DiagnosticsView.vue:146`）。
- 公开聊天流仅包含对用户可见的消息，不可直接接入原始工具 trace（`apps/core-go/internal/httpapi/browser/ndjson.go:526-633`）。

## Requirements

- R7.1：按逻辑执行关联并标明每次物理请求的轮次、阶段（工具请求后续/最终回答等）和状态；每条 Prompt/Response 仍属于自己的 model-run ID。
- R7.2：展示安全的 Tool 调用与结果关联摘要，足以解释为什么又出现下一次模型请求；不暴露原始工具参数、隐藏推理或凭证。
- R7.3：诊断时间明确标注其采用的 Owner 查看时区；若关联用户/摇光消息，消息自身按发送当时的作者时区呈现，不依赖隐式宿主时区。
- R7.4：公开聊天只显示已提交的可见消息；不因诊断改动重复插入中间 Response。

## Acceptance criteria

- [ ] 模拟一轮两次模型请求、一次 Tool 往返，Owner 页面显示“第 1/2 次”等有序标签和中间/最终语义，各 Prompt/Response 配对正确。
- [ ] 失败、取消及未返回 Response 的请求明确标示，不把上一次 Response 贴到下一次。
- [ ] Tool 摘要只有脱敏身份/状态，公开聊天无内部 trace；多轮记录经刷新仍稳定排序。
- [ ] 时间文本带明确时区，固定 instant 的显示结果符合所选规则。

## Out of scope

- 不展示隐藏思考内容或改变 ADK 执行轮数策略。
