# 摇光第六阶段：执行与验证方案 (Implementation Plan)

## Ordered Implementation Checklist

### Step 1: Group 1 - AI 基础模块物理拆解
1. [ ] 创建目录：
   - `internal/ai/model`
   - `internal/ai/prompt`
   - `internal/ai/agent`
   - `internal/ai/task`
2. [ ] 迁移并重构 `internal/ai/model`：
   - 迁入 `eino_model_runtime.go`, `provider_queue.go`, `provider_redis_queue.go`, `provider_runtime_support.go`, `crypto.go`
   - 提取 `provider.go` 的基础 ProviderClient，抽象出 `ProviderDatabase` 最小数据访问接口
   - 迁移对应单元测试：`eino_adk_runtime_test.go`(模型测试部分), `provider_queue_test.go`, `provider_redis_queue_test.go`, `provider_stream_lifecycle_test.go`
   - 验证：`go test ./internal/ai/model/...`
3. [ ] 迁移并重构 `internal/ai/prompt`：
   - 迁入 `provider_prompt_composer.go`, `provider_prompt_format.go`, `provider_prompts.go`, `prompt_slots.go`, `typed_slots.go`
   - 迁移对应单元测试：`provider_prompt_composer_test.go`, `provider_prompt_format_test.go`, `provider_prompts_test.go`
   - 验证：`go test ./internal/ai/prompt/...`
4. [ ] 迁移并重构 `internal/ai/agent`：
   - 迁入 `adk_conversation_runtime.go` (ADK Loop 逻辑)
   - 迁移对应单元测试：`eino_adk_runtime_test.go`(ADK 循环部分)
   - 验证：`go test ./internal/ai/agent/...`
5. [ ] 迁移并重构 `internal/ai/task`：
   - 迁入 `model_tasks.go`, `provider_schemas.go`, `provider_shape.go`, `provider_language.go`, `initialization_fidelity.go`
   - 迁移对应单元测试：`provider_schemas_test.go`, `initialization_fidelity_test.go`
   - 验证：`go test ./internal/ai/task/...`
6. [ ] 验证 AI 模块内部与整体项目编译及回归：
   - `go test ./internal/ai/...`
   - `go test ./...`

### Step 2: Group 2 - Capability 基础模块拆解
1. [ ] 创建目录：`internal/capability`
2. [ ] 迁入 Capability 基础设施：
   - `capability_core.go`
   - `capability_runtime.go`
   - `capability_prompt_policy.go`
   - `capability_requests.go`
   - `tool_contract.go`
3. [ ] 迁移对应单元测试：
   - `capability_core_test.go`
   - `capability_codec_fixture_test.go`
   - `capability_payload_migration_test.go`
   - `capability_payload_migration_fixture_test.go`
   - `capability_transaction_integration_test.go`
   - `tool_contract_test.go`
4. [ ] 在 `core.App` 中适配 `capability` 包引用，保证现有 `builtinCapabilities` 无缝接入。
5. [ ] 验证：
   - `go test ./internal/capability/...`
   - `go test ./...`

### Step 3: Group 3 - Conversation 领域模块拆解
1. [ ] 创建目录：`internal/conversation`
2. [ ] 迁入会话相关代码：
   - `conversation_runtime.go`
   - `turn_decision.go`
   - `turn_takeover.go`
   - `query_continuation.go`
   - `takeover_scope.go`
   - `visible_output.go`
   - `raw_history.go`
   - `conversation_summary.go`
3. [ ] 迁移对应测试文件：
   - `conversation_runtime_test.go`, `query_continuation_test.go`, `raw_history_test.go`
   - `turn_chain_budget_test.go`, `turn_path_cost_report_test.go`, `turn_takeover_test.go`
   - `turn_takeover_chain_test.go`, `turn_takeover_f01~f05_test.go`, `turn_takeover_recovery_test.go`
   - `takeover_scope_matrix_test.go`, `visible_output_test.go`, `conversation_summary_test.go`
4. [ ] 保持 `mutations.go` 中的 `HandleTurn` 调用语义不变。
5. [ ] 验证：
   - `go test ./internal/conversation/...`
   - `go test ./...`

### Step 4: Group 4 - Cognition 领域模块拆解
1. [ ] 创建目录：
   - `internal/cognition/wakeup`
   - `internal/cognition/reflection`
   - `internal/cognition/autonomy`
2. [ ] 迁入 WakeUp 与 Reflection 相关代码及测试。
3. [ ] 验证：
   - `go test ./internal/cognition/...`
   - `go test ./...`

### Step 5: Group 5 - 领域业务与模型拆解
1. [ ] 创建并迁移 `internal/personality`
2. [ ] 创建并迁移 `internal/memory`
3. [ ] 创建并迁移 `internal/schedule`, `internal/media`, `internal/visualidentity`
4. [ ] 验证所有单模块及全局测试。

### Step 6: 最终验收与依赖分析
1. [ ] 运行静态分析工具或脚本，验证无循环依赖。
2. [ ] 运行全量测试基线：`go test ./...`。
3. [ ] 编制重构前后对比总结与阶段七候选债务清单。

## Validation Commands
```bash
cd apps/core-go
go vet ./...
go test -v ./...
go run cmd/api/main.go --help  # 确保主程序入口编译正常
```
