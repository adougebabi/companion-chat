# 原生工具调用与 marker 兼容迁移

## Goal

基于已验证的 MTPLX 原生 tool_calls，将媒体和待定事件从 marker 迁移到结构化工具，同时保留旧模型兼容和应用层 SSE 契约。

## Confirmed Provider Evidence

真实 MTPLX `qwen3.8-27b-abliterated-mtplx-optimized-speed` 已通过：非流式 `tool_calls`、同一 `index` 的流式参数分片、`[DONE]` 终止和 tool-result follow-up。证据与 demo 位于父任务 `research/`。

## Requirements

- capability registry 支持现有 `scene_event`、工作区已有的 `media_event` 和新的 pending-event tool contract。
- 完整累加 `index`、id、name、arguments；只在完整调用后做严格 JSON/schema 校验。
- 明确同轮多工具执行顺序、幂等/去重、causation ID、续答和失败语义。
- 原生调用优先；旧 marker parser/redactor 明确作为 fallback，malformed call fail-closed，不使用关键词触发副作用。
- 工具 JSON 不进入可见 `token` SSE；保留 `token`、`done`、`error`、`done.messages` 外层协议。
- 兼容 reasoning_content、provider 不完整 chunk、断线、未知工具和 continuation failure。

## Acceptance Criteria

- [ ] media/pending structured tools 在 mock 分片和真实 MTPLX fixture 下能落库、入队和续答。
- [ ] 旧 marker 仍能按现有严格 schema 处理，内部 JSON 不泄露，失败不会产生副作用。
- [ ] 多工具、重复调用、 malformed arguments、未知 tool、provider error 和浏览器断线均有测试。
- [ ] 现有 `media_event` 未提交实现被审查并合并为唯一能力路径，不存在重复 dispatcher。
- [ ] `npm test` 和 `/api/companion/chat` SSE 集成验证通过。

## Dependencies And Out Of Scope

- 依赖父任务 MTPLX research；依赖 shared-scene task 保持 `scene_event` 领域语义；依赖 backend child 提供 LLM/storage/queue ports。
- 不改变浏览器 SSE 外壳、不删除 marker fallback、不迁移数据库表和 API 前缀。
