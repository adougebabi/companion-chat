# Research: Thinking Output Audit

- Query: Audit why hidden thinking/reasoning content can appear in deployed chat output, tracing MTPLX/OpenAI-compatible stream parsing, reasoning/reasoning_content/content/tool-call cleanup, SSE token presentation, and frontend rendering boundaries; identify root cause and regression tests.
- Scope: mixed
- Date: 2026-08-23

## Findings

### End-to-end data flow

The production path is:

`MTPLX HTTP SSE -> createMtplxCompletionPort() -> consumeMtplxStream() -> chat-turn-flow presentation -> chat-turn-sse-adapter -> runtime sendSse() -> browser readSse() -> conversations.appendTransientToken() -> MessageBubble`.

- [`server/runtime/runtime.js:390-395`](../../../../server/runtime/runtime.js) constructs the raw MTPLX completion port and wraps it with the chat streaming adapter. The runtime therefore uses the infrastructure parser in the live chat path.
- [`server/infrastructure/llm-provider.js:221-298`](../../../../server/infrastructure/llm-provider.js) owns upstream SSE framing, JSON decoding, visible-text accumulation, and native tool-call accumulation.
- [`server/application/chat-turn-flow.js:437-491`](../../../../server/application/chat-turn-flow.js) normalizes the completion and creates `sseToken()` presentation events from `completion.tokens` unless marker/capability continuation rules suppress them.
- [`server/http/chat-turn-sse-adapter.js:136-151`](../../../../server/http/chat-turn-sse-adapter.js) selects only presentation events with `type === 'token'`; [`server/http/chat-turn-sse-adapter.js:240-285`](../../../../server/http/chat-turn-sse-adapter.js) emits those events and also forwards live `onToken` callbacks. This boundary performs no reasoning or markup filtering.
- [`server/runtime/runtime.js:441-446`](../../../../server/runtime/runtime.js) serializes normalized SSE events as `data: ${JSON.stringify(event)}\\n\\n`; it does not inspect token contents.
- [`web/src/composables/useChatStream.ts:42-72`](../../../../web/src/composables/useChatStream.ts) parses only the outer SSE envelope, and [`web/src/composables/useChatStream.ts:149-183`](../../../../web/src/composables/useChatStream.ts) appends every `token` to the pending assistant message.
- [`web/src/stores/conversations.ts:141-145`](../../../../web/src/stores/conversations.ts) concatenates token text without classification. [`web/src/components/chat/MessageBubble.vue:8-30`](../../../../web/src/components/chat/MessageBubble.vue) renders the resulting message text directly. The browser has no reliable way to distinguish an answer token from a leaked reasoning token after the backend emits it.

### Concrete parser behavior

`consumeMtplxStream()` reads only `choice.delta.content` as visible text at [`server/infrastructure/llm-provider.js:240-245`](../../../../server/infrastructure/llm-provider.js). It separately consumes `delta.tool_calls` and `choice.message.tool_calls` at [`server/infrastructure/llm-provider.js:245-252`](../../../../server/infrastructure/llm-provider.js).

- `delta.reasoning_content` is ignored.
- `delta.reasoning` is ignored.
- `choice.message.content` in a streaming payload is ignored.
- Tool-call fragments are accumulated and normalized separately; their arguments do not become token events by this parser.
- The only visible-text cleanup is `cleanToolCallArtifacts()` at [`server/infrastructure/llm-provider.js:531-537`](../../../../server/infrastructure/llm-provider.js), which removes pseudo tags matching `<TOOL_CALL>`/`</TOOL_CALL>` and spelling variants. It does not remove `<think>`, `<thinking>`, `<analysis>`, `<reasoning>`, or provider-specific reasoning wrappers.

The non-streaming JSON path has the same boundary shape. `completionFromJson()` extracts only string `message.content` at [`server/infrastructure/llm-provider.js:300-321`](../../../../server/infrastructure/llm-provider.js), parses `message.tool_calls` at [`server/infrastructure/llm-provider.js:322-328`](../../../../server/infrastructure/llm-provider.js), and applies only `cleanToolCallArtifacts()` to the visible string. A separate `message.reasoning`/`message.reasoning_content` field is ignored, but reasoning embedded in `message.content` is preserved.

`sidecarCandidate()` also treats an object-valued `delta.content` as a possible structured sidecar at [`server/infrastructure/llm-provider.js:26-67`](../../../../server/infrastructure/llm-provider.js). This is a separate compatibility concern: an object content payload is not emitted as text, but it can be mistaken for a control sidecar. It does not provide a general reasoning sanitizer.

### Root cause

The primary root cause is an upstream contract mismatch: the implementation assumes hidden reasoning is always delivered in a separate `reasoning_content`/`reasoning` field, while some deployed reasoning models can put thought text in the visible `content` stream, commonly wrapped in `<think>...</think>` (or equivalent tags). Because the parser forwards every string `delta.content` fragment to `onText` after removing only TOOL_CALL tags, the hidden text is immediately turned into a token presentation event and cannot be removed by SSE or the frontend later.

This is consistent with the current code and a direct probe:

- A `delta.reasoning_content: "R"` payload produces `{text: ""}`.
- A `delta.reasoning: "R"` payload produces `{text: ""}`.
- A `delta.content: "<think>R</think>C"` payload produces `{text: "<think>R</think>C"}`.
- A `delta.content: "<TOOL_CALL>"` payload produces `{text: ""}` because that is the only supported cleanup.

Therefore, the likely deployment failure is not an SSE framing or Vue rendering bug. It is the parser accepting reasoning-laden `content` as visible text. A custom/injected LLM port that returns raw `reasoning` in `text`/`tokens` would be an additional variant: [`server/application/chat-turn-flow.js:95-114`](../../../../server/application/chat-turn-flow.js) trusts normalized completion text/tokens and has no reasoning-specific cleanup, so the same leak would flow to presentation.

### Tool-call and presentation boundaries

- Native tool-call JSON is accumulated by index/id and normalized to structured calls at [`server/infrastructure/llm-provider.js:134-213`](../../../../server/infrastructure/llm-provider.js). It is not emitted as a token by `consumeMtplxStream()`.
- The flow can suppress visible tokens when a native capability call requires a continuation at [`server/application/chat-turn-flow.js:476-491`](../../../../server/application/chat-turn-flow.js). This protects tool-only turns, but it does not sanitize visible text on turns that contain both content and tool calls.
- The continuation path cleans only TOOL_CALL tags at [`server/application/chat-turn-flow.js:559-618`](../../../../server/application/chat-turn-flow.js). A reasoning wrapper in continuation `next.text` would still be accepted as visible text.
- [`server/contracts/index.js:837-850`](../../../../server/contracts/index.js) bounds SSE token strings and validates their shape; it does not enforce a visible/reasoning distinction. [`server/http/chat-turn-sse-adapter.js:117-134`](../../../../server/http/chat-turn-sse-adapter.js) also sends `done.messages` based on the flow result, so leaked text can persist in the final assistant message even if a client ignores live tokens.
- [`web/src/api/contracts.ts:115-138`](../../../../web/src/api/contracts.ts) normalizes message text as a string and preserves other object keys; [`web/src/components/chat/MessageBubble.vue:20-31`](../../../../web/src/components/chat/MessageBubble.vue) renders it as ordinary visible copy. Frontend filtering would be too late and would need to duplicate provider-specific parsing rules.

### Existing tests and coverage gap

The focused provider/SSE/flow suite passes: `node --test test/llm-provider.test.mjs test/chat-turn-sse-adapter.test.mjs test/chat-turn-flow.test.mjs test/companion-api-modular-provider-debug.test.mjs` => 30 passed, 0 failed.

Existing coverage includes:

- [`test/companion-api-modular-provider-debug.test.mjs:292-326`](../../../../test/companion-api-modular-provider-debug.test.mjs): `reasoning_content` is present as a separate delta and is absent from `completion.text`; native tool-call fragments are preserved.
- [`test/llm-provider.test.mjs:44-61`](../../../../test/llm-provider.test.mjs): TOOL_CALL pseudo tags are removed from streaming and JSON completion content.
- [`test/chat-turn-flow.test.mjs`](../../../../test/chat-turn-flow.test.mjs): tool-only capability turns and TOOL_CALL artifacts do not become visible text.
- [`test/chat-turn-sse-adapter.test.mjs:40-69`](../../../../test/chat-turn-sse-adapter.test.mjs): token presentation ordering and the authoritative `done` alias are checked.

Missing regression cases are exactly the variants implicated by the deployment symptom: `delta.reasoning`, `delta.content` containing `<think>`/`<thinking>`/`<analysis>` blocks, split wrapper tags across chunks, mixed reasoning plus answer fragments, non-streaming `message.content` with wrappers, and a full flow/SSE assertion that no reasoning marker reaches either `token` or `done.messages[].text`.

### Related contracts and historical evidence

- [`.trellis/spec/backend/structured-turn-contract.md:29-30`](../../../../.trellis/spec/backend/structured-turn-contract.md) states that hidden reasoning must never enter user-visible chat or ordinary API DTOs.
- [`.trellis/spec/backend/error-handling.md:15-27`](../../../../.trellis/spec/backend/error-handling.md) defines the upstream MTPLX SSE -> server SSE translation and explicitly requires reasoning content, tool JSON, call IDs, and dedupe keys to stay out of visible tokens.
- [`.trellis/spec/guides/cross-layer-thinking-guide.md:16`](../../../../.trellis/spec/guides/cross-layer-thinking-guide.md) identifies the upstream MTPLX SSE -> server SSE translation as a cross-layer boundary requiring explicit key ownership.
- Archived research [`.trellis/tasks/archive/2026-08/08-20-fluctlight-architecture-performance-modernization/research/mtplx-tool-call-results.md:48`](../../../../.trellis/tasks/archive/2026-08/08-20-fluctlight-architecture-performance-modernization/research/mtplx-tool-call-results.md) records a real MTPLX probe where `reasoning_content` appeared in separate chunks and was intentionally excluded by consuming only `delta.content` and `delta.tool_calls`. That evidence validates the separate-field assumption for that model, but does not cover models that inline reasoning in `content`.
- Commit `8689758` (2026-08-23) added TOOL_CALL-tag cleanup at [`server/infrastructure/llm-provider.js:243`](../../../../server/infrastructure/llm-provider.js) and [`server/infrastructure/llm-provider.js:321`](../../../../server/infrastructure/llm-provider.js), but did not add reasoning-wrapper cleanup. The current deployment symptom can remain after that fix.

### Recommended regression tests

1. Add provider parser fixtures for `delta.reasoning` and `delta.reasoning_content` alongside visible `delta.content`; assert both reasoning fields are absent from `text`, `tokens`, `onText`, `structuredTurn.text`, and `parseErrors` does not treat them as visible content.
2. Add wrapper fixtures for `<think>private</think>answer`, `<thinking>private</thinking>answer`, and `<analysis>private</analysis>answer`; assert only `answer` is forwarded. Include opening/closing tags split across separate SSE chunks, because per-fragment regex cleanup cannot remove a wrapper reliably when a tag boundary spans fragments.
3. Add mixed-stream fixtures where reasoning arrives first, then a native tool call, then visible content; assert tool-call accumulation remains complete, visible text is only the answer, and `onText` never receives reasoning.
4. Add JSON completion fixtures for `message.reasoning`, `message.reasoning_content`, and wrapped `message.content`; assert the same normalized contract as streaming.
5. Add an application/SSE integration fixture that feeds a reasoning-wrapped completion through `createChatTurnFlow()` and `createChatTurnSseAdapter()`; assert no emitted `token` event and no `done.messages[].text` contains reasoning markers or private reasoning text.
6. Add a negative test for ordinary user-visible prose containing a literal `<think>` string if the product intends to preserve user text. This forces the implementation to define whether sanitization is provider-output-only (preferred) versus generic text rewriting.

## Caveats / Not Found

- No live deployment endpoint, model name, or raw production MTPLX payload was available in the repository, so the exact wrapper syntax cannot be proven from local code alone. The `<think>`/`<thinking>`/`<analysis>` cases are the most likely OpenAI-compatible reasoning-model variants and should be confirmed against one captured production response with secrets/redacted.
- The repository's existing real probe evidence only covers separate `reasoning_content`; it does not prove that the deployed model in this report inlines reasoning in `content`.
- No frontend code currently exposes a reasoning channel; after a token is emitted, the browser treats it as ordinary assistant text. The fix should therefore be owned by the provider/application normalization boundary, with SSE/frontend tests serving as leak-detection regressions rather than primary sanitizers.

## Implementation Outcome

The shared `server/contracts/hidden-reasoning.js` filter now removes leading
provider reasoning wrappers during streaming, including attribute-bearing tags
split across chunks, while preserving literal tags embedded in ordinary prose.
The MTPLX parser, application completion normalizer, structured assistant
messages, and SSE token contract all use the visible-text boundary. Regression
coverage includes streaming/JSON wrappers, split attributes, literal prose,
structured message text, and token whitespace preservation.
- The research agent made no code, spec, configuration, or git changes outside this research file.
