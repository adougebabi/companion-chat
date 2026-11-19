# Claim evidence evaluation history

2026-11-19. Scope: the existing cognition claim evaluator, settlement and
hypothesis read model. This is an implementation decision for the dedicated
new-evidence bug, not a change to long-term Memory lifecycle or model prompts.

## Fact identity and compatibility

A repetition label alone cannot establish that two complete statements are the
same fact. Evaluation compares the original label and exact canonical content
before applying the existing evidence-set repetition rule. No similarity,
keyword or inferred semantic merge is introduced.

Existing canonical claim IDs, labels and content remain unchanged on upgrade.
Ordinary new claims retain their supplied repetition label. When that label
already identifies a different complete statement, only the new claim receives
a deterministic Core storage discriminator. Evaluations retain the original
label; hypothesis reads restore it. A discriminator occupied by another complete
identity fails closed. Different statements from one source remain independent.

## Current assessment and historical support

Migration 0118 adds only the evaluation table and indexes. It does not rewrite
canonical claims, change their uniqueness or backfill imagined evaluation runs.
On the first subsequent settlement of a legacy claim, its actual current row is
saved as `legacy_current`; an unknown original evaluation time remains NULL.

Each distinct evaluated input records its original source, evidence, classification,
confidence, disposition, expiry and business time. The application appends these
snapshots and does not update them. Source identity is bound by the caller;
model-supplied source identifiers do not replace the settlement source.

New approved/uncertain evidence can update the current claim using the evaluated
state and evidence snapshot. Old support remains traceable in history, rather
than being blindly unioned into current support. An omitted/repeated evaluation
cannot overwrite an existing active or uncertain claim as rejected. A known older
business-time evaluation is retained without replacing newer current state.
Rejected/expired claims cannot revive from replay or unchanged evidence; genuinely
new valid evidence is evaluated through the original uncertainty policy.
Novelty requires a support reference not already recorded for this complete
fact. A new settlement source with only old support, support rotation or a subset
does not renew or reactivate it. Such evaluations remain traceable as unapplied;
their Core application reason is separate from the model's disposition.

An assessment fingerprint excludes retry time and its derived expiry, preserving
the first accepted expiry on exact replay. It includes source, complete statement,
classification, confidence, evidence and disposition. Evidence order is immaterial
to repetition; the recorded snapshot preserves its original sequence.

## Transaction and authority boundaries

Settlement uses the caller's short transaction. Sorted per-instance/per-label
transaction locks serialize concurrent retries and avoid reversed multi-claim
lock acquisition. Canonical state and evaluation snapshots roll back together.
The normal inbox/source/authority fences and opaque evidence validation remain.
No Provider, network or other external call runs inside this transaction.

Evaluation history is audit data and is not registered as a semantic generation
source. Actual canonical changes continue through the existing context-generation
trigger. History does not itself become Memory or new evidence for another claim.

API/Worker still require explicit migration before readiness. Tests apply it only
to disposable databases; this change does not migrate a real business database.

## Verification

Real PostgreSQL tests cover the original loss after evaluation, current hypothesis
reads, immutable prior snapshots, concurrent replay without TTL renewal, distinct
facts with shared labels/source, reordered multi-claim replay, honest legacy
snapshots, rejected/expired rows, older evaluations and transaction rollback.
Upgrade, fresh installation, head rerun and storage constraints are checked with
existing released-migration immutability regressions. Existing claim uncertainty,
opaque references, direct settlement and Memory-source correction tests remain.
These are deterministic/controlled checks, not real-model semantic acceptance.
