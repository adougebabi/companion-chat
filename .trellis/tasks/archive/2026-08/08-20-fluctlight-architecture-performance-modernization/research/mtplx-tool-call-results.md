# MTPLX Native Tool-Call Verification

Date: 2026-08-20

## Endpoint

- Base URL: `http://100.80.75.9:8000/v1`
- Model: `qwen3.8-27b-abliterated-mtplx-optimized-speed`
- `/v1/models` returned HTTP 200 with one model and context length 262,144.
- The unauthenticated probe returned HTTP 401 as expected; no credential is recorded here.

## Tests

The reproducible probe is [`mtplx-tool-call-demo.mjs`](./mtplx-tool-call-demo.mjs). It uses a read-only `get_weather(city)` tool and never calls a project capability.

### Non-streaming

Request: `stream=false`, `tool_choice=required`.

Observed result:

```json
{
  "finish_reason": "tool_calls",
  "tool_calls": [
    {
      "id": "call_<generated>",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": "{\"city\":\"Shanghai\"}"
      }
    }
  ]
}
```

### Streaming

Request: same tool, `stream=true`, `max_tokens=64`.

Observed result: HTTP 200, 15-17 SSE chunks, and a terminal `[DONE]`. The tool call was split across chunks:

1. A chunk contained `delta.tool_calls[0]` with `index: 0`, generated `id`, `type: "function"`, function `name: "get_weather"`, and an empty argument fragment.
2. A later chunk contained the same index with `function.arguments: "{\"city\":\"Shanghai\"}"`.
3. The final tool chunk had `finish_reason: "tool_calls"`.

The model also emitted `reasoning_content` chunks before the tool call. The current server intentionally consumes `delta.content` and `delta.tool_calls`, so hidden reasoning does not leak to the visible chat stream.

### Tool-result follow-up

The tool call was returned to MTPLX as an assistant `tool_calls` message followed by a `tool` message with the matching `tool_call_id`. With `tool_choice=none` and `max_tokens=256`, MTPLX returned a normal assistant response with `finish_reason: "stop"`.

## Conclusion

The configured MTPLX instance natively supports the OpenAI-compatible tool-call lifecycle required by this project:

- non-streaming structured tool calls;
- streaming `tool_calls` with fragmented arguments;
- assistant tool-call history plus tool-result follow-up;
- stable generated call IDs and `index` values.

The project can migrate media and pending-event capabilities from marker transport to native tools. The migration still needs a compatibility fallback for older models and must preserve the current application-level SSE envelope (`token`, `done`, `error`). Tool arguments must be accumulated by `index`, validated only after the complete call is assembled, and kept out of visible token events.

The demo's first follow-up run used `max_tokens=128` and ended with `finish_reason: "length"`; after raising the budget to 256 it ended with `stop`. This is a model budget signal, not a tool-call compatibility failure.
