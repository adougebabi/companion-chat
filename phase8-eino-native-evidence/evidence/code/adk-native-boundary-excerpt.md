# ADK native boundary excerpt

Source: `apps/core-go/internal/core/eino_model_runtime.go` and `apps/core-go/internal/core/provider.go`.

```go
// The Eino message is already the typed native boundary for both ADK
// and fixed-task calls. Do not recover calls from content or sidecars;
// valid typed siblings may survive a malformed sibling, but each accepted
// call must retain its real Eino ID.
calls, err := normalizeEinoNativeToolCallsIndependently(response.Message, providerRequestID)
```

```go
// A structured sidecar may still be present for a thinking-enabled provider,
// but its tool_calls field is diagnostic noise on an ADK completion.
delete(completion.Structured, "tool_calls")
```

The strict normalizer rejects missing/conflicting IDs and non-object arguments;
it never derives a call ID from name, position, request ID, prose, or reasoning.
