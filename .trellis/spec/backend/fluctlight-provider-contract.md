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
- Settings cannot activate a role until preflight proves required capabilities. Health may later degrade without making Core readiness false.
- Every result records role, endpoint/model ID, capability/model version when available, prompt/schema version, timing, token usage/budget, and correlation IDs.
- No implicit role/model fallback. Failure follows explicit interaction/workflow retry/deferred/no-op/terminal rules.
- Provider adapter returns normalized transport/structured results and bounded parse diagnostics. It does not parse visible prose for semantic effects or choose domain actions.
- API keys are resolved only in Python through the configuration secret contract and never returned to Node/browser/debug output.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Role has no endpoint/model assignment | Role unavailable with explicit configuration error; no fallback. |
| Structured role returns invalid schema | Reject result and follow owning failure policy; do not parse prose. |
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
```
