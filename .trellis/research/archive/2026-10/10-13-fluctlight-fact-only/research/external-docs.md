# 外部接口文档核对（2026-10-13）

通过 `npx ctx7@latest library` 先解析，再以 `docs` 查询；命令在沙箱外执行，未传凭据或私聊内容。每个问题各使用两次查询。

- Eino 选择官方高可信文档 `/websites/cloudwego_io`（6574 snippets，85.86），仓库实际依赖 `github.com/cloudwego/eino v0.7.37`；检索未返回该精确版本文档。文档说明工具真实执行结果可回填、可纠正执行错误不必转化为整段假成功。当前仓库的 adapter/Runner 是实施依据；不引入最新文档中可能不属于 v0.7.37 的 Agentic API 或额外 middleware。
- LM Studio 选择 `/websites/lmstudio_ai_developer`，核对 `/v1/chat/completions` 的 messages、temperature、top_p、max_tokens；聊天模板由服务自动应用。检索也返回旧 REST v0 示例，方案保持现有 /v1 兼容适配器，不迁移到 v0。
- 查询结果支持无工具的独立回复请求与参数传递，不证明当前部署健康、视觉可用或自然对话效果。finish_reason=length 的拒发规则来自本次需求；现有 adapter 的解析/usage 保存需要在实施中专测，不能将示例当通过证据。

## 文档摘录

### Wrap Tool Execution Errors with ADK Middleware

Source: https://www.cloudwego.io/docs/eino/faq

Use `WrapInvokableToolCall` in `ChatModelAgentMiddleware` to convert tool execution errors into string results. This prevents interruption of the Agent flow and allows the model to handle the error.

```go
func (m *safeToolMiddleware) WrapInvokableToolCall(
    _ context.Context,
    endpoint adk.InvokableToolCallEndpoint,
    _ *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
    return func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
        result, err := endpoint(ctx, args, opts...)
        if err != nil {
            if _, ok := compose.IsInterruptRerunError(err); ok {
                return "", err // Don't convert interrupt errors
            }
            return fmt.Sprintf("[tool error] %v", err), nil
        }
        return result, nil
    }, nil
}

```

--------------------------------

### Agentic Model Tool Call Result Output

Source: https://www.cloudwego.io/docs/eino/core_modules/components/agentic_chat_model_guide

Illustrates the output structure for a FunctionToolResult block returned to the agentic model. This format is used to convey the results of executed tools back to the model.

```go
&schema.AgenticMessage{
    Role: schema.AgenticRoleTypeUser,
    ContentBlocks: []*schema.ContentBlock{
        schema.NewContentBlock(&schema.FunctionToolResult{
            CallID: "call_1",
            Name:   "get_weather",
            Content: []*schema.FunctionToolResultContentBlock{
                {
                    Type: schema.FunctionToolResultContentBlockTypeText,
                    Text: &schema.UserInputText{Text: "sunny"},
                },
            },
        }),
    },
}
```

--------------------------------

### Invoke and Stream Tool Execution

Source: https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide

Methods for running tools using `Invoke` for direct execution or `Stream` for streaming results.

```go
// run tools using `Invoke`
func (tn *ToolsNode) Invoke(ctx context.Context, input *schema.Message,
    opts ...ToolsNodeOption) ([]*schema.Message, error)
    
// run tools using `Stream`
func (tn *ToolsNode) Stream(ctx context.Context, input *schema.Message,
    opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error)
```

--------------------------------

### Tool Callback Logs: OnToolStart and OnToolEnd

Source: https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual

These log snippets show the execution flow of tool calls within the agent. They indicate when a tool starts and ends, along with the parameters used and the results obtained.

```text
=========[OnToolStart]=========
{"restaurant_id": "1001", "topn": 5}
=========[OnToolEnd]=========
[{"name":"Braised Pork","desc":"A piece of braised pork","price":20,"score":8},{"name":"Spring Beef","desc":"Lots of boiled beef","price":50,"score":8},{"name":"Stir-fried Pumpkin","desc":"Mushy stir-fried pumpkin","price":5,"score":5},{"name":"Korean Spicy Cabbage","desc":"This is blessed spicy cabbage, very delicious","price":20,"score":9},{"name":"Hot and Sour Potato Shreds","desc":"Sour and spicy potato shreds","price":10,"score":9}]
```

```text
=========[OnToolStart]=========
{"restaurant_id": "1002", "topn": 5}
=========[OnToolEnd]=========
[{"name":"Braised Spare Ribs","desc":"Piece by piece spare ribs","price":43,"score":7},{"name":"Big Knife Twice-cooked Pork","desc":"Classic twice-cooked pork, big pieces of meat","price":40,"score":8},{"name":"Fiery Kiss","desc":"Cold pig snout, spicy but not greasy","price":60,"score":9},{"name":"Chili Mixed with Preserved Egg","desc":"Pounded chili preserved egg, a rice killer","price":15,"score":8}]
```

### 4. Let the Model Self-correct After Tool Execution Failure (Instead of Interrupting the Flow)

Source: https://www.cloudwego.io/docs/eino/faq

When a tool execution fails in Eino, the default behavior is to interrupt the agent flow. A recommended approach is to return error information as a normal Tool Result to the model, enabling automatic correction and retries. This can be achieved using ADK Middleware's `WrapInvokableToolCall` method or directly within the compose layer using `ToolCallMiddlewares`.


### Python example: Create chat completion

Source: https://lmstudio.ai/docs/developer/openai-compat/chat-completions

Use the OpenAI Python client to send a chat completion request to a local LM Studio server. The prompt template is applied automatically for chat-tuned models. Provide inference parameters like temperature in the payload. The response is printed to the console.

```python
from openai import OpenAI
client = OpenAI(base_url="http://localhost:1234/v1", api_key="lm-studio")

completion = client.chat.completions.create(
  model="model-identifier",
  messages=[
    {"role": "system", "content": "Always answer in rhymes."},
    {"role": "user", "content": "Introduce yourself."}
  ],
  temperature=0.7,
)

print(completion.choices[0].message)
```

--------------------------------

### POST /api/v0/chat/completions

Source: https://lmstudio.ai/docs/developer/rest/endpoints

Chat Completions API. You provide a messages array and receive the next assistant response in the chat. Supports streaming and various generation parameters.

```APIDOC
## POST /api/v0/chat/completions

### Description
Chat Completions API. You provide a messages array and receive the next assistant response in the chat.

### Method
POST

### Endpoint
/api/v0/chat/completions

### Parameters
#### Request Body
- **model** (string) - Required - The model identifier to use for the completion.
- **messages** (array) - Required - Array of message objects, each with role and content.
  - **role** (string) - The role of the message (e.g., "system", "user", "assistant").
  - **content** (string) - The content of the message.
- **temperature** (number) - Optional - Sampling temperature (e.g., 0.7).
- **max_tokens** (integer) - Optional - Maximum number of tokens to generate. Use -1 for no limit.
- **stream** (boolean) - Optional - Whether to stream the response.

### Request Example
```bash
curl http://localhost:1234/api/v0/chat/completions \
  -H "Authorization: Bearer $LM_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "granite-3.0-2b-instruct",
    "messages": [
      { "role": "system", "content": "Always answer in rhymes." },
      { "role": "user", "content": "Introduce yourself." }
    ],
    "temperature": 0.7,
    "max_tokens": -1,
    "stream": false
  }'
```

### Response
#### Success Response (200)
- **id** (string) - Completion ID.
- **object** (string) - Always "chat.completion".
- **created** (integer) - Unix timestamp of creation.
- **model** (string) - Model used.
- **choices** (array) - Array of completion choices.
  - **index** (integer) - Choice index.
  - **logprobs** (null) - Log probabilities (null).
  - **finish_reason** (string) - Reason for finishing (e.g., "stop").
  - **message** (object) - Assistant message.
    - **role** (string) - Role ("assistant").
    - **content** (string) - Generated content.
- **usage** (object) - Token usage.
  - **prompt_tokens** (integer) - Number of prompt tokens.
  - **completion_tokens** (integer) - Number of completion tokens.
  - **total_tokens** (integer) - Total tokens.
- **stats** (object) - Generation statistics.
  - **tokens_per_second** (number) - Tokens per second.
  - **time_to_first_token** (number) - Time to first token in seconds.
  - **generation_time** (number) - Total generation time in seconds.
  - **stop_reason** (string) - Reason for stopping (e.g., "eosFound").
- **model_info** (object) - Model information.
  - **arch** (string) - Model architecture.
  - **quant** (string) - Quantization.
  - **format** (string) - Model format.
  - **context_length** (integer) - Context length.
- **runtime** (object) - Runtime information.
  - **name** (string) - Runtime name.
  - **version** (string) - Runtime version.
  - **supported_formats** (array) - Supported formats.

#### Response Example
```json
{
  "id": "chatcmpl-i3gkjwthhw96whukek9tz",
  "object": "chat.completion",
  "created": 1731990317,
  "model": "granite-3.0-2b-instruct",
  "choices": [
    {
      "index": 0,
      "logprobs": null,
      "finish_reason": "stop",
      "message": {
        "role": "assistant",
        "content": "Greetings, I'm a helpful AI, here to assist,\nIn providing answers, with no distress.\nI'll keep it short and sweet, in rhyme you'll find,\nA friendly companion, all day long you'll bind."
      }
    }
  ],
  "usage": {
    "prompt_tokens": 24,
    "completion_tokens": 53,
    "total_tokens": 77
  },
  "stats": {
    "tokens_per_second": 51.43709529007664,
    "time_to_first_token": 0.111,
    "generation_time": 0.954,
    "stop_reason": "eosFound"
  },
  "model_info": {
    "arch": "granite",
    "quant": "Q4_K_M",
    "format": "gguf",
    "context_length": 4096
  },
  "runtime": {
    "name": "llama.cpp-mac-arm64-apple-metal-advsimd",
    "version": "1.3.0",
    "supported_formats": ["gguf"]
  }
}
```
```

--------------------------------

### Supported payload parameters

Source: https://lmstudio.ai/docs/developer/openai-compat/chat-completions

Lists the parameters that can be included in the chat completion request payload. See the OpenAI API reference for parameter semantics.

```python
model
top_p
top_k
messages
temperature
max_tokens
stream
stop
presence_penalty
frequency_penalty
logit_bias
repeat_penalty
seed
```

### LM Studio 0.3.18 > Streaming options and tool‑calling improvements

Source: https://lmstudio.ai/docs/developer/api-changelog

* Added support for the `stream_options` object on OpenAI‑compatible endpoints. Setting `stream_options.include_usage` to `true` returns prompt and completion token usage during streaming [‡](https://lmstudio.ai/blog/lmstudio-v0.3.18#:~:text=%2A%20Added%20support%20for%20%60,to%20support%20more%20prompt%20templates).
* Errors returned from streaming endpoints now follow the correct format expected by OpenAI clients [‡](https://lmstudio.ai/blog/lmstudio-v0.3.18#:~:text=,with%20proper%20chat%20templates).
* Tool‑calling support added for Mistral v13 tokenizer models, using proper chat templates [‡](https://lmstudio.ai/blog/lmstudio-v0.3.18#:~:text=,with%20proper%20chat%20templates).
* The `response_format.type` field now accepts `"text"` in chat‑completion requests [‡](https://lmstudio.ai/blog/lmstudio-v0.3.18#:~:text=,that%20are%20split%20across%20multiple).
* Fixed bugs where parallel tool calls split across multiple chunks were dropped and where root‑level `$defs` in tool definitions were stripped [‡](https://lmstudio.ai/blog/lmstudio-v0.3.18#:~:text=,being%20stripped%20in%20tool%20definitions).

--------------------------------

### Structured Output

Source: https://lmstudio.ai/docs/developer/openai-compat/structured-output

You can enforce a particular response format from an LLM by providing a JSON schema to the `/v1/chat/completions` endpoint, via LM Studio's REST API (or via any OpenAI client).


