# 实施顺序与验证门禁

## 当前状态与任务树
planning；已获创建任务授权，未运行 task.py start、未修改产品源码。按 Trellis brainstorm 要求，最新计划需下一条用户回复批准后才激活首个子任务。主代理负责代码、方案取舍与最终验证；按用户 AGENTS，子代理仅探索/检索/核验，不写产品代码。

| 批次 | 子任务 | 责任与依赖 |
| --- | --- | --- |
| A | 10-07-goal-execution-correctness | G00–G03；首次 Attempt/标准迁移与 A/B 标准回归 |
| B | 10-07-goal-evidence-progression | A 后 G04–G07；真实事件/统一评价/推进；同步 G11/G12 |
| C | 10-07-goal-review-owner-governance | B 后 G08–G10；Review/投影/Tool/API/UI；同步 G11/G12 |
| D | 10-07-goal-migration-joint-acceptance | A/B/C 后 G11–G13 综合迁移/存量对账/三 E2E/报告 |

父任务持有完整源需求、跨批次设计、验收矩阵与最终集成门禁。子任务完成不代表父任务完成。

## 执行清单
- [x] 创建任务、保存 source-requirements.md、确认 master/ee1e468/clean。
- [x] 初步三路 G00 代码定位并抽查关键缺口。
- [x] 复跑领域4子场景、Workflow5用例，保存 go test -json。
- [x] 编写 PRD/design/implement，策划全部 A/B/C/D 验收矩阵。
- [ ] 最新规划获得用户后续批准；校验 context manifest；task.py start A。
- [ ] A：读取 before-dev 对应 specs 与完整待改代码；故障复现；门禁/许可/Attempt认领和出口/分类退避；混合标准请求拒绝/稳定版本；增量迁移；暂停启动竞态与工具前失败隔离DB测试。
- [ ] B：Stage/Commitment 和父级校验；真实来源 journal/evidence 候选；消息/Outcome/事实提交+durable evaluation；统一 CAS 逐标准结算，移除购物/Reflection旧完成编排；event-triggered推进/Resolution/关系权威入口；脚本 Provider 三 E2E 基础。
- [ ] C：Review 先补评；持久周期/原因/停滞/期限；通用紧凑 projection 与各 Surface一致；Goal Tool注册/独立及native loop；Core/BFF/OpenAPI/client/UI 生命周期和历史；权限/注入拒绝与预算回归。
- [ ] D：真实存量夹具 upgrade/中断重跑/repair；Worker崩溃与启动恢复；全矩阵/三 E2E/真实模型/媒体分层；迁移操作说明与报告；核查旧生产分支不再调用。
- [ ] 每批 trellis-check、针对性测试及受影响契约，更新 spec/任务进度；用户授权范围内提交，不自动推送/部署。
- [ ] 最终G13六门禁逐项证据通过才完成；未通过保持 in_progress，准确列未验证链；finish-work journal/archive仅用于实际完成任务。

## 场景矩阵及结果记录
acceptance-matrix.tsv 完整保存 A01–A10/B01–B12/C01–C12/D01–D12 的场景/断言，当前全标 planned。每项实现时填真实测试名/日志/证据 ID/叶子 PASS/FAIL/SKIP/BLOCKED；计划测试名不能当实际运行证据。
E2E1 必须正式聊天提交→未预绑定Intention候选→证据→Evaluation→完成→WakeUp不再待表达；草稿/引用为反例。E2E2 必须双方确认，单方表达/沉默/拒绝不完成，关系权威与Resolution可查。E2E3 查询无物→计划→合法到期获取→实际物品ID入库→独立使用，取消/重试/失败/未入库不提前完成；请求快照不算真实媒体质量。

## 可重跑命令
仓库根：python3 .trellis/scripts/task.py validate <task>。
apps/core-go：go test -json ./internal/core -run '^TestGoalIntentionStageS07$' -count=1；随后针对新增用例并用真实隔离数据库，保留json日志。
完成每批：go test -json ./...；go test -race ./internal/core ./internal/workflow；go vet ./...；go build ./...。命令失败保存完整日志，叶子skip不当pass。
根：pnpm --filter @fluctlight/browser-client generate；pnpm --filter @fluctlight/browser-client typecheck；pnpm --filter @fluctlight/browser-client test；pnpm --filter @fluctlight/web lint；pnpm --filter @fluctlight/web typecheck；pnpm --filter @fluctlight/web test；pnpm --filter @fluctlight/web build；./infra/acceptance/check-core-openapi.sh。API生成、UI交互/刷新/CAS/mobile按变更核验。
隔离DB：一次性 pgvector/pgvector:pg16、随机localhost端口和任务凭据、tmpfs，自有容器自清理。只向该集群设置 GO_CORE_TEST_DATABASE_URL/CORE_GO_DATABASE_URL；Runner empty→head、0049→head、head rerun、失败事务回滚/组合FK/并发。Docker不可用先检查安全替代测试设施，确实不可用则写BLOCKED与完整启动重跑步骤，不连生产凑验收。
Worker联合需独立Temporal/Redis/PG测试设施和可控故障；真实Provider/媒体使用项目既有验收配置，不输出秘密；按三层脚本、实际模型、媒体逐一记录，未配置不造PASS。

## 风险与回滚点
Agent/tool adapter、Goal lifecycle和标准schema、migrations Runner、Reflection、publication、Worker、OpenAPI/client是高风险点。每步保留旧数据快照夹具与提交边界；禁止长事务等待Provider。未知副作用先核对，不整体重放failed Agent。数据库一旦写新数据优先前向修复，生产回滚需备份/停写，禁止无授权删除卷/库。

## 最终报告
docs/fluctlight-goal-closed-loop-implementation-report.md 包含前后基线、G00–G13状态/代码入口、完整触发到Review/Owner调用图、数据迁移/回填/恢复、命令/环境/叶子数量、故障证据、三E2E目标/版本/证据/评估ID、模型调用条件/完整请求预算对比、未验证项/重跑命令。既有CI validate-go/web禁用需明确报告，未经审查不把禁用流水线作证据。
