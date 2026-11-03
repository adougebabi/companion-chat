# Technical Design

## Analyzer Port

Add a small infrastructure adapter dedicated to persona extraction. It accepts `{description, signal, trace, personaId?}` and returns a validated application-owned object. It uses the existing MTPLX provider settings and its OpenAI-compatible `stream({stream:false})` transport, but does not reuse `companion.turn.v1` because persona extraction is a different schema.

The adapter is responsible for:

- bounded request timeout and provider error mapping;
- extracting the assistant content from an OpenAI-compatible JSON response;
- removing optional Markdown JSON fences;
- rejecting empty, non-object, unknown-key, oversized, or malformed output.

The application analyzer is responsible for the persona schema, field allowlist, provenance, preview construction, and session creation. It must not infer missing values with regular expressions. If the model omits a field that the product contract requires, the analyzer returns a bounded validation error; it does not silently invent a rule-derived value.

## Runtime Wiring

The production runtime already resolves the MTPLX provider for chat. Construct the JSON completion port from that provider and pass a `personaAnalyzer` adapter through `createBasicCompanionServices` -> `createCompanionRouteService` -> `createInterviewService`. The analyzer is the only configured operation for the production `analyze` route. If a test or custom composition intentionally omits it, the application returns `501` rather than selecting `repository.analyze()`.

## Session and Preview

On successful analysis, normalize a bounded object with a versioned source (`source: 'llm'`) and create a ready interview session containing only structured answers and inferred field names. The raw description is request-scoped. The existing preview/activation ports remain the persistence owner. User overrides at activation replace generated values and are marked user-confirmed.

## Compatibility

Existing draft/ready interview rows continue through repository `get`, `answer`, `preview`, and `activate`. The old repository analyzer may remain as an explicitly test-only/legacy method for old callers, but it is unreachable from the default production analysis service and must have a regression test proving that boundary.

## Failure Semantics

Provider or shape failure occurs before `createInterview`; the route returns `502` and leaves SQLite unchanged. No raw model response or user description is persisted. `interviewId` is returned only after the ready row is committed.
