# Agent prompt composition and timing review

## Scope

Reviewed the current candidate for `docs/llm-agent-prompts.md` and the link from `docs/llm-prompt-composition.md`.

## Independent findings and fixes

- Native Cognition fixed rules were initially described as cumulative across all shared variants. Corrected the document to distinguish the generic `NativeCognitionInstruction` path from the separate Owner life-request and NPC opportunity operation rules. Source: `model_tasks.go:615`, `owner_life_requests.go:394-408`, `world_opportunity_cognition.go:22-26`.
- Corrected the World Event FormalAgent registered name to `world-event-generation`. Source: `formal_agents.go:79`.
- Corrected Diary Expression composition: it uses the `conversationReplyModelRole` and `FactOnlyReply` surface, while its system message is the running-protocol heading, diary-only operation rule, and dynamic persona. It does not add chat Writer v4-v8 protocol or the generic runtime protocol. Source: `diary_runtime.go:261`, `provider_prompt_composer.go:234-239`.

## Final verification

- Compared catalog IDs with the registry: all 30 appear once.
- Local Markdown links resolve; no trailing whitespace remains.
- Reviewed the prompt/spec boundaries and the call-site/component entries. Production callers and unconfirmed/unused registered entries are distinguished.
- No runtime prompt or call behavior was changed. No tests were run for this documentation change.

## Result

Pass. The reviewed documentation scope has no unresolved source-reference or semantic findings.
