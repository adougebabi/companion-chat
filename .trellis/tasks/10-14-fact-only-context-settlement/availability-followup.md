# 2026-10-14 可用性回归追加要求

## 用户明确决策

用户报告 ProcessCognitionActivity 在 2026-10-14 03:50:30 UTC（11:50:30 +08）再次失败，错误 context_reference_index_too_large；明确表示可接受超过优化阈值后继续使用、后续再优化，不接受任意容量门槛一刀切阻断。

本追加需求直接修改当前任务的容量处理策略，继续在 codex/fact-only-reply-vision 上实施，无新任务/部署授权。此前四笔工作提交已完成，当前 HEAD 26bfc58。

## F14：容量阈值只告警

- 内部 ContextReferenceIndex 的数量、聚合字节和单快照字节阈值改为 advisory；超限合法数据保留完整索引与快照继续运行，不能只调大固定值、静默丢 refs 或裁数据。
- 重复注册已有引用幂等，不因数量达到阈值而失败。
- 同一条内部冻结上下文/Tool传递路径上的等价容量拒绝和静默 nil 截断也要消除，防止下一层继续失败；模型自填参数/外部请求与内部权威上下文分别处理。
- 保留所有权、作用域、内容/版本/hash、合法JSON、实际引用成员和必要业务准入检查，不能以可用性为名伪造事实或授权。
- 在模型前的状态装配层产生可解释诊断：实际条目/总字节、分类型数量/字节、最大快照、超出的建议阈值及阶段/关联身份；仅元数据，不打印正文、凭据或原始快照。即使诊断存储不可用也有安全的可见告警，诊断失败不改变业务结果。
- 定向测试覆盖过往所有容量失败路径、重复注册、内部传递/重放，以及篡改/越权仍拒绝；不改真实业务数据、不实际发布测试消息。

## 先验反馈回路

命令：FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/core -run '^TestContextReference(CapacityIsAdvisory|DuplicateAtAdvisoryThresholdIsIdempotent)$' -count=1 -v

已在旧实现 RED：reference_count 与 aggregate_bytes 返回 context_reference_index_too_large；single_snapshot_bytes 返回 context_reference_snapshot_invalid；duplicate_registration_at_threshold 返回 context_reference_index_too_large。全部为纯本地实际 builder/validator，未请求模型/数据库。

## 实施结果

引用三类容量门槛改为advisory；三处Active Memory容量裁剪移除；Core/canonical snapshot/resolver/prepared codec、Tool冻结copy、resulting-state、ActionOutcome引用传播贯通。独立核验补获并修复24项模型影响+1项实际状态导致结算失败，新增回归先RED再GREEN，保留模型24项合同。

事件context.reference.capacity_warning在模型前记录worker安全元数据，随后尝试Owner存储；无DB时仍可见，不打印快照正文。告警包含stage、decision、as_of、关联身份、数量/编码总字节/快照总字节/最大快照/分类型/前5条描述与建议阈值。沿用既有2秒best-effort同步写，不能宣称零延迟。

最终八项顶层容量回归、完整Go test/race/vet/build及pnpm typecheck/test/build通过，原条件化SKIP保留。结果见verification/capacity-check-summary.json；交付说明见docs/context-capacity-availability.md；独立核验见research/capacity-review.md。

当前追加修复未提交，待阶段3.4一次确认。生产日志只证实原前置失败，实际命中哪条容量不能从旧错误区分；未连接生产、部署或重试该workflow，不宣称已恢复。
