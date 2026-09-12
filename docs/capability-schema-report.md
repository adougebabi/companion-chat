# Capability schema report

The pre-refactor renderer serialized the 11 production manifests to 7,857
bytes (ordinary conversation: 7,564 bytes; native cognition: 6,500 bytes), as
recorded in the task research baseline. The largest thick contracts were
`schedule.replan` (1,514 bytes), `memory_event` (1,375), and `scene_event`
(1,099).

The refactored renderer is `RenderCapabilityTools`. It consumes only
`CapabilityDefinition.InputSchema`; there is no second Parameters schema or
manifest-to-definition conversion in the active runtime. Use
`CapabilityToolSchemaStats` to record exact post-change bytes/chars for a catalog:

```go
bytes, chars := core.CapabilityToolSchemaStats(registry.Catalog(surface))
```

After the Prompt Context/Memory additions and the S12 future-event time-contract
hardening, the deterministic catalog measurement is 8,005 bytes/chars for all
13 definitions, 7,467 for the 11-definition conversation surface, and 6,495 for the 9-definition
native-cognition surface. The two added definitions are the transactional
`active_memory_event` and conversation-only pure QUERY `memory.recall`. The
Active definition requires original time expression/precision for creates and
an expiry for `future_event`, and explains the relevance-start boundary. The
JSON is ASCII-only for the current definitions, so bytes and rune count are equal.

Actual Provider token usage remains unavailable unless the response envelope
exposes `usage`. Prompt Context Assembly separately uses its documented
conservative pre-call estimator for capacity enforcement; that estimate is not
reported as actual Provider token usage.

The post-change catalog boundaries are metadata-driven:

- conversation: no `moment.publish` or WakeUp-only initialization;
- WakeUp/autonomy: installed capabilities for those surfaces;
- native cognition: capabilities declared for native cognition;
- reflection: no capability catalog unless a future definition explicitly
  declares that surface.

Every schema regression test checks that image/schedule/memory contracts do not
expose renderer, schedule revision/completion, storage/evidence, actor, or
idempotency implementation fields.
