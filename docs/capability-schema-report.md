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

The deterministic catalog measurement is 4,864 bytes/chars for all 11
definitions, 4,326 for the 9-definition conversation surface, and 3,734 for
the 8-definition native-cognition surface. The JSON is ASCII-only for the
current definitions, so bytes and rune count are equal.

Provider token usage is intentionally reported as unavailable unless the
current Provider response envelope exposes `usage`; this change does not add a
tokenizer or estimate tokens from characters.

The post-change catalog boundaries are metadata-driven:

- conversation: no `moment.publish` or WakeUp-only initialization;
- WakeUp/autonomy: installed capabilities for those surfaces;
- native cognition: capabilities declared for native cognition;
- reflection: no capability catalog unless a future definition explicitly
  declares that surface.

Every schema regression test checks that image/schedule/memory contracts do not
expose renderer, schedule revision/completion, storage/evidence, actor, or
idempotency implementation fields.
