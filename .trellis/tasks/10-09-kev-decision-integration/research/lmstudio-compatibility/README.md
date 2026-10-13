# LM Studio compatibility probe

`probe.py` uses only Python's standard library and fixed synthetic prompts. It
sends serial POST requests to the explicitly supplied OpenAI-compatible base
URL plus `/chat/completions`. It never loads/downloads a model, changes server
configuration, reads application credentials, calls Core, retries a request,
or executes a tool. The read-only tool definitions have no implementation.

Local verification, with no inference or network request:

```sh
python3 .trellis/tasks/10-09-kev-decision-integration/research/lmstudio-compatibility/probe.py --self-test
```

Run the same matrix against an already available model:

```sh
python3 .trellis/tasks/10-09-kev-decision-integration/research/lmstudio-compatibility/probe.py \
  --base-url http://localhost:1234/v1 \
  --model MODEL_ID \
  --output /tmp/lmstudio-compatibility-MODEL_ID.json
```

The endpoint is explicit; this harness provides no authentication/header option
and rejects credentials, query strings and fragments in the base URL. Redirects
are not followed. Supply the OpenAI base path (`/v1` for LM Studio), rather than
its native `/api/v1` API. Run each candidate with the same cases and endpoint to
compare models. Model/template/backend/load settings must be recorded separately
by the operator: the probe cannot attribute a failure to model ability alone.

`--case NAME` can be repeated to select individual cases. All omitted cases
remain `NOT_RUN` in the report, with their fixed request intact. `--timeout`
defaults to 30 seconds and is limited to 120 seconds. Requests use temperature
zero, explicit `enable_thinking=false`, the same `max_completion_tokens` key as Core, and at most 128 output tokens (256 for
the conditional schema). Responses are bounded to 2 MiB and actual HTTP error
bodies to 16 KiB, with explicit truncation flags. Request/response bodies are
saved; request/response headers and application secrets are never collected.

The matrix covers:

| Case | Required prerequisite | Acceptance |
| --- | --- | --- |
| `text` | Yes | Exact fixed text, no native call |
| `strict_json` | Yes | Exact closed JSON object |
| `nullable_anyof` | No | Exact null-valued JSON object |
| `conditional_oneof` | No | Exact discriminator/number JSON object |
| `required_empty` | Yes | At least one native call with valid ID and `{}` |
| `required_underscore` | Yes | At least one native `probe_read` call with exact arguments |
| `required_dot` | Yes | At least one native `probe.read` call with exact arguments |
| `auto` | No | Native call under auto choice for a direct tool request |
| `none` | No | Exact fixed text under none choice |
| `required_sse` | Yes | Actual SSE deltas, valid native call, and `[DONE]` |
| `two_tools` | No | At least one valid native call; multiple calls recorded separately |
| `tools_strict_json` | No | Required native call while final JSON schema is supplied |
| `auto_tools_strict_json` | No | Native call under auto while final JSON schema is supplied; known lazy-grammar rejection recorded separately |
| `continuation_empty` | Yes | Exact previous synthetic ToolResult after assistant content `""` |
| `continuation_null` | No | Same continuation with assistant content `null` |

The continuation supplies a synthetic assistant call ID `probe0001` and a
matching `tool` message whose variable text is `PROBE_RESULT_7318`. The final
user message asks to consume that result without containing its answer. This
tests message-template handling and feedback consumption; it is not evidence
of any real tool execution or business commit.

Native checks use only `message.tool_calls` or actual SSE `delta.tool_calls`,
aggregating argument fragments by their real tool index. Text/JSON resembling
a call cannot pass. Reports retain each native ID, argument parsing result,
finish reason, usage, original response and (for streams) decoded SSE events.
Missing termination/usage fields remain null; token exhaustion is not inferred.
For tools plus JSON, `return_shape` distinguishes native calls from final
content and `final_json_parse_valid` reports content parsing independently.
Two offered tools do not establish multi-call support unless the actual
`multiple_native_calls_observed` field is true.

`PASS` means the synthetic case's assertion passed. `FAIL` means an otherwise
parseable successful response did not satisfy its content/native assertion.
`UNSUPPORTED` requires an explicit HTTP 400/422/501 body stating unsupported,
not supported, or not implemented. Other HTTP/network/timeouts/malformed
response/observation-limit failures are `BLOCKED`, with separate failure layer
and actual bounded error evidence. `NOT_RUN` cannot count as acceptance.

Exit code is **0 only when every required prerequisite passes**; otherwise it
is **1**. Optional failures/unsupported features remain visible but do not
establish or invalidate the narrower prerequisite gate. Selecting only one
case therefore exits 1 because other prerequisites were not run. A green
matrix proves only these bounded synthetic provider prerequisites, not complete
Core/Eino acceptance, model accuracy, arbitrary JSON Schema support, production
prompt behavior, cancellation propagation, or domain success.

The 2026-10-13 audit records the original-template baseline, the V7-template
Engine Protocol ON matrix, and the matrix after the user disabled Engine
Protocol in `baseline-results.json`, `engine-on-native-v7-results.json`, and
`engine-off-results.json`. Requests are identical across these matrices.
The original OFF report passes all 14 then-defined cases. A later real Agent
failure exposed the omitted auto+JSON-schema combination. The current probe adds
it as an optional 15th case: its rejection must remain visible, while Core avoids
that combination by keeping all native Tool phases free of final response grammar.
See `auto-schema-repro-results.json` and `production-final-phase-failure.json`.
The effective legacy-mode template and GUI
switch were not independently observed in that locked-screen phase; see
`engine-off-environment.json` and `audit.md` for the attribution limits.

`engine-off-roundtrip-results.json` adds five synthetic requests in two dependent
sequences, preserving actual outbound `request_json` strings and immutable
request snapshots. Each sequence reuses IDs actually returned by the preceding
native response, inserts only fixture results, and ends with tool-free strict
JSON. The discovery sequence grows the actual next request schema.
`engine-off-roundtrip-initial-recording-defect.json` is historical evidence of
the first recorder's mutable-message aliasing defect; it must not be used as
acceptance. The corrected report came from fresh requests, not reconstructed
historical payloads. None of these sequences executes a business tool.
