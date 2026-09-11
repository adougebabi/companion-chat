# Applicable Project Contracts

The following existing specs are authoritative for implementation:

- `.trellis/spec/backend/fluctlight-cognitive-runtime.md`: LLM owns semantic meaning; Core owns validation, policy, state, idempotency and action execution. No keyword/regex semantic fallback. Capability slots are registry-based; realization cannot add semantic effects.
- `.trellis/spec/backend/structured-turn-contract.md`: native and sidecar tool calls normalize once; native tools are the sole complete capability schema; no second chat commit boundary; ToolResult and protocol metadata stay out of visible output.
- `.trellis/spec/backend/fluctlight-provider-contract.md`: one leading system message; explicit role/provider capabilities; no implicit model fallback; record role/model/prompt/schema/correlation/timing/token provenance.
- `.trellis/spec/backend/fluctlight-life-world-contract.md`: Event > accepted Schedule > explicit pending context authority; schedule and scene rules are domain-owned and must not become prompt-only heuristics.
- `.trellis/spec/backend/fluctlight-memory-contract.md`: memory owner/visibility/evidence filters precede ranking; authoritative memory precedes async embedding.
- `.trellis/spec/backend/fluctlight-media-contract.md` and `media-prompt-contract.md`: media intent/concept is frozen before worker execution; media prompt rendering cannot invent semantics; provider/storage calls are outside business transactions.
- `.trellis/spec/backend/fluctlight-autonomy-contract.md`: installed/preflighted capability slots are additive; external actions require frozen policy-authorized decisions and stable IDs.
- `.trellis/spec/backend/fluctlight-persistence-contract.md`: one application transaction owner; no external I/O inside PostgreSQL transaction; stable outbox/intent and replay IDs.
- `.trellis/spec/backend/fluctlight-workflow-contract.md`: exactly one durable workflow runtime; restart/replay uses stable domain intent IDs; old history must remain replayable or be drained before incompatible deployment.
- `.trellis/spec/backend/fluctlight-event-contract.md`: PostgreSQL is durable event authority; Redis is at-least-once transport only.
- `.trellis/spec/backend/debug-observability.md` and `logging-guidelines.md`: diagnostics are bounded/redacted; do not log credentials, hidden reasoning, full sensitive context, or unbounded payloads.
- `.trellis/spec/backend/directory-structure.md` and `quality-guidelines.md`: Go Core owns domain and provider behavior; keep changes in `apps/core-go/internal/core`; run Go test/race/vet/build/gofmt gates.
