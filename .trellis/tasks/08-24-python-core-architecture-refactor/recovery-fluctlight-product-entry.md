# R0: Fluctlight Product Entry Recovery

## Status

Implementation recovery slice. The Owner authorized implementation on
2026-08-25 after a browser inspection found product-flow drift in the
clean-start surface. This document refines delivery order only; it does not
change the parent architecture decisions or introduce legacy compatibility.

## Problem

The current browser starts an empty `New conversation` after login, while the
Core cognition contract requires every turn to target an explicit Fluctlight
participant. The UI also presents the internal `Actor` abstraction as the
product directory, leaves instance rows inert, hides the practical creation
entry point, and presents English-only product copy.

This is a P0 product-flow defect, not a visual regression: a fresh Owner can
reach a composer that cannot submit a valid turn.

## Canonical Language

- Product and instance type: `Fluctlight` / `Fluctlight instance`.
- A created instance's name is the primary label in ordinary UI.
- `Actor` is an internal typed identity (`Human | Fluctlight`) for future
  multi-participant conversations. It is not a primary product navigation
  label.
- `Participant` is a membership record for an opened conversation. It is not
  the instance directory.
- This recovery deliberately uses `Fluctlight`, not the retired product name.

## R0 Contract: Direct Conversation Entry

```text
authenticated Owner
  -> load accessible Fluctlight directory
  -> restore locally persisted selected Fluctlight ID, otherwise select first
  -> Core atomically gets or creates the Owner <-> Fluctlight direct conversation
  -> load the bounded newest message page
  -> send only with the selected Fluctlight as an explicit conversation participant
```

### Invariants

1. There is at most one active direct conversation per `(owner_actor_id,
   fluctlight_actor_id)` pair. The database owns the mapping; browser storage
   only remembers the selected Fluctlight ID.
2. A request with no Fluctlight participant is rejected. A turn with no target,
   or with a target outside the conversation membership, is rejected before a
   user message is persisted.
3. The BFF never derives the target. It forwards the selected target to Core;
   Core checks authorization and conversation membership.
4. With no accessible Fluctlights, no conversation is created and the composer
   is disabled. The primary action is `Create Fluctlight`.
5. Group-chat UI remains out of scope. The Actor/Participant domain model stays
   intact so it can support it later.

## Delivery Sequence

### R0-A: Directory, selection, direct conversation

- Core direct-conversation query/command, Core OpenAPI and generated client.
- BFF query and generated browser client.
- Pinia bootstrap, persisted selection, stale-ID fallback, bounded history,
  disabled no-selection composer.
- Product-facing Fluctlight directory with selectable rows, a direct creation
  entry point, and Chinese UI copy using the `Fluctlight` name.

### R0-B: Creation lifecycle

- `Blank slate`: minimal identity fields -> activate -> select -> direct chat.
- `From description`: initialization role analysis -> editable preview ->
  explicit activate -> select -> direct chat.
- Both paths create a complete Fluctlight foundation and initial inner state in
  one domain transaction. The old persona wizard supplies behavioral evidence
  only; no old routes/components/DTOs are restored.

### R0-C: Configuration repair

- Separate LLM/Embedding endpoints and role bindings from Media Providers.
- One endpoint may serve many model roles. Each role binds endpoint, model,
  budget, timeout and a matching successful preflight.
- ComfyUI/h3 are media settings, never model roles. `media_prompt` is an LLM
  role and is independent from ComfyUI.
- Endpoint mutation invalidates dependent roles; structured-role preflight is
  schema-specific; settings become typed and auditable; deployable Compose
  has no repository-known secret fallbacks.

### R0-D: Product closures

- Instance detail, cursor history, Moments data flow, diagnostics correlation
  and the remaining capability-inventory closures continue under their owning
  modules. Placeholders cannot be presented as delivered features.

## Explicitly Forbidden

- Reintroducing `/api/companion/*`, old persona naming, old DTO adapters,
  old SSE aliases, old storage/job compatibility, or old Node/SQLite runtime.
- Auto-creating an empty conversation on login, refresh, or first render.
- Presenting a global Fluctlight list as `Conversation participants`.
- Allowing an undefined or non-member Fluctlight turn target.
- Mixing ComfyUI URL/workflow inputs into an LLM endpoint/role form.

## Validation Ownership

R0 uses only focused implementation checks while construction continues.
The final cross-product Compose/browser/capability/security/backup and legacy
deletion acceptance remains T12-owned, per D037/D038. R0 does not create a
partial public cutover.

## R0-A Implementation Record (2026-08-25)

Implemented:

- `fluctlight_direct_conversations` is a durable composite Owner/Fluctlight
  mapping with one mapped Conversation. `ConversationService.get_or_create_direct`
  serializes first opens with a PostgreSQL transaction advisory lock, then
  returns the mapped bounded page on every later open.
- New Core and BFF queries:
  `GET /internal/fluctlights/{fluctlight_id}/conversation` and
  `GET /api/fluctlights/{fluctlightId}/conversation`.
- Both internal creation and browser turn transport reject an omitted target;
  Core additionally verifies that the target is a conversation member before
  persisting a user message.
- Generated Core and browser client artifacts include the new query and make
  the browser turn target mandatory.
- Pinia owns the accessible Fluctlight directory and selected ID. On bootstrap
  it restores a still-accessible local ID, otherwise chooses the first current
  item. With no item it clears the active conversation and disables composing.
- Vue presents Chinese product copy, `Fluctlight 实例` navigation, selectable
  rows, the current instance in the chat header, and a practical creation
  entry. Provider and media forms are visually and submission-wise separated.

Migration: `0013_direct_conversation`. The initial longer revision identifier
exceeded this deployment's historical `alembic_version.version_num VARCHAR(32)`
column and rolled back cleanly; the final identifier is intentionally shorter.

Focused implementation evidence:

```text
Core ruff: passed
Core conversation/migration focused pytest: 14 passed
Core OpenAPI artifact parity: passed
Core, browser client, BFF, web typecheck: passed
Web boundary tests: 3 passed
Web production build: passed
BFF focused tests: 14 passed
Compose rebuild/migration: passed; Core healthy
Browser: authenticated bootstrap -> selected Fluctlight -> direct conversation
         -> reload recovery; composer enabled; no console errors
```

No whole-system T12 regression, backup/restore, legacy-deletion, or long-run
resource validation was run here. Those remain owned by T12.

## Integrated Product Record (2026-08-25)

The user requested one integrated verification candidate rather than R0-only
intermediate handoffs. The clean-start candidate now includes:

- Direct Owner-to-selected-Fluctlight conversation, durable selection recovery,
  bounded history and an explicit older-history cursor action.
- Product-facing Fluctlight directory, selected-instance identity detail, and
  Chinese user-visible navigation/copy. `Actor` remains internal only.
- Two activation-only creation paths: minimal `blank_slate`, and description
  analysis through the explicit `initialization` role followed by an editable,
  non-persistent preview and explicit `llm_defined` activation. No description
  analysis failure can create a default or heuristic Fluctlight.
- The real Moments read path for the selected Fluctlight, including an honest
  empty state when its authoritative feed has no entries.
- Built-in Diagnostics, redacted browser-visible records, and an LLM/Embedding
  versus Media Provider settings split. Settings lists every persisted model
  role binding and endpoint validation state.
- Endpoint replacement atomically removes its dependent role bindings. Runtime
  resolution now requires `capability_status=available`; any old or changed
  endpoint must be preflighted and rebound deliberately.

Focused verification completed before the final Compose rebuild:

```text
Core ruff: passed
Focused Core conversation/create/provider/moments tests: passed
Core/browser/BFF/web typecheck: passed
Core OpenAPI parity: passed
Web tests: 3 passed; production build: passed
BFF tests: 14 passed
Compose migration: successful; Core healthy
Browser: direct-chat recovery, instance detail, both creation modes, Moments,
         Provider binding list and separated media settings rendered without
         console errors
```
