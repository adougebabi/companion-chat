# Diagnostic trace excerpt

Source: `apps/core-go/internal/core/eino_model_runtime.go` and
`apps/core-go/internal/core/adk_conversation_runtime.go`.

The recorded bounded chain is:

```text
adk.model.input
  → adk.tool.requested / adk.tool.rejected / adk.tool.dispatched
  → adk.tool.result
  → adk.model.output
  → adk.run.termination
```

`adk.model.input` records only message counts, formal tool-result IDs and the
number of assistant/tool pairs matched in the actual next input. Tool arguments
are represented by a digest. Every event carries the parent correlation and a
bounded `run_id`; database/export filtering is owner-authorized and redacted.
