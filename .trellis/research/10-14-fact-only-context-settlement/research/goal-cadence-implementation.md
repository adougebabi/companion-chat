# F15 implementation record

基线 ec1ed82。本记录只覆盖 Goal cadence；没有修改 Goal submit wire、完成/证据判断、Tool 权限、模型配置或 workflow history 命令序列。

## Source changes

- 新增 `internal/core/goal_agent_cadence.go`：Evaluation 原子周期准入、最后 Planner native Agent 准入、Evaluation durable due repair。
- `goal_evaluation_runtime.go`：Life/request lock 内使用 DB 时间；pending/retry/frozen 都先通过 available_at 与实例级下一周期；defer 不增加 attempt，不处理 source；失败 available_at 至少等到下一周期。原冻结 snapshot、claim revision 和 accepted journal 不重建。
- `goal_sources.go`：队列时间取原 merge 与 persisted cadence 的较晚值，workflow intent 复用请求时间；nil goal IDs 规范为空数组；沿原锁顺序。
- `goal_planner.go`：Owner/review/full suggestions 例外删除，满额统一返回可解释的无模型结果并保留事件；旧队列 claim 与最终 native Agent 都检查。Agent 的 persisted processing run、claim revision、actor/profile/mode 是准入依据，模型文字不能制造 Owner 事件。成功 commit 恢复仍先于 fence。
- Planner automatic 日/逐事件预算在最后 native Agent 调用前用 Life 锁写账，失败同日/同完成事件不再重获模型预算。Actor/profile late events 只有遇到尚未花费的日/完成 episode 才排新 run。Repair 使用现有 lifecycle ticker 生产稳定当地日期事件，重查空位/逐事件预算，不重置旧 failed run。
- `cmd/worker/main.go`：既有 repair tick 接入 Evaluation due repair，复用 PostgreSQL intent 和原 Temporal deferred/not_before。
- 新增 `internal/migrations/goal_agent_cadence.go`，head `0063_goal_agent_cadence`，previous `0062_reply_media`。Runner 在所有 released installer 后重装；旧 SQL 常量未修改。

## Durable data

`goal_agent_cadence(fluctlight_id PK/FK, evaluation_next_at, evaluation_request_id, updated_at)`：DB 时间下首次准入设置下一次为 now()+30min，同实例/profile/Worker 共用。新旧 producer 即使直接入队也必须在实际 claim 过 fence。

`goal_planner_admissions(fluctlight_id, episode_key PK, run_ref, admitted_at)`：episode_key 为当前 IANA 时区本地 `daily:YYYY-MM-DD`，或某个真实 Owner/正式 completed event 的精确 `event:seq`。不用全局水位代表已花预算，private profile 低序号事件不会被别的 profile 消耗。一次 Agent episode 合并当前可见未花费预算；失败也保留账。

迁移只回填 operational clock/admission：从旧实际 attempt 的记录保守继承最近 Evaluation 时刻和历史 Planner 日/可见具体事件预算。无 Goal/evidence/source/history rewrite/delete；head rerun 不重置 clock。

## RED and verification

- 先添加 `TestGoalAgentCadenceHead`，旧实现实测 RED：head=0062_reply_media，期望0063。实现后 GREEN。
- runtime PostgreSQL 新场景无法执行 RED/GREEN：本机没有 PG、Docker daemon 不可用，GO_CORE_TEST_DATABASE_URL 空。新增的持久化场景只有编译和 SKIP 证据，不能写成 runtime GREEN。
- 最终定向命令：`FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -race ./internal/core ./internal/migrations -run 'Test(Goal|PostgresGoal)' -count=1 -json`，退出0。含子测试计数：133 PASS，89 SKIP，0 FAIL。原始输出 `/tmp/goal-cadence-targeted-tests.json`。因 httptest loopback listener 需 sandbox 外权限，首次 sandbox 内尝试失败后已在批准范围外重跑通过。
- `go vet ./internal/core ./internal/migrations ./cmd/worker` 通过。
- `go build ./cmd/worker` 通过；生成的本地 worker 二进制已移除。
- gofmt 已应用全部本轮改动 Go 文件；`git diff --check` 通过。

新增 PG 用例：Evaluation pending/frozen/profile/30min boundary/attempt、full 全 source 与 old queue、失败同日/private 精确预算、无新事件 periodic repair、fresh/0062 upgrade/head rerun。现有测试仅按新时序显式推进存储时刻，更新已被用户覆盖的满额 suggestions 断言；未关闭生产 cadence，也未增加 global skip。

## Acceptance limits / handoff

需要真实 disposable PostgreSQL 执行以上5个新 PG 场景及全部原 Goal persistence tests，才能验证函数 SQL、并发原子性、旧数据升级、native loop partial journal 与所有历史测试断言。当前测试通过包含大量旧 PG SKIP，不能代表这些场景已实际验收。独立 checker 及主代理最终全量质量门禁尚未由本子代理执行。真实 Provider/Temporal/Redis 和线上业务均 NOT_RUN。没有 git add/commit/push/部署。
