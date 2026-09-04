# Fluctlight Provider Contract

## Scenario: Explicit Model Roles With Capability Validation

### 1. Scope / Trigger

- Trigger: a model endpoint is configured, a role is assigned, capability preflight runs, or initialization/assessment/realization/reflection/embedding/media-prompt inference executes.
- Provider adapters normalize protocol and transport. They do not own Fluctlight semantics, numeric state, policy, or side effects.
- First delivery supports explicit role assignment without implicit fallback chains.

### 2. Signatures

```text
ProviderEndpoint
  id / kind / base_url / encrypted_api_key
  capability_status / checked_at

ModelRole
  role: initialization | cognitive_assessment | action_realization |
        reflection | embedding | media_prompt
  provider_endpoint_id / model_id
  required_capabilities
  token_budget / timeout / retry_policy
```

```python
preflight(role: ModelRole) -> CapabilityReport
complete_structured(role, schema, input) -> StructuredResult
stream_realization(role, input) -> AsyncIterator[ProviderChunk]
embed(role, inputs) -> VersionedEmbeddings
```

### 3. Contracts

- Generative roles may share one endpoint/model, but remain independent settings with independent budgets and provenance.
- `initialization`, `cognitive_assessment`, and `reflection` require strict structured-output/schema preflight.
- `action_realization` requires streaming, abort propagation, bounded diagnostics, and correct UTF-8/chunk handling.
- `embedding` requires an embedding endpoint and fixed dimensions recorded with each vector/index version.
- `media_prompt` requires its declared structured/text output contract and cannot execute media generation itself.
- `media_prompt` may serve both B text prompt generation and C structured multimodal acceptance using the same configured role/model; C transport, vision-capability, timeout, or schema failures are surfaced to the owning media flow as an infrastructure condition, never inferred as a content `pass` or `reject`.
- Settings cannot activate a role until preflight proves required capabilities. Health may later degrade without making Core readiness false.
- Every result records role, endpoint/model ID, capability/model version when available, prompt/schema version, timing, token usage/budget, and correlation IDs.
- No implicit role/model fallback. Failure follows explicit interaction/workflow retry/deferred/no-op/terminal rules.
- Provider adapter returns normalized transport/structured results and bounded parse diagnostics. It does not parse visible prose for semantic effects or choose domain actions.
- API keys are resolved only in Go Core through the configuration secret contract and never returned to BFF/browser/debug output.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Role has no endpoint/model assignment | Role unavailable with explicit configuration error; no fallback. |
| Structured role returns an empty/mismatched transport shape | Normalize only the affected fields (missing → typed empty, object ↔ array container repair), preserve native tool calls independently, and let the owning domain validator decide whether the resulting semantic payload is usable; never parse arbitrary prose. |
| Realization role lacks streaming/abort | Preflight fails; role cannot activate. |
| Embedding dimensions change unexpectedly | Reject vectors, mark role/index mismatch, require new embedding version. |
| Timeout/token budget exceeded | Cancel/bound result and follow owning retry/terminal policy. |
| Provider/model is temporarily unavailable | Report degraded role health; request/workflow handles explicit failure. |
| API key decryption fails | Configuration error; do not use env/old-key fallback. |
| Provider returns hidden reasoning/raw diagnostics | Bound/redact and keep out of ordinary result/trace/browser contract. |

### 5. Good / Base / Bad Cases

- Good: one local chat model passes five role preflights with separate budgets; every artifact records its actual role/model/prompt version.
- Good: an embedding model upgrade creates a new dimension/model index and background rebuild without mixing distances.
- Base: reflection role is degraded while realization remains healthy; interactions continue, reflection workflows retry explicitly.
- Bad: one global model string with unknown capabilities, silently substitute realization for assessment, parse malformed structured output as prose, or hide fallback under Provider adapter logic.

### 6. Tests Required

- Endpoint/settings tests for encrypted keys, safe summaries, role assignment, shared model mapping, and atomic invalid-patch rollback.
- Role-specific preflight tests for structured schema, stream/abort/chunking, embedding dimensions, media-prompt output, timeout, and token budgets.
- Provenance tests assert every result stores role/endpoint/model/prompt/schema/correlation metadata without credentials or hidden reasoning.
- Failure tests prove one degraded role does not silently use another and follows owning interaction/workflow policy.
- Provider adapter contract suite runs against fake normalized adapters and configured OpenAI-compatible test endpoints.
- Architecture tests prevent Provider adapters from importing domain policy/repositories or implementing semantic regex/keyword fallbacks.

### 7. Wrong vs Correct

#### Wrong

```python
model = settings.default_model
try:
    return await provider.complete(model, prompt)
except Exception:
    return await provider.complete(settings.fallback_model, prompt)
```

#### Correct

```python
role = model_roles.require("cognitive_assessment")
role.require_capability("structured_output", schema_version)
return await provider.complete_structured(
    role=role,
    schema=SemanticAssessmentV1,
    input=assessment_input,
)

## Scenario: Go Provider Preflight And Explicit Media Failure

### 1. Scope / Trigger

- Trigger: Go Core configures a model role or a media workflow receives an
  endpoint/model capability error.

### 2. Signatures

- `ConfigureProviderRole(ctx, actorID, payload)` persists a preflight only
  after the endpoint model list contains the selected model.
- Provider calls carry deterministic idempotency/request headers.
- `ProviderModels(ctx, actorID, endpointID)` normalizes common model-list
  envelopes: OpenAI-style `data[]` and Ollama-style `models[]`, accepting
  string entries and object `id`/`name`/`model` fields. Unknown envelopes stay
  empty and cannot activate a role.

### 3. Contracts

- Endpoint reconfiguration invalidates bound roles until the next preflight.
- A missing ComfyUI model is a bounded failure; no alternate model is chosen.
- Successful and failed model runs are recorded with redacted diagnostics.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| endpoint/model missing or unavailable | reject role; return `provider_endpoint_not_found`, `provider_models_unavailable`, or `provider_model_not_available`; no role row |
| model list uses a supported `data[]`/`models[]` envelope | normalize IDs, deduplicate and sort before matching |
| model list is empty or has an unsupported envelope | reject role with `provider_model_not_available`; no role row |
| configured media model absent | retry, then mark media intent `failed` |
| diagnostic contains credentials/hidden reasoning | redact/drop before persistence |

### 5. Good/Base/Bad Cases

- Good: preflight passes and retries reuse one request ID.
- Base: a previously healthy endpoint degrades and returns a bounded Provider
  error while Core readiness remains healthy.
- Bad: silently selecting the first available transformer after a 400.

### 6. Tests Required

- Fake `/models` preflight success/unknown-model tests.
- Header idempotency and recursive diagnostic-redaction tests.
- Real ComfyUI model-not-found test asserting failed durable media state.

### 7. Wrong vs Correct

#### Wrong

```go
workflow["transformer"] = firstAvailableModel()
```

#### Correct

```go
markMediaIntentFailed("provider_model_not_available")
return err
```
```
