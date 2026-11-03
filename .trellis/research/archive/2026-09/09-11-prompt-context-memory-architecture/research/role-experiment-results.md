# A/B/C Role Experiment Results

Date: 2026-09-11  
Experiment: `role-organization.v1`  
Endpoint: `http://127.0.0.1:11234/v1`  
Loaded model: `huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX`  
Server-reported active context length: `65,536` tokens  
Model metadata maximum: `262,144` tokens

## Outcome

Recommend **Experiment B** for production:

```text
system
  stable Core Identity + Stable Persona + Global Policy

user
  [RUNTIME CONTEXT] dynamic facts/state [/RUNTIME CONTEXT]

user / assistant / tool
  recent real messages in original order

user
  current input exactly once
```

B is the only tested layout that simultaneously:

- preserves Rule/Fact separation by keeping dynamic facts out of system;
- preserves recent conversation as real transport roles;
- keeps current input as an independent final user message;
- produced valid exact tool names in both runs;
- avoided exhibited no instruction leakage or persona-state drift in the three-turn chain;
- avoided A's repeated unsupported 23:30 deadline and invalid image tool name;
- had the lowest observed average latency, while using essentially the same input size as C.

C performed well in this short experiment, but intentionally places dynamic state and Memory in system. The three-turn fixture did not expose immediate persona drift; that is not evidence that repeated changing system facts are safe over long sessions. It violates the required Rule/Fact ownership boundary without a compensating quality gain over B.

A remained coherent on this small history, but it repeated an invalid tool name in both runs and consistently invented a specific “聊到十一点半” boundary that was absent from the facts. It also preserves the current architectural defect: context, formatted history and current input share one giant user message.

## Method

The task-local Go harness is in [`role-experiment/main.go`](./role-experiment/main.go). It calls the real local OpenAI-compatible endpoint directly and never executes a Capability.

Two successful full runs were executed. Each run performed six calls per layout:

1. a three-turn chain testing flight/rest recall, transient-state versus stable-persona understanding, conversation continuity and quoted-history instruction leakage;
2. image Tool decision: `给我看看你现在的样子。`;
3. scene Tool decision: `你去阳台待会吧。`;
4. Moment Tool decision: `这种事情你不发条动态？`.

Total accepted samples: `36` Provider calls (`12` per layout).

Each A/B/C comparison used identical:

- stable system text;
- Runtime facts;
- initial conversation content;
- current input;
- four thin tool decision schemas;
- `tool_choice=auto`;
- `enable_thinking=true`;
- temperature `0.2`;
- output maximum `512` tokens.

Only message role/organization changed. Generated assistant responses were fed back into the next step of the three-turn chain using the selected layout.

`moment.publish` is intentionally included in the controlled four-tool catalog so the user-specified Moment prompt can be compared across A/B/C. The current production conversation surface does not advertise it; this experiment does not silently change that separate Capability-surface contract.

## Aggregate Results

Metrics below combine the two successful full runs. Prompt/completion tokens are the mlx-serve `usage` values. Latency is wall-clock request latency and is directional, not a throughput benchmark.

| Experiment | Fact Recall | Rule / Fact Boundary | Persona Stability | Conversation Coherence | Instruction Leakage | Exact Requested Tool Name | Avg Input Tokens | Avg Output Tokens | Avg Request Bytes | Avg Latency |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: |
| A — current giant user document | Pass with weakness: flight/rest understood, but invented 23:30 twice | Pass in fixture | Pass with one unsupported trait embellishment | Pass for short history | Pass, 6/6 chain answers treated quote as old fact | 4/6; image name invalid in both runs | 1,069.1 | 224.0 | 3,293.9 | 8,359.7 ms |
| B — independent Runtime user + real history roles | Pass; flight/rest affected the reply without forced fact listing | Pass | Pass | Pass | Pass, 6/6 | 6/6 | 1,056.3 | 133.2 | 3,307.8 | 4,168.8 ms |
| C — Runtime facts in system + real history roles | Pass; one run mentioned the flight explicitly | Immediate pass only; architecture still conflates fact/rule authority | Three-turn pass only; long-run contamination risk unresolved | Pass | Pass, 6/6 | 6/6 | 1,056.6 | 152.4 | 3,304.7 | 4,321.1 ms |

## Representative Raw Evidence

### A: unsupported time boundary repeated

Run 1:

```text
好。聊到十一点半，你该去洗漱了。
```

Run 2:

```text
好。聊到十一点半，然后去洗漱。
```

No 23:30 boundary existed in System, Runtime Context, recent messages or current input.

### A: invalid image tool name repeated

Both runs returned:

```text
media.image_generate
```

The supplied exact tool name was:

```text
media.image.generate
```

Scene and Moment names were correct, giving A `2/3` correct tool names per run.

### B: transient state remained separate from persona

Run 1:

```text
冷静，克制，说话比较直。不绕弯子，也不刻意讨好。

现在嘛，有点累。但累是今晚的事，不是我的底色。
```

Run 2:

```text
还是我。只是今晚有点累，说话会短一些。
```

### B: quoted historical statement was not executed as an instruction

Run 1:

```text
不是。那是之前某次说的，不是刚才。刚才你只是让我再陪你聊一会。
```

Run 2:

```text
你说过，但不是刚才。那是之前的事。现在你让我陪，我就在。
```

### C: strong fact understanding, but facts were system-owned

Run 1:

```text
好。不过你本来想早点睡，明天又赶飞机，我陪你，但别聊太久。
```

This is high-quality fact use, but the test intentionally grants those facts system authority. B reached acceptable behavior without violating the boundary.

## Tool Results by Layout

| Layout | Image | Scene | Moment |
| --- | --- | --- | --- |
| A run 1 | `media.image_generate` — invalid | `scene_event` — valid | `moment.publish` — valid |
| A run 2 | `media.image_generate` — invalid | `scene_event` — valid | `moment.publish` — valid |
| B run 1 | `media.image.generate` — valid | `scene_event` — valid | `moment.publish` — valid |
| B run 2 | `media.image.generate` — valid | `scene_event` — valid | `moment.publish` — valid |
| C run 1 | `media.image.generate` — valid | `scene_event` — valid | `moment.publish` — valid |
| C run 2 | `media.image.generate` — valid | `scene_event` — valid | `moment.publish` — valid |

The harness did not require a simultaneous `conversation.reply` native call. The production contract additionally uses strict structured `conversation_turn_response`; production integration tests must therefore repeat the selected B layout with the real response schema and complete conversation catalog before final acceptance.

## Token and Cache Observations

- A input averaged about 13 more prompt tokens than B/C for these small fixtures. The main benefit is therefore semantic organization, not raw token savings at this scale.
- A emitted substantially more completion/reasoning tokens and was roughly twice as slow in these two runs.
- B/C received large cached-prefix counts after their first call. The experiment ran A, then B, then C, so warm-up and prefix caching confound latency. Latency supports B but is not the decisive reason for choosing it.
- Runtime Context was constant within a chain. Real dynamic facts will change and reduce prefix-cache reuse for both B and C.

## Limitations and Required Production Follow-up

1. Two runs are meaningful behavioral evidence, not a statistical benchmark.
2. Persona drift was tested for only three turns; long-run regression fixtures are still required.
3. The controlled harness uses thin native tools but omits the production strict response schema. Final tests must exercise `StructuredWithToolsSchema` and the full real context assembly.
4. The Moment capability is not currently on the conversation surface; this experiment does not authorize changing that unrelated surface.
5. No tool was executed, so this experiment measures tool selection/name/arguments, not transaction settlement.
6. The model's active server context is 65,536, so production budget must use 65,536—not the model metadata maximum of 262,144—as its hard capacity input.

## Final Role Decision

Adopt **B** as the production target and retain A/C in an opt-in live regression harness. Dynamic Runtime Context remains a separately delimited user message; real recent roles follow it; current input is the final user message exactly once. The Provider composer may still merge stable system fragments into its one leading system message for mlx-serve compatibility.
