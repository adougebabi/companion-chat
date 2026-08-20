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

## Confirmed Delivery Decisions

- The canonical internal envelope is one `CapabilityCall` with the provider call id, non-negative provider index, capability name, accumulated argument text, parsed arguments, source (`native` or `marker`), persona id, causation user-message id, and a deterministic idempotency key. Native and marker adapters must produce this envelope before dispatch.
- The registry exposes exactly three application capabilities in this migration: `scene_event`, `media_event`, and `pending_event`. Each entry owns its tool schema, marker adapter, cardinality limit, validator, durable effect handler, and continuation result shape. There is one dispatcher, not one dispatcher per capability.
- Provider order is the order of non-negative `index` values, with first-seen order as the tie-breaker. A duplicate call for a capability whose cardinality is one is rejected for that capability; other capability calls continue in order. `media_event.count` remains the supported way to request up to three media assets.
- Native is authoritative per capability. If a supported native call is present, valid, invalid, duplicated, or incomplete, the matching marker fallback is blocked for that capability in the same turn. A turn with an unknown native tool is fail-closed for all marker side effects and records a bounded diagnostic. Marker fallback remains available when no native call for that capability is present.
- Idempotency uses the existing SQLite rows and JSON payloads: scene facts carry the call key in their event payload, media jobs/messages carry it in their existing payload fields, and pending events continue to use their existing `(persona_id, dedupe_key, not_before)` constraint. This task does not rename or replace existing tables and does not add a second persistence store.
- One tool-result continuation is allowed. Tool-call JSON, tool errors, and reasoning content never become visible text. A continuation failure keeps already committed effects, persists a capability-specific bounded fallback sentence, and finishes with the existing `done` envelope rather than rolling back durable facts.

## Acceptance Criteria

- [ ] 一个 registry/dispatcher 统一处理 `scene_event`、`media_event` 和 `pending_event`；不存在重复的 capability dispatcher。
- [ ] mock 分片和真实 MTPLX fixture 能按 `index` 完整累加 id/name/arguments，并在完整调用后严格校验；reasoning/tool JSON 不进入可见 token。
- [ ] native media/pending calls 能在 `/api/companion/chat` 中完成落库、入队和一次 tool-result continuation；pending native 与现有 marker 使用同一 durable contract。
- [ ] native 调用按 provider index 顺序执行；重复/重放通过现有 SQLite rows 和 payload provenance 幂等，不创建重复 scene event、message 或 job。
- [ ] native 优先矩阵生效：同能力 native 成功、失败、重复或不完整时均阻止 marker fallback；无 native call 时旧 marker 仍按严格 schema 处理。
- [ ] malformed arguments、未知 tool、provider error、缺失 `[DONE]`、不完整 chunk 和浏览器断线都有测试，失败不会泄露内部 JSON 或产生未授权副作用。
- [ ] SSE 外层继续使用 `token`、`done`、`error`，`done.messages` 保持权威且 `done.message` 兼容别名继续可用。
- [ ] `npm test`、`node --check server.js`、临时 `DATA_DIR` API/SSE smoke 和兼容性扫描全部通过。

## Dependencies And Out Of Scope

- 依赖父任务 MTPLX research；依赖 shared-scene task 保持 `scene_event` 领域语义；依赖 backend child 提供 LLM/storage/queue ports。
- 不改变浏览器 SSE 外壳、不删除 marker fallback、不迁移数据库表和 API 前缀。
