# LLM Prompt Composition Contract

## 1. Scope / Trigger

Every formal Agent, its typed subtasks, Writer, media/vision task, correction
request and Kev decision must obey this contract when constructing model input.
This is the current contract; older task records are historical evidence.

## 2. Signatures

```go
prompt.RenderJSON(value any) string
PromptComposer.ComposeAssembly(PromptAssemblyInput) (PromptAssemblyResult, error)
PromptComposer.ComposeTaskMessages(role string, messages []map[string]any) []map[string]any
formalPromptMessages(id FormalAgentID, messages []map[string]any) []map[string]any
goalEvaluationPromptProjection(snapshot, projection) ContextProjection
shareGoalEvidencePayloads(sources []any) map[string]any
```

The authoritative entry inventory is [LLM prompt composition](../../../docs/llm-prompt-composition.md).
Update that inventory and `TestEveryFormalAgentSharesLosslessProtocolComposition`
when adding an Agent; shared Agent IDs can own several explicit task variants.

## 3. Contracts

### No application token limits

**禁止为了优化提示词而新增、恢复、调高、调低或换名设置 prompt/context
输入上限、response/output 输出上限、分段 token 配额、预算拒绝或按 token
估算截断。不得发送 `max_tokens` / `max_completion_tokens`。**

Keep `prompt-unlimited.v1`. Token estimates/usage are diagnostics only. Preserve
large valid input, complete native Tool results and unsummarized whole turns.
Legacy configuration/database token fields are compatibility storage, not active
settings. Do not introduce a fallback ceiling, reserve or section target.
Upstream length/context failures remain real Provider failures. Query scope,
pagination, domain argument validation, timeouts, cancellation and iteration
guards are separate existing contracts, never excuses for a prompt token cap.

### Composition and format

- One leading system protocol: shared authority rules once, task rules once,
  trusted Working Persona only when needed. Analysis tasks do not add an empty
  persona/roleplay section. Persona meaning is unchanged.
- Structured model data uses deterministic JSON everywhere. Stable task facts
  may precede volatile Runtime Context for caching; a fact has one owner and
  must not be repeated across stable/runtime/current input.
- Private chat retains original user/assistant roles, content, order and sender
  time provenance. WakeUp and Moment history is explicitly historical data.
  Default/background surfaces do not automatically acquire a chat transcript.
- Only explicitly tagged `promptTaskData` may be parsed as trusted task data;
  string messages (especially initialization Owner descriptions) never promote
  a supplied core_persona into system.
- Current user input appears once. A user's JSON is still user text; transport
  cannot parse, rewrite or change its format based on content shape.
- Images, native ToolCall/ToolResult and response schemas keep native transport
  structures. Only explicitly built business views enter the JSON renderer.
  Kev keeps its external `{state,questions}` envelope with JSON semantic state.
- Renderer functions do not query databases, judge relevance, infer authority,
  mutate sources or select different syntax based on table shape/token count.

### Model views, metadata and evidence

- Use task-owned field allowlists. DB row IDs, revisions, digests, request/run/
  workflow identity, leases and scheduling machinery belong to Core/diagnostics.
  Actual event times, validity windows, deadlines, subject, uncertainty, false,
  zero and unknown have domain meaning and must survive.
- Necessary opaque refs bind through the frozen authorized Core context.
  Explicit exceptions: persona profile/rule selectors; exact Owner-requested
  Tool target/asset IDs; Studio allowed definition IDs/versions; Planner query/
  commit CAS and target selectors. Each exception has a consumer and tests.
- Never recursively strip arbitrary `*_id`, `*_ref`, `version` or rewrite
  natural-language strings. A user document, Actor value_json, tool schema's
  properties/required/default, and an external domain-query result are data.
  Entity discovery and aliasing traverse only declared Core receipt nodes.
- GoalEvaluation owns selected definitions in stable/current, with unrelated
  goal background separate. Runtime must not re-inject the selected goals.
- Historical `capability.catalog` evidence carries discovered names, actual
  availability/loading/admission results, not a second copy of executable
  tool documentation/schema. The live native catalog retains its real schema.
- Exact repeated evidence `data` may use `payload_ref` into `evidence_payloads`.
  Every source keeps its own eN/time/subject/valid/can_support_success/goal_refs;
  existing hydration resolves every original event independently. A changed
  body is a different payload. No fuzzy merges or event-count reduction.
- Schedule items appear once. `current_item_index` and `upcoming_item_indexes`
  are zero-based into `items`; one start/end pair plus timezone replaces purely
  derived display variants. The complete authorized plan remains in Core.

### Memory and history

Retain memory/summary-first continuity, not a fixed ten-message window. Candidate
queries retain their existing scope/count rules. Only a selected, valid summary
or memory with proven source coverage can replace fully covered whole turns;
partial coverage keeps the entire turn. The latest protected interaction groups,
actual current-turn exclusion and real triggering-user backfill remain.

Same source does not mean same fact: deduplication requires the same complete
source set and exact semantic payload. Different claims, subjects, time bounds,
certainty and contradictions survive. Shared provenance can be represented once
with inheritance only when every original field and order can be restored.
Memory/summary/history do not override authoritative current state.

Old durable Writer protocol/version/hash and media continuation fences remain;
changing the presentation does not relabel old draft contracts or replay effects.

## 4. Validation & Error Matrix

| Input | Required behavior |
| --- | --- |
| Large valid current input/Tool result and tiny legacy token settings | Full request, no output cap keys |
| Same evidence body with distinct event refs/times/validity | Shared body, distinct events and source hydration |
| Same source with two different claims | Keep both |
| Half a turn covered by summary | Keep the whole turn |
| JSON user text, schema `item_ids`, domain `version` | Preserve original data, do not discover fake entities |
| Unknown/foreign/stale executable ref | Existing authority rejection, no guessed replacement |
| Catalog receipt in historical evidence | Semantic discovery result; actual live schema unchanged |
| Analysis/NPC/Studio task | Only its declared input, no unrelated persona or chat |

## 5. Good / Base / Bad Cases

Good: source e1 and e2 share payload:1 while each keeps its timestamp and validity.
Base: a single source stays inline. Bad: discard e2 because the text matches e1,
restore a 12K prompt threshold, or recursively delete `properties.item_ids`.

## 6. Tests Required

`prompt_composition_contract_test.go` covers the complete Agent registry,
lossless transport (including large integers/images/native results), task view
envelopes, evidence hydration/schema opacity, single Goal ownership, schedule
factoring and shared-source distinct claims. Maintain whole-turn, stable-prefix,
persona and private Writer regressions. Physical no-cap tests cover ordinary
structured/stream, Writer and the third Tool continuation reaching request four.

Size comparisons measure the actual composed messages on identical/specified
data. Reconstructed user captures are opt-in local inputs, never checked-in
private fixtures or proof of live model quality, tokenizer usage or latency.

## 7. Wrong vs Correct

Wrong: shrink `max_input_tokens`, drop every `_id`, or convert any JSON-looking
message into a compact table.

Correct: a typed task projects authorized semantic fields, encodes them once
with `RenderJSON`, preserves native/current text, and retains all necessary
evidence regardless of estimated size.
