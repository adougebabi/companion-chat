# Implementation Plan

1. Read backend specs and current interview/provider contracts; identify the exact JSON response shape supported by MTPLX.
2. Add the bounded JSON completion/analyzer adapter and unit tests for normal JSON, fenced JSON, empty output, malformed output, unknown fields, timeout, and provider failure.
3. Add strict application normalization and preview/session creation. Remove implicit repository analyzer fallback and wire the default runtime analyzer.
4. Add route/runtime integration tests proving provider invocation, ready `interviewId`, no raw description persistence, no-row-on-failure, and activation compatibility.
5. Run focused tests, then the full project quality gate before starting the sibling media task.

## Validation

- `node --test test/interview-analyzer.test.mjs test/llm-provider.test.mjs`
- relevant modular companion API/runtime tests
- `npm test`
- `npm run typecheck`
- `npm run build`
- `git diff --check`
