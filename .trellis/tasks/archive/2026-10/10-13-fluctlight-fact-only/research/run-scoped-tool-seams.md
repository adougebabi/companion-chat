# Run-scoped tool seams（代码核验，2026-10-13）

范围：core/capability 代码及相关测试；未读 README/CONTEXT/spec/架构/design，未运行模型或回归，未修改产品代码。当前检索未发现 `app_capabilities.go`，实际 composition root 为 `core/app.go` + `builtin_capabilities.go`。

## 核心事实与可用 seam

- **Core 有自己的 registry/runtime，不能把 internal/capability 版本当作生产唯一实现。** `apps/core-go/internal/core/tool_contract.go:34` 定义具体 `CapabilityRegistry`；`core/capability_core.go:1228` 定义自己的 `CapabilityRuntime`。`internal/capability/registry.go:12` 与 `capability.go:1320` 是另一套。Core 的部分类型 alias capability，但 registry/runtime 不是 alias。
- `core/app.go:151-158`：`NewCapabilityRegistry(builtinCapabilities(app)...)` → `app.Capabilities` → `NewCapabilityRuntime(app.Capabilities, app.ContextResolver)`。`core/capability_runtime.go:11` 的 `capabilityRegistry()` 无 ctx；有 App.Capabilities 就返回该共享指针，否则重建 builtin。`capabilityRuntime():19` 缓存 App.Runtime。不能为并发 run 临时改 App.Capabilities/Runtime。
- `core/tool_contract.go:55-83` 注册时同时缓存 `entry.Definition()` 和 entry，检查 RequiredContext 一致，重复名错误：`if _, exists := registry.capabilities[definition.Name]; exists { return fmt.Errorf("capability %q already registered", definition.Name) }`。没有 replace/clone/view API，没有 registry reader interface。`Lookup:119` / `Definition:139` / `Catalog:147` 全无 ctx。注册后再改变实例 Definition 会与缓存 definition 分裂。
- `core/capability_core.go:1142`：`Capability` 只有 `Definition()`, `RequiredContext()`, `Execute(context.Context, CapabilityInvocation, CapabilityContext)`。Prepare、Preflight、CandidateValidator 都是 capability-owned 可选接口，带 ctx。`DirectToolCapability`（core/tool_execution.go 中接口引用）为 direct Tx publication seam。此组接口可以保持 generic execution 无 capability-name 分支。
- **native 无 ctx 限制是实在的。** `core/native_tool_contract.go:16-24` 原文：`NativeDefinition() CapabilityDefinition` / `BindNativeArguments(json.RawMessage, ContextProjection)` / `NativeOutput(any, ContextProjection)`；`nativeDefinitionProvider.NativeDefinitions([]CapabilityDefinition)` 也不带 ctx。`appADKCapabilityInvoker.NativeDefinitions:26` 无 ctx，只能读取 invoker receiver 已持有的数据；当前 `:29` 每次走 `i.app.capabilityRegistry().Lookup(definition.Name)`。因此 ctx 可以携带请求级 view，但无 ctx 方法须借由 invoker receiver 或预先固定的 capability 实例拿到 view；不能在 NativeDefinition 内凭空读取当前 ctx。仅覆写 wire schema 不会同步 direct canonical validation。
- `core/adk_conversation_runtime.go:66-79` `ADKCapabilityRequest` 当前为 identity + Projection，无 registry/view。`newAppADKCapabilityInvoker:82` 拷贝 request 到 invoker；这个 receiver 生命周期本来是 request scoped。`withADKCapabilityContextRefs:40` 已使用 `context.WithValue(... adkConversationContext{Invoker, Trace, Refresh, Refs})`；`nativeDefinitionsForContext`（native_tool_contract.go:36）从 ctx 提取 invoker，再调无 ctx `NativeDefinitions`。**ctx 携请求级 registry/execution view 技术上可用，当前没有现成 key/读取函数或 view 接口；必须让所有会重新读取 App registry 的入口消费同一 view。**
- `core/adk_conversation_runtime.go:569` canonical definitions validation 在 invoker/context 装载之前；`:577-581` 构造 request/invoker，`:599` 才把 ADK state 放 ctx。若 view 依赖 ADK state，早期 validation 看不到，需注意安装顺序。
- `core/tool_execution.go:151-165` 每次 direct 执行组装独立 resolver/runtime，注释明确 `Do not mutate App.Runtime while concurrent direct callers are resolving their scope.` 这是可消费请求级 registry 的执行 seam，但当前仍把 `a.capabilityRegistry()` 传进去。`runtime.Prepare` 先 canonical schema validate 后 preflight/Prepare，因此 legacy canonical text schema 不会允许 refs 输入仅靠 Prepare 转译。
- `core/capability_core.go:1368-1400` Prepare：definition 走 `.Definition`，preflight/preparer 却直接访问 `runtime.Registry.capabilities[name]`。改 Lookup 的 overlay 会被这两处绕过；若改成 reader interface 也必须修此直接 map 依赖。internal/capability 对应 `capability.go:1475,1480` 同风险。
- `core/builtin_capabilities.go:54,148,235-245` 当前只有一个 `conversationReplyCapability{publication}`，Definition 固定 legacy，RequiredContext CurrentLife，Execute 返回 execute_tool_required，ExecuteDirectTx 读取 args[text] 并调用 publication。`core/tool_contract.go:196-206` canonical required=[text]、additionalProperties=false；若 fact_only refs 也要求 direct 同合同，native-only adapter 不足。

## 需同步/核对的读取点

以下列出完整 rg 得到的生产调用点类别；是否改为 run view 由主代理决定，特别是历史 settlement 是否仅依赖不变 metadata。

**必须保持定义/实现一致的执行链**

- `adk_conversation_runtime.go:107` Definition；`:184` implementation/native contract；`:201` NativeDefinition validation；`:267` ExecuteTool(ctx)；`:361` NativeOutput lookup；`:637,647` validateADKCapabilityDefinitions canonical comparison（reflect.DeepEqual）。
- `native_tool_contract.go:29` NativeDefinitions implementation lookup；`:36-45` ctx bridge/wire schema。
- `tool_execution.go:106-107` authorizer lookup（在 DB authorization 前）；`:165` runtime construction；`:172` Definition；`:210` implementation/classification；`:237` Prepare。`tool_authorization.go:20` autonomy policy Definition；`goal_execution_admission.go:58` admission Definition。
- `capability_core.go:1249,1253,1282,1333` Execute definition/implementation/required context；`:1372,1395,1400` Prepare definition/direct-map hooks；`:1433` ExecuteMany failure policy；`:1450,1454` ExecuteTransactional definition/implementation。internal/capability counterpart `capability.go:1345,1349,1378,1452,1475,1480,1513,1530,1534`。
- `capability_runtime.go:97,110,174` candidate validation + hook；`:236,241` binding/snapshot RequiredContext；`:382` reply output definition validation。`persona_action_capabilities.go:86` runtime registry Definition。

**catalog、discovery、run initialization**

- `capability_runtime.go:38-42` capabilityCatalog。
- `builtin_capabilities.go:141-146` discovery/catalog closures 捕获 app，签名 `func(CapabilitySurface)` 无 ctx；不会自动随 context registry 改变。
- `kev_tools.go:64,93` catalog/discover → nativeDefinitionsForContext(ctx, authorizedDiscoveryDefinitions(...))；`:134-142` 若有 selection 使用 frozen selection.original，否则用无 ctx closure；`:156,159,168,221` App registry utilities；`:239,254` utility definition lookup。注意 selection.original 也必须从同一 run view 冻结，不能仅改变后续 wire。
- `cognition_agent.go:83` conversation catalog；`model_tasks.go:495,522` native cognition/autonomy catalog；`agent_result_adapter.go:867` wakeup catalog。
- `goal_planner.go:278` goal planner catalog；`goal_evaluation_task.go:231,243` goal evaluation prompt/run catalog；`visual_identity_agent.go:133` registry local binding。
- `intelligence.go:403` exposed Definitions；是否为全局安装目录或目标资源合同需主代理判断。
- `conversation_runtime.go:59-69,146-163` ConversationCapabilityContext → ADKCapabilityRequest 是 narrow run input 链，目前无 view。

**其余 metadata/settlement 读点（全部列出，避免漏掉 global fallback）**

- `capability_requests.go:224` installed request lookup。
- `turn_takeover.go:74,368` candidate preview definitions。
- `composite_actions.go:148,182,192` validation/deferred output metadata。
- `action_outcome.go:152,217,469` outcome definitions/build；`life_activity_capabilities.go:305` build outcomes。
- `workflow_ops.go:197,323,426,428,498` required failures/outcomes。
- `operations.go:699,701` outcomes。
- `cognition.go:317,592,726,995,1080,1082,1134` committed-mutation test / required context / outcomes / partial mutation definitions。
- `intelligence.go:942` deferred media metadata。
- `goal_execution_admission.go:224,230,239,256,379,381` settlement/outcomes。
- `agent_result_adapter.go:625,673,1010,1237,1269,1427` validation/outcomes/settlement。

## 重要测试与风险

- `capability_core_test.go:604` TestCapabilityRegistryRejectsNilDuplicateInvalidAndContextMismatch；`:227` TestBuiltInCapabilityExecutionClassesUseGenericMetadataAndInterfaces；`:431` dummy full chain；`:540` frozen Prepare；`:836` TestGenericAgentAndToolExecutionDoNotDispatchBusinessNames（检查 generic 三文件不能出现具体 CapabilityName literal 比较/switch）。
- `conversation_runtime_test.go:29` schema divergence before provider I/O；`:53` canonical outside default catalog。
- `eino_adk_runtime_test.go:667` TestADKCapabilityDefinitionsValidateRegistrationWithoutExecutionStageGate；`:714` physical provider identity；`:738` production single reply；`:512` streaming same native loop。
- `native_tool_contract_test.go:22` native goal preserves canonical API；`:156` physical schemas/reference repair；`:257` invalid native arguments traced + changed call ID rejection。用于观察 native/direct 分离的既有合同，fact_only 需另证 direct/native同合同。
- `kev_test.go:302` run scoped catalog purposes/parameters；`:412` discovery cannot widen/partial load；`:533` hidden tool discovered/executed；`:600` independent pure query boundary。
- `tool_publication_test.go:52,108` reply publish/replay/conflict + unowned target；`:260` control values never publish。`tool_execution_test.go:195` repeated call keeps projection；`:142,158` correctable argument errors vs dependency failures。

风险：共享 map 变更导致并发 run 合同漂移；缓存 definition 与 capability methods 不一致；prepare direct-map 绕过 lookup overlay；Kev.original 或 captured catalog closure 仍返回 global schema；early reflect.DeepEqual 与后置 ctx view 不一致；direct runtime validation 先于能力 Prepare 导致 refs 被 legacy拒绝；receipt replay 位于 Prepare 之前，若 mode 不参与持久 operation identity/authority，跨配置重试合同如何解释需核验。当前 run refresh 可以更新 Projection（adk_conversation_runtime.go:99），不要把随 refresh 漂移的 projection 当作 run 固定合同配置。
