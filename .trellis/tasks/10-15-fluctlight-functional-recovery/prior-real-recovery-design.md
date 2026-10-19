# Technical design

## Authority and boundaries
The active App mounts the v3 native module tree. Repair that tree and its BrowserClient adapter, preserving the confirmed shell/tokens, rather than editing unmounted Vue views. Reuse existing Vue components/services through explicit integration where useful. Core remains domain authority; browser transport validates/maps only. Preserve Qwen/Cydonia and Agent/Tool loops.

## Public recovery
Replace fixture-ID/status assumptions and map actual Go detail fields. Hydration preserves session identity and independent success/error/unavailable states, uses request epochs, and avoids treating all HTTP failures as empty data. Global identity/list reads are required; independent modules fail locally. Successful writes await service receipts and authoritative readback before feedback.

## About you
Natural entry: people/owner, labelled 关于你. Target is the effective human sender (authenticated session Actor until another participant is selected). Reuse existing ten-field draft codec and Actor facts/context service. Owner CAS is current_facts_revision; other Actor CAS is context_version. Keep draft/idempotency on conflict or transport failure; read source/history from Actor context. Cache by fluctlight+actor. No new profile table; actor_facts remain consumed by cognition/reply projections and invalidate existing caches.

## World/NPC
Keep Studio drafts separate from published immutable definition snapshots and runtime instances. Add additive world/place/NPC persistence scoped to a Fluctlight runtime, independent NPC Actor identity, version/reason/source and safe archive. Reuse existing Actor relationship/life event/memory contracts for controlled encounter receipts. Creating/applying NPC never creates acquaintance; world reads never change effective location. Keep legacy text location authoritative until explicit catalog association; no same-name historical backfill. Apply same template to two runtimes with distinct IDs.

## Compatibility and rollout
Use new linear migrations after 0063; verify fresh and prior-head isolated databases. Do not mount old user volumes or alter history. Existing unsupported Studio types retain truthful record-only semantics until implemented; completed types execute actual apply and expose resulting revision. API additions synchronize route inventory, OpenAPI/client generators and tests.

## Validation
Dedicated Compose project fluctlight-recovery-1015/new volumes with loopback dependency ports; host Go/Vite for edit/restart. Secrets only in private /tmp env. Test DB helpers create isolated databases. Browser tests use actual Core; Mock contract tests reported separately. Local model discovery is availability evidence only; real inference must be executed to claim passage.
