# Kev 实施计划

## 阶段门与责任

本轮是同一集成交付，采用一个任务管理七个点和联合验收，不因工作流划分而提前宣布局部完成。规划 gate 后 task.py start。主线程负责设计取舍、生产代码修改与最终验证；子代理只承担独立只读探索/核验，遵守本仓 AGENTS.md 的 clean fork/单轮/等待规则。

## 有序执行清单

- [x] 保存来源，确认以完整实施文档为正式需求。
- [x] 三路只读核验真实入口，主线程读取权威架构/规范并点验关键代码。
- [x] Context7 官方文档 + 锁定依赖源码核验 Eino v0.7.37。
- [x] 隔离 native-loop/真实 OpenAI adapter fake HTTP 试验：wire schema 补载、整批 Tool 拦截。
- [x] 完成 PRD、design、实施计划和真实 spec/research manifests。
- [x] 提交完整规划摘要，用户批准后已执行 task.py start。

### I1 基线与框架高风险验证（先于大范围接入）

- [ ] 读取 trellis-before-dev 和本次目标层全部 relevant specs。
- [ ] 记录当前 commit/status、后端与浏览器基线；隔离数据库/服务能力清单。
- [ ] 为现有 queued model/loop 添 production seam 与行为测试：Generate/Stream wire 可见 schema、真实 discovery feedback、原 tool IDs/receipts、不共享状态。
- [ ] persona 整批 pre-admission→合法 switch→B 生成：A 未执行 batch 无任何副作用，流式不泄漏，no 能阻止后续覆盖；不能另建 loop。
- [ ] 若桥接无法满足原 loop/批次边界，停在该技术点修订设计；不得用静态 tool-name prompt 或 fake success 交付。

### I2 共享判定与记录

- [ ] 有界 System One client、版本化 specs、严格逐题 parser/策略、父取消。
- [ ] additive migration、settings revision/allowlist/secret、DecisionRepository。
- [ ] 可靠 raw-before-adopt、实际 application 更新、bounded fsync spool/volumes/幂等导入/容量告警。
- [ ] 同实例/进程 permit、选择总预算、熔断和精确 original fallback。
- [ ] disabled 无额外准备/HTTP；response 后 recheck config/state。

### I3 七点真实适配

- [ ] WakeUp preflight→gate→原 body；Reflection evidence→gate→原 proposal；各自 durable deferred workflow。
- [ ] Goal completion 逐 Goal 分支、exact subset/remainder/source 保留；Planner 自动 gate/manual bypass；deferral 不耗 attempt budget。
- [ ] 最大三次 no 后下一合格 original；not_before durable timer；完成提交后 latest state 规划。
- [ ] tools.select 与真实独立 discovery/load；authorized executor/visible schema/依赖状态同步；explicit load 不重否决。
- [ ] persona.switch 完整规则/优先级/no authority/CAS/once-per-run，与 I1 validated seam 连接。
- [ ] context.select 仅 optional；required/活动目标 catalog/承诺/ref pairing；关闭从源恢复。

### I4 管理和诊断

- [ ] Owner settings typed controls、生效 config revision、secret write-only、手动 protocol test。
- [ ] Kev list/detail/export DTO/filter/scoped SQL/stable cursor；Actor 过滤不能授予权限。
- [ ] Core/browser generators+generated clients，Web section/state epoch/raw详情/关联/时区。
- [ ] retention 七天动态配置+真实 bounded cleanup；spool 记录查询/导入限制说明。

### I5 验收、配置、报告

- [ ] T1–T10 逐项执行，覆盖 AC1–AC8，不仅 parser tests。
- [ ] 用户本地配置全启用实际通过 settings 机制应用/核验；地址从 Core/Worker 网络核验；不改远程生产。
- [ ] 完整 replay 文件若找到真实沿用；找不到仅固定 fixtures 明示限制；保留已知错例 expected。
- [ ] 真实 Kev 四场景 E2E 串行、隔离 Actor/存储、测试外部适配，服务不可达明确 NOT_RUN。
- [ ] 全后端+Web checks、迁移 empty/previous/head、原 workflow history 兼容、UI production/mobile+console。
- [ ] kev-integration-report.md：入口共用表、配置/关闭/endpoint/诊断操作、命令退出码和数量、mock/replay/live区分、未验证/失败归因。
- [ ] 独立只读核验与主线程最终验证；更新真实 spec 契约；按项目流程处理 commit/finish，不推送或部署未授权环境。

## 验收映射

| 文档测试 | 重点 | PRD |
| --- | --- | --- |
| T1 | global/per-point/off/unavailable，原请求/Tool/prompt/调用轨迹等价，Kev HTTP=0 | AC1 |
| T2 | 全协议矩阵、partial、低概率 choice、guarded、取消 | AC1 |
| T3 | 七点真实 yes/no/fallback/disabled 应用 | AC1–AC5 |
| T4 | 水位/cycle/retry不消耗，延后兜底，manual/forced/sleep/capacity/concurrency | AC2 |
| T5 | 原生 loop actual wire 增长/补载/真实结果，授权/多Actor/独立Tools/affected Agents | AC3 |
| T6 | required/catalog/承诺/ref pairing/budget/source不变/恢复 | AC5 |
| T7 | A batch无副作用、B、no、stale、失败、stream、once | AC4 |
| T8 | config disable races，DB/spool failure，cancel/circuit/budget，逐题导出/权限/order | AC6 |
| T9 | 固定协议回放与真实 Kev active 四场景；准确率和 adapter 正确性分开 | AC7–AC8 |
| T10 | 后端/集成/前端/迁移/浏览器回归和明确 baseline/new/env 区分 | AC8 |

## 验证命令

从仓库根执行，实际环境权限按测试需要处理：

```
go -C apps/core-go test ./internal/ai/agent ./internal/workflow -count=1
go -C apps/core-go test ./internal/core -run 'Test(Kev|GoalPlanner|GoalAssessment|GoalEvaluation|Persona|ProviderADK|RuntimeRecovery|ContextReference)' -count=1
go -C apps/core-go test -race ./... -count=1
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

Go真实数据库测试需项目既有隔离 fixture；无 DB 的 SKIP 不视 PASS。显式 migrate 只针对任务自建 disposable DB，empty→next head、0055→next head、head rerun/rollback 均测。可用时运行 infra/compose/run-platform-smoke.sh 与 infra/acceptance/run-go-live-provider-smoke.sh 相关 suites；不得复用含真实数据的 Compose volume 作为隔离环境。实施后补可直接运行的 Kev E2E 命令与参数，不虚构当前未实现 CLI。

## 风险/回滚点

I1 是 schema/人格关键风险点；I2 是可靠记录与 config race；I3 是 Temporal history/lease/cursor；I4 是 API/client parity/Owner隔离。每段完成后跑 owning tests，然后集成全量。失败先保持原路径；rollback 全局关闭，保留原 Tool receipts、审计和新增 schema。未发模型请求恢复原 catalog/context，已发请求和已提交副作用不撤回、不重放。

## 当前验收约定（用户 2026-10-09 更新）

以本地代码测试、类型检查、构建与静态质量通过为交付门禁。正式环境和真实 Kev E2E 用户自行执行；交付 migration、设置操作、验收命令与 NOT_RUN 清单。不要为本任务再安装/启动 Docker 或数据库。

## 本轮交付状态

七点实现、设置/诊断/配置命令已交付。两轮只读核验提出的取消、spool、权限/批次问题由主线程修复；新增原生 Generate/Stream 回归发现并修复丢弃提案的内层 callback 泄漏。最终本地 race/vet/build/generate/typecheck/test/build 均通过；证据与正式操作见 docs/kev-integration-report.md。正式数据库/模型/部署验收按用户要求留待其执行。产品代码尚未提交，等待 Phase 3.4 的一次提交确认；未运行 archive/finish，不改变其他任务。

## 正式反馈修复交付

App URL/computed 路由回归 4 RED→GREEN；无新证据 candidate 入队回归 RED→GREEN。0057 PostgreSQL 逻辑运行租约在 6 个入口快照前接入，withTransaction 提交 fence；重入、跨 owner、取消、过期释放回归通过。独立只读核验未发现确定的新 lease/deadlock/时序问题。Go race 1370 PASS/490 SKIP/0 FAIL，Web/client 98 PASS，类型/vet/build 通过。真实数据库验收仍由用户执行。当前分支 codex/kev-runtime-recovery，产品改动未提交。


## Goal Evaluation 完成未落库反馈修复（2026-10-09）

用户提供的 response 中 goal:2/3/4 的 review 均引用 stage:1.1；goal:2 在描述无现有阶段时还使用 adjust/object_ref。引用按目标编号绑定，旧全局 enum 会放行跨目标选择，hydration 随后拒绝整批，导致 goal:1 的 completed 也未提交。用户看到物理 response，没有领域提交错误展示；未查询正式数据库，不将推断错误码当作生产实测。

本轮 schema 改为按 Goal 判别并限制对象/标准所有权及 create/adjust 操作；wire/coverage 可纠正一次完整输出，替换仍校验；wire 与耗尽 final-contract 错误 terminal。诊断独立展示目标评估提交状态/error/result，成功结果包含实际 goal_outcomes。原批次原子性、CAS、证据与 paused settlement 规则保留；无新 migration，head 仍 0057_logical_agent_leases。

本地最终验证：Go race 1374 PASS、490 SKIP、0 FAIL（含子用例）；Go vet/build、pnpm generate/typecheck/test/build、git diff --check 通过；Web/client 共 98 PASS。独立只读核验无 findings。SKIP/真实数据库/真实模型/正式部署仍由用户验收。当前 codex/goal-evaluation-output-recovery 未提交。
