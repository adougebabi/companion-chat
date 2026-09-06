# Technical Design

## 1. Decisions

### 1.1 Actor is the conversation semantic primitive

Human and Fluctlight are both `Actor` references. Provider transport roles remain
`system`, `user`, and `assistant`, but they do not identify the domain sender.
The server-authoritative `author_actor_id` is the source of truth for every
message. Provider-facing messages carry a stable per-invocation actor alias and
an Actor projection.

The projection contains, at minimum:

```json
{
  "ref": "actor_a",
  "type": "human|fluctlight",
  "role": "owner|romantic_partner|peer|collaborator|unknown"
}
```

Display names are optional presentation data; they are not used as identity.

### 1.2 Relationship is first-person and directed

The first delivery keeps the existing ownership shape:

```text
subject = current Fluctlight (self actor)
target  = one Human or Fluctlight Actor
```

It answers “who is this Actor to me?” and stores role/label, relationship
state, summary, evidence, and revision. A relationship such as “Actor B is my
romantic partner” is therefore a relationship of `actor_a` to `actor_b`, not a
third-party instruction to help two other Actors.

Third-party social mediation and arbitrary `Actor A -> Actor B` graph facts are
out of scope for this delivery and remain a future group/social-graph layer.

### 1.3 Relationship snapshot is system context

For each Provider invocation, Runtime builds a frozen system block containing:

- `self_actor`;
- the current speaker / addressed participant Actor projection;
- the directed relationship snapshot from self to that Actor;
- matching relationship-scoped goals and intentions;
- relationship/context revision identifiers.

The block is regenerated when the conversation participant, speaker, or
relationship revision changes. It is stable across assessment, tool decision,
freeze, and realization for one turn. The current message remains a transport
`user` message whose content identifies its Actor sender.

`composeProviderMessages` must render this as a named system section rather
than flattening it into an unstructured operation-rule bullet.

### 1.4 Cognition and Reflection have separate responsibilities

Conversation cognition consumes the relationship snapshot to choose an
immediate response or action. It never mutates long-lived Relationship, Goal,
or Intention projections.

Reflection consumes the evidence window and may autonomously apply:

- relationship role/state revisions;
- relationship-scoped goal create/update/complete/pause candidates;
- intention candidates linked to those goals.

There is no Owner approval step for these semantic evolutions. Runtime still
owns evidence-window validation, optimistic concurrency, idempotency, actor
authorization, and external side-effect execution gates.

### 1.5 Relationship origin and lifecycle

Relationship values are layered rather than created in one place only:

1. Authorization metadata establishes non-negotiable system facts such as which
   Human Actor created/owns the Fluctlight. This is not a Relationship row and
   does not make the Human a special semantic participant.
2. Optional `initialization` seeds come from the creation/activation input. They
   can declare that a target Actor is a partner, collaborator, family member,
   or another supported role, plus an initial relationship goal. They are
   starting facts, not immutable Core Persona.
3. `reflection` revisions are the Fluctlight's evidence-backed interpretation
   of later interaction. Reflection can discover a previously `unknown`
   relationship, deepen or cool it, change its role, and create/adjust its
   relationship goal without an Owner approval step.

Every Relationship snapshot and revision carries its source/provenance. A
missing seed means the relationship between any two Actors is `unknown`, not an
invented role. Authorization facts remain available in a separate context and
remain protected even when a social relationship evolves.

### 1.6 Relationship lookup capability

Register a read-only `relationship.lookup` capability in the native capability
registry. It reads the current Fluctlight-to-target relationship for a target
Actor that the current conversation/context is allowed to reference. The
current speaker relationship is always injected automatically; the capability
is for additional, explicit lookups and must not return an unrestricted global
relationship dump.

The capability is MCP-compatible at the contract boundary but remains a
Runtime-owned read model in this task. It has no side effects and no revision
write path.

## 2. Data contracts

### 2.1 Relationship

Extend the existing relationship record with a typed role projection, keeping
the existing metrics/trend/revision fields compatible:

```json
{
  "target_actor_id": "actor_b",
  "role": {
    "primary": "romantic_partner",
    "secondary": [],
    "label": "伴侣"
  },
  "metrics": {"trust": 0.8, "intimacy": 0.7},
  "trend": "improving",
  "summary": "……",
  "provenance": {
    "source": "initialization|reflection|manual",
    "evidence_refs": ["inbox_…"]
  },
  "revision": 4,
  "evidence_refs": ["inbox_…"]
}
```

The role must remain a separate semantic field from metrics. The migration may
use a JSONB role envelope for compatibility, but the Go validation contract
must define canonical role codes and reject malformed values.

### 2.2 Relationship-scoped Goal

Extend goal persistence and Provider projection with:

```json
{
  "scope": "relationship",
  "target_actor_id": "actor_b",
  "description": "维持与 actor_b 的感情并持续升温"
}
```

Generic goals keep `scope=general` and a null target. A relationship goal is
always first-person from the current Fluctlight. Its linked Intention retains
the goal ID and may include a relationship trigger/target snapshot.

Creation/activation may supply optional relationship-goal seeds. If none are
supplied, Reflection is the only path that can create a relationship goal from
subsequent evidence.

### 2.3 Reflection candidates

Reflection response/apply contracts add typed goal and intention candidate
collections. Candidates include an operation, target goal when updating, the
optional target Actor, evidence refs, and idempotency key. Candidate application
is part of the existing reflection transaction and state-revision CAS.

### 2.4 Relationship governance read/edit contract

The detail read model enriches each relationship target with server-derived
Actor metadata:

```json
{
  "target_actor_id": "actor_b",
  "target_actor_type": "human|fluctlight",
  "is_current_user": true,
  "role": {"primary": "romantic_partner", "secondary": [], "label": "伴侣"},
  "metrics": {},
  "trend": "improving",
  "summary": "…",
  "provenance": {"source": "reflection", "evidence_refs": []},
  "revision": 4
}
```

`is_current_user` is computed by Core from the authenticated Human Actor ID;
the browser cannot submit or override it. It is a UI identity marker, not a
relationship role.

Add an authenticated relationship edit command for one existing target pair.
The request may update role/labels, metrics, trend, summary and emotional
association, and must carry `expected_revision`, `evidence_refs`, and an audit
reason. `target_actor_id` is path identity and cannot be changed. The command
creates a new relationship revision and governance row rather than overwriting
history. Goal/intention editing remains a separate contract.

## 3. End-to-end flow

1. The authenticated session or internal workflow resolves the server-side
   sender Actor.
2. Creation/activation persists authorization metadata and any optional
   initialization relationship seeds; it does not materialize a social
   Relationship solely because an Actor is the creator/Owner. Context projection then loads
   the current Fluctlight, current speaker, bounded
   messages, relationship row/revision, and matching relationship goals and
   intentions.
3. Provider composition renders the relationship snapshot in the leading
   system message and renders the incoming message as `actor_a` content in the
   transport user message.
4. Cognition proposes an immediate decision. No relationship/goal mutation is
   allowed in this path.
5. Runtime freezes and executes the decision through existing capability and
   authorization boundaries.
6. The processed evidence window schedules Reflection.
7. Reflection compares relationship evidence against the current revision and
   proposes relationship/goal/intention changes, including discovery from
   `unknown` and correction of initialization seeds.
8. Runtime validates evidence, target Actor, current revisions and idempotency,
   then applies the semantic revisions atomically and advances the reflection
   watermark.
9. The next turn receives a fresh system snapshot with the new relationship and
   matching goals/intents.

10. The detail endpoint returns Actor type and the server-derived current-user
    marker. Governance UI renders the relationship editor and submits a CAS
    edit; a successful edit is visible as a new revision and audit record.

## 4. Compatibility and rollout

- Keep persisted `conversation_messages.kind` and Provider transport roles
  unchanged; add Actor-aware projections around them.
- Existing relationships without a role normalize to `unknown`.
- Existing goals without scope normalize to `general` with no target.
- Existing reflection proposals remain valid; missing new optional candidate
  collections normalize to empty arrays.
- Relationship revision and goal/intention changes remain append/audit-friendly;
  no destructive migration or history rewrite is required.

## 5. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Relationship stale within a turn | Freeze revision in the system snapshot and regenerate after revision changes |
| Model confuses transport `user` with Human Actor | Include explicit sender Actor projection and tests for mixed Human/Fluctlight history |
| One relationship goal accidentally applies to another Actor | Require `scope` + `target_actor_id` matching before action selection |
| Reflection mutates on one weak signal | Preserve evidence-window and provenance requirements; do not add cognition writes |
| Lookup leaks unrelated relationships | Resolve only authorized target Actors and return one direct relationship projection |
| Composer flattens relationship system data into prose | Add a dedicated named system section and composer tests |
