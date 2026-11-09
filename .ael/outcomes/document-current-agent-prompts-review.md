# Agent prompt catalog semantic review

## Scope

Reviewed the current Agent prompt catalog and its prompt-composition index entry.

## Findings and disposition

- Compared the headings in `docs/llm-agent-prompts.md` with the IDs in `formal_agents.go`: all 30 current registry entries appear exactly once.
- Checked local Markdown targets: all referenced source files and linked prompt composition/specification documents exist. An independent review also checked source anchors.
- An independent model review found the first draft described only Writer tail-corruption repair. The document was updated to also describe high-confidence narration/protocol-leak repair, its private task-data boundary, no-tool/no-media behavior, and failure blocking. Those details were checked against `reply_style.go` and `reply_provider.go`.
- The new document distinguishes exact short source excerpts from semantic summaries, explains that final requests include dynamic context and native multimodal/tool data, and does not treat registration as proof of production use.
- The prompt construction contract is preserved: no token limits or estimate-based truncation are introduced. `docs/llm-prompt-composition.md` links to the catalog and reports the registry count as 30.

## Verification

Static coverage comparison, local-link target check, and whitespace/diff checks passed. No tests were run for this documentation-only change.

## Result

Pass. No unresolved semantic or source-reference findings remain in the reviewed documentation scope.
