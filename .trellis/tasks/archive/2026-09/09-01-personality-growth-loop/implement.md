# 执行计划：完整人格成长闭环

## 阶段 1：统一事实与数据模型

- [x] 盘点现有 `cognition_inbox`、`cognition_assessments`、`cognition_decision_proposals`、`cognition_frozen_actions`、`cognition_wakeups`、`fluctlight_inner_states` 和 `fluctlight_state_revisions` 的 producer/consumer。
- [x] 设计并添加新的 migration head，补齐 appraisal、internal dynamics、focus cycle、action result、drive revision、preference revision 和 future trigger preference 的权威表/字段、索引与幂等约束。
- [x] 设计可扩展的 Drive/Preference slot registry：稳定 key、kind、value_schema、生命周期、decay/update policy、provenance 和 supersede 链；新增 slot 不需要重新发布代码。
- [x] 定义 Go wire envelopes 和 normalize/validate 函数，保证未知 schema、未知 candidate、foreign evidence、越界数值和重复 effect fail closed。

## 阶段 2：Experience → Appraisal → Internal Dynamics

- [x] 把 conversation、life event、wake-up 和 action result 统一为带 source/correlation/evidence 的 cognition fact。
- [x] 扩展 `cognitive_assessment` structured response，保存 Appraisal、Attention、Thought、Desire、Agency 的 bounded summaries 和 action proposal。
- [x] 实现 Core-owned internal dynamics reducer：应用时间衰减和已验证 appraisal/result，产生 PAD、mood、momentum、regulation、任意 Drive slots、conflicts 的 requested/applied delta 与 revision。
- [x] 将 reducer 与 chat/wake-up 的 caller transaction 连接；Provider/实现失败时不能写入默认状态。

## 阶段 3：开放 Agency → Action → Result

- [x] 让 Agency 使用 Capability manifest 选择任意已安装能力或 no-op，不在 wake-up 逻辑中写死产品 action 类型；未知能力转为 `capability.request` tool call，而不是失败成一段自由文本。
- [x] 复用/扩展 frozen action、Temporal intent、lease、cancel、retry 和 governance，保持 stable action/provider/workflow IDs。
- [x] 新增 action-result fact/ledger，记录输出、失败、取消、预期偏差及 Owner/关系反馈，并触发后续 reflection。
- [x] 为重复 settlement、Worker 重启、过期 lease、Provider ambiguous result 增加回归测试。
- [x] 在 `CapabilityExecutor`/`CapabilityManifest` registry 中增加 `capability.request` native slot，严格校验需求参数并写入全局 capability request pool。
- [x] 增加 owner-only capability request 查询和审核 API/UI，支持 proposed/reviewing/accepted/rejected/fulfilled/cancelled，以及重复需求聚合和人工插件版本回写。

## 阶段 4：Reflection → Self Model → Drives / Preferences

- [x] 扩展 Reflection proposal normalize/filter/validate/apply，支持任意 drive-slot recalibration、preference-slot revision、attention priority 和 future trigger preference；新 slot 需经过 schema/范围/生命周期校验。
- [x] 实现 evidence window + watermark + state revision CAS；每类慢变量写不可变 revision、before/after、evidence refs、confidence、policy/model/prompt version。
- [x] 让 `BuildContextProjection` 读取新的 drive/preference/trigger slot projections，并在下一次 wake-up、对话 assessment 和 agency 中生效；未知 slot 以声明式数据传递，不能被错误映射为已有 drive。
- [x] 明确重复 claim/no-new-evidence 的 deterministic no-op，以及候选冲突时的保守合并/拒绝策略。

## 阶段 5：Wake-up、Web、诊断与兼容

- [x] 调整 `WakeUpWorkflow` 让它调用统一阶段流水线，保留 durable timer、Continue-As-New、cadence 设置和 disabled/inactive 语义。
- [x] 扩展 Core detail、诊断、Web 治理页面和运行策略，展示阶段摘要、成长 revision、drive/preference 变化和 action/result 状态；不泄露 hidden reasoning 或 credentials。
- [x] 更新 Provider smoke fixture、README、backend/frontend code-spec 和必要的 generated contract artifact。
- [x] 验证旧 `daily_review.current_day`、旧 autonomy actions、旧 wake-up rows 和 active workflow history 的兼容读取/回滚路径。
- [x] 验证缺失能力只生成需求池记录，不生成伪造 Action；手工注册插件并 preflight 后，需求可标记 fulfilled 并出现在下次 capability catalog。

## 验证命令

```bash
gofmt -l apps/core-go apps/gateway-go
GOCACHE="$PWD/.gocache" go -C apps/core-go test -race ./...
GOCACHE="$PWD/.gocache" go -C apps/core-go vet ./...
GOCACHE="$PWD/.gocache" go -C apps/core-go build ./...
GOCACHE="$PWD/.gocache-gateway" go -C apps/gateway-go test ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.env ./infra/compose/run-platform-smoke.sh
```

## 风险与回滚点

- **迁移风险**：新表必须 additive；迁移失败时启动失败，不删除旧事实。
- **Provider schema 风险**：先完成 normalize/validate 和 fake-provider 测试，再开放新的 candidate 类型。
- **动作风险**：先保留 action freeze/lease/idempotency，再把能力 manifest 扩展为开放选择；任何未安装或未 preflight 的能力仍不可执行。
- **并发风险**：先锁定每个 Fluctlight 的 cognition writer 和 revision CAS，再启用 action-result re-entry。
- **Workflow 风险**：先增加 replay/Continue-As-New 测试；新 Worker 无法重放旧 history 时保持旧 build，不强制 cutover。
- **回滚点**：可独立停止新 wake-up dispatch、回退 Worker build、禁用新 candidate 应用；已提交事实和 revision 保留为审计数据。

## 开始实现前的审阅项

- [x] 用户确认“开放 Action 空间”意味着不增加产品类型白名单，但接受能力 manifest、授权、硬安全、资源和幂等这些技术不变量。
- [x] 用户确认任意 Drive/Preference slot 的值模型：Drive 使用 bounded pressure，Preference 使用声明式 typed value schema。
- [x] PRD、设计、执行计划中的字段名和验收标准已统一。
