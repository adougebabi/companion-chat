# LM Studio 兼容性审计与运行模式对照

日期：2026-10-13（Asia/Shanghai）。应用侧来源审阅、实际独立协议请求与模板来源对照分开记录。

## 当前结论

用户关闭 Engine Protocol 后，同一 Cydonia IQ3_M 在公开 API 上通过全部14个当时定义的协议用例，另通过真实返回调用 ID 的私聊协议往返与动态发现往返。当前模型具备这些合成用例所要求的原生工具能力，不能归因为“模型不支持”或“LM Studio 整体不支持”。后续真实Agent暴露矩阵遗漏的auto工具+最终schema组合，服务明确HTTP400拒绝；Core共享请求层已本地修复为native阶段省略最终grammar，保留严格最终校验/隔离修正。此前Engine Protocol失败的内部具体环节仍未定位。本文保留各阶段证据，不将短协议验收扩写为正式业务验收。

## 修正前实测环境

- LM Studio 0.4.20 Build 1；Engine Protocol 运行路径由实际 HTTP 错误确认。
- 模型：cydonia-24b-v4.3-heretic-v2-i1，mradermacher GGUF IQ3_M；加载上下文32768、parallel4。
- 当前模型能力元数据：trained_for_tool_use=false、vision=false。它是当前服务能力识别，不能单独当作权重训练结论。
- 所有14条请求均为固定合成内容；没有 Core 调用、业务 Tool 执行、聊天发布、模型加载或服务设置改动。
- 使用与 Core 相同的 max_completion_tokens 键，128–256 输出预算、temperature0；原始请求与响应见 baseline-results.json。

## 现场矩阵

| 用例 | 状态 | HTTP | 原生调用数 | 终止原因 |
| --- | --- | --- | --- | --- |
| text | PASS | 200 | 0 | stop |
| strict_json | PASS | 200 | 0 | stop |
| nullable_anyof | PASS | 200 | 0 | stop |
| conditional_oneof | PASS | 200 | 0 | stop |
| required_empty | FAIL | 200 | 0 | stop |
| required_underscore | FAIL | 200 | 0 | length |
| required_dot | FAIL | 200 | 0 | length |
| auto | FAIL | 200 | 0 | length |
| none | PASS | 200 | 0 | stop |
| required_sse | FAIL | 200 | 0 | stop |
| two_tools | FAIL | 200 | 0 | length |
| tools_strict_json | FAIL | 200 | 0 | stop |
| continuation_empty | BLOCKED | 400 | — | — |
| continuation_null | BLOCKED | 400 | — | — |

合计5 PASS、7 FAIL、2 BLOCKED。SSE收到真实事件及DONE，但没有native tool_calls，所以原生SSE用例失败；不能把它写成SSE传输不支持。工具与严格JSON混合失败也尚不能单独归因两种格式冲突，原生前提已失败。

工具结果续轮两例的真实错误：

```text
provider HTTP 400: Unable to generate parser for this template.
Automatic parser generation failed: Jinja Exception:
Only user, system and assistant roles are supported!
```

## 来源与归因

1. 应用代码仍只执行原生ToolCalls；必要工具选择、动态加载、native IDs/receipts和最终tool-free repair不应被正文兼容解析取代。直接HTTP探测不经过应用/Eino，也重复出现无native调用，故当前首要问题不在应用工具执行器。
2. Cydonia与decensored发布者的模板相同，均无工具支持；上游是具备工具能力的Mistral Small3.2。当前Tokenizer注册的是V7工具标记，缺少V11的CALL_ID/ARGS；不能直接套用llama.cpp最新精确模型名模板。
3. 微调、abliteration、IQ3_M量化之后的残余工具能力未知。必须先修送达/序列化/解析，再用同权重重跑，才能讨论模型能力。
4. 当前仅有这一生成模型可用。没有通过下载/加载另一模型构造平台对照，也没有声明其视觉能力可用。

## 可审阅修复候选

- original-template.jinja：原模板备份（与观察到的当前发布模板一致，真正应用前仍须重读确认）。
- native-tools-v7.jinja：保持原Cydonia纯文本、连续user消息的格式，增加AVAILABLE_TOOLS、V7 JSON数组ToolCalls及TOOL_RESULTS/TOOL_CONTENT。
- 使用位置索引在最后user前放目录，重复相同user不重复放目录；IDs原样保留，不制造/截断ID。
- 与官方V7一样，模板输入合法argument JSON应先由服务规范化为对象；模板对残留的畸形字符串使用tojson保留为字符串。LM Studio实际规范化行为须在真实续轮中验证。
- 离线用Python Jinja2验证；未声称已在Engine Protocol/minja中通过，也未声称权重已能正确调用。

## 执行前控制实验计划（历史记录）

候选模板须在LM Studio的Load Parameters → Advanced → Chat Template应用并reload，随后对同一个模型重跑矩阵。模板修正后若native调用与续轮恢复，根因是模板/服务接入；若工具定义与历史已正确送达但模型仍失败，再判断权重或native解析器，必要时引入可靠原生工具模型对照。
该步骤改变当前正在被正式环境使用的本机模型加载配置，会短暂停止推理。记录此计划时尚未执行；后续经用户授权已应用并reload，结果见下节。保留原模板；不修改Core业务状态、Kev设置或预算。

## 已应用 V7 模板，Engine Protocol 仍开启

用户明确授权替换模板并重载。通过 LM Studio Load Parameters → Advanced → Chat Template 应用 native-tools-v7.jinja 并点击重新加载，界面 reload-needed 标记消失。执行时用户已将软件更新为0.4.26 Build4、上下文改为8192；这些更新不是助手操作。模板重载前后公开API的加载参数一致。

同一14例结果为5 PASS、9 FAIL、0 BLOCKED，见 engine-on-native-v7-results.json：工具定义进入模型输入，但原生生成用例仍全部没有 tool_calls；ToolResult历史从HTTP400变为HTTP200，模型能读取结果但添加说明/代码围栏，未满足精确输出断言。SSE传输及DONE正常，失败点为native字段。模板渲染缺陷已修复，运行路径的原生调用仍未恢复。

额外固定格式、few-shot和普通天气请求均无原生调用；不含tools的logprobs控制首token为`[`，不能据此断言parser吞掉了一个已生成的TOOL_CALLS token。带tools的logprobs请求被服务显式拒绝，故没有取得内部生成与解析的逐token对照。

## 用户关闭 Engine Protocol 后的公开接口复测

用户回复“我刚刚关了，似乎可以了？你测试下？”。助手保留该状态，通过相同公开端点重跑完全相同的14个请求，结果14 PASS、0 FAIL、0 BLOCKED，见 engine-off-results.json。Mac锁屏导致本轮无法独立读取Developer开关或旧运行模式实际采用的模板；off为用户报告，不能声称已通过界面再次核验。V7覆盖配置未由助手撤回，但旧运行模式是否采用该load-time override未单独证实。

公开model API确认同一模型key、IQ3_M、context8192、eval batch2048、physical batch512、parallel4、flash attention与KV offload。禁用的draft选项参数显示不同默认值，未宣称全配置逐字一致。模型能力元数据仍为trained_for_tool_use=false、vision=false，前者不应推翻实际原生工具结果。

| 验收组 | 原模板、Engine ON | V7模板、Engine ON | 用户关闭 Engine 后 |
| --- | --- | --- | --- |
| text / strict_json / nullable_anyof / conditional_oneof / none | 5 PASS | 5 PASS | 5 PASS |
| required_empty / required_underscore / required_dot / auto | 4 FAIL，无native | 4 FAIL，无native | 4 PASS，真实ID及参数 |
| required_sse / two_tools / tools_strict_json | 3 FAIL，无native | 3 FAIL，无native | 3 PASS，SSE及双调用均实际观察到 |
| continuation_empty / continuation_null | 2 BLOCKED，HTTP400 | 2 FAIL，HTTP200但额外正文 | 2 PASS，精确读取结果 |

当前two_tools真实观察到两个独立调用（并非只根据用例名字推断）。示例required_dot返回name=probe.read、arguments={"value":"7318"}、非空id=653492179、finish_reason=tool_calls。

额外两条依赖往返见 engine-off-roundtrip-results.json，共5个物理请求。独立核验发现首版探针记录持有可变messages引用，后续历史污染早期请求记录；该版单独保存在 engine-off-roundtrip-initial-recording-defect.json，不用于验收。主线程修正为保存实际发送的序列化request_json及反序列化快照，重新发送5个物理请求；以下PASS均取自新鲜重跑，没有回填或重建旧请求。私聊两步messages数2→5、发现三步2→5→8，下一请求原样接入上一响应的真实ID，末轮无tools：

1. 仅提供合成conversation.reply(text) schema → 服务返回真实nativecall及id → 以同一id注入合成receipt → 零tools严格JSON最终输出读取receipt：PASS。没有发送任何私聊。
2. 仅提供capability.discover(names) → 真实nativecall → 合成schema结果 → 下一物理请求实际加入probe.read并收到真实调用 → 合成结果 → 零tools严格JSON读取receipt：PASS。没有执行Core工具或改Agent授权。

当前短请求耗时0.30–3.46秒；这是极小合成请求的端到端时间，不是正式提示词或完整Agent的延迟上限。没有重试、正文转ToolCall、放宽合同或修改应用产品代码。保持用户关闭后的模式用于正式验收；后续若重开该模式或更换LM Studio/runtime/template，应再次跑同矩阵。此次不再把工具输出遗漏修成额外应用重试。

## 真实 Agent 最后一轮的 auto + schema 缺口与 Core 修复

用户指定turn_03f5a472-5d1f-429f-a966-f381e94f6e2b。只读诊断确认conversation_cognition共10次物理请求：前9轮native，最后一次在13:52:47.276→47.459（Asia/Shanghai）失败，failureStage=model、provider_request_failed、HTTP400 `Cannot combine structured output constraints with lazy grammar`。第9轮conversation.reply id=400669475已completed。此次没有进入最终DTO校验或settlement，页面“无法连接”文案不能替代真实技术详情。精简证据见production-final-phase-failure.json，没有保存私聊正文。

完全合成的三请求复现见auto-schema-repro-results.json：required+schema PASS200；auto+schema为同一HTTP400；tool-free+schema PASS200。原14例只测required+schema，缺少auto组合。现probe增加optional auto_tools_strict_json第15例，保留实际服务拒绝，不能宣称服务已支持该混合协议。

本地产品修复只在eino_model_request_policy.go：读取catalog narrowing及请求options之后的实际Tools，非空时统一省略response_format，同时effective format为nil供预算使用。包括没有task policy的工具Agent；required/auto、thinking和目录权限保留，不能在reply/root就绪时提前切断可选调用。原schema未修改；成功native停止后仍严格校验finalDTO，合法结果直接验收；非法结果沿用现有一次tool-free repair，原schema保留，tools/choice/parallel移除，thinkingfalse，原生历史/refs/取消保留。没有Provider400重试、正文转Tool、伪造ID/回执或新Agent框架。

实际SDK/Eino HTTP/SSE新回归在修前12组均RED，确切复现lazy-grammar HTTP400；修后12组GREEN，覆盖Generate/Stream×taskpolicy/no-policy×valid/repair/invalid-repair，真实ID和结果配对、reply后可选read、一个replyeffect、独立physical身份、合法final无多余请求、单次无Tool纠正及无效纠正继续失败。原Planner可选commit/snapshot/repeatedID测试和Goal/VisualIdentity/Conversation请求断言保留效果检查；受控router按canonical私有catalog识别无format阶段，不在生产解析正文。

最终主线程全Go race1662 PASS、514 SKIP、0 FAIL（含子用例），vet/build、gofmt及scoped diff通过。独立只读审阅无具体code/spec finding。DB和其它opt-in验证仍SKIP；首个沙箱全量跑因httptest监听被禁中断，随后相同测试在允许监听环境exit0。证据：/tmp/tool-final-schema-red.log、/tmp/tool-final-schema-green.log、/tmp/tool-final-schema-full-race-unsandboxed.jsonl及.stderr。真实混合请求拒绝已复现；本地修复的完整正式业务验收仍由用户升级Core/Worker后执行。

## 验收边界与安全边界

全部通过仅证明当前配置的有界provider前提。未执行真实私聊发布、目标完成事务、图片生成、全量业务prompt、长上下文或正式数据验收；单次矩阵不能给出模型的长期成功率。vision=false也没有通过本次文本工具测试被改变。

曾拟直接比较LM Studio内部llama.cpp后端，但该方案需要从进程参数读取内部API密钥，自动审批拒绝，操作没有执行；未读取凭据或绕过认证。无认证的内部props请求为401。最终通过用户关闭模式后的公开API完成对照。

## 公开来源

- https://lmstudio.ai/docs/developer/openai-compat/tools
- https://lmstudio.ai/changelog/lmstudio
- https://huggingface.co/coder3101/Cydonia-24B-v4.3-heretic-v2/blob/main/chat_template.jinja
- https://huggingface.co/TheDrummer/Cydonia-24B-v4.3/blob/main/chat_template.jinja
- https://huggingface.co/mistralai/Mistral-Small-3.2-24B-Instruct-2506
- https://github.com/mistralai/mistral-common/blob/535b4d0a0fc94674ea17db6cf8dc2079b81cbcfa/src/mistral_common/tokens/tokenizers/instruct.py (V7 inherits JSON-array call encoding; V7 tool results use id + TOOL_CONTENT)
