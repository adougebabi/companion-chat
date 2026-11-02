# Fluctlight r1 defects D1–D6

## Goal and evidence
Resolve the six defects explicitly authorized in the user request; evidence is /workspace/ft-runs/20261102-r1/REPORT.md and logs/.

## Requirements and acceptance
- D1: persona compilation failures remain recoverable or explicitly failed; chat immediately explains failed preparation; frontend recovery calls the existing persona-preparation/recover API. Regression coverage across domain, dispatch and UI.
- D2: long image/outfit jobs cannot hold the chat serial lane; preserve transactional state consistency and test concurrent lane behavior.
- D3: transient studio timeouts receive bounded retries with backoff before terminal settlement; test retry and exhaustion.
- D4: llm_defined uses the user-entered name verbatim (FT-青岚); test model-proposed alternatives.
- D5: wardrobe lifecycle diagnostics persist valid machine reasons; test actual noop outcome.
- D6: disabled vision has readable configuration guidance; image prompts and negative prompts forbid text; test provider-bound requests.
- Run pnpm install, build, web lint, typecheck, full JS and Go tests; fix failures without weakening assertions.

## Constraints and exclusions
Do not access /workspace/accept or any *.env. No git commit or index writes; .git read-only. No image artifacts. No token input/output limits or prompt truncation. No unrelated feature work. Report each changed old assertion with justification, migration/config needs and remaining limitations in Chinese.
