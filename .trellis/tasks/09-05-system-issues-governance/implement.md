# 实施计划：系统问题治理（第一阶段）

## 开始前门禁

- [x] 用户确认 Redis 权威边界：推荐 Redis 只作低延迟调度/触发提示，PostgreSQL/Temporal 保留事实与恢复权威。
- [x] 读取 `trellis-before-dev`，按 backend/frontend 层加载本次会修改的 spec 指南。
- [x] 复核当前工作树只包含本任务规划文件和用户已有改动。
- [x] 确认不会实现提示词构成重构；当前五项均不需要它作为前置。

## 有序工作项

### 1. 现状盘点（只读产出）

- [x] 整理 Temporal workflow/intent/queue/activity/producer/management 清单。
- [x] 整理 Googleapis CSS、字体声明和构建运行时证据。
- [x] 整理 media diagnostics、Provider queue、reflection/wakeup 的跨层调用链。
- [x] 将盘点结果写入任务 research/notes，作为后续实现基线。

验证：`rg` 全仓库交叉检查；必要时运行 `go test`/前端静态测试以确认现有基线。

### 2. Googleapis CSS

- [x] 根据盘点结果移除外部 import；仓库没有合法本地字体资产，因此未引入自托管文件。
- [x] 统一字体声明与 system fallback，不引入新远程依赖。
- [x] 添加前端静态测试，确保 CSS 入口不含 Google Fonts 运行时依赖。
- [x] 运行 `pnpm --filter @fluctlight/web test`：32/32 通过。
- [x] 运行 `pnpm --filter @fluctlight/web build`：Vite production build 通过。

验证：`npm run build`（apps/web）；本地静态 server 检查 network/computed style；运行相关 layout tests。

回滚点：仅 CSS/字体入口文件。

### 3. 诊断中心媒体提示词

- [x] 修正 Core model-run 语义 role 与 binding role 的持久化契约，并补充回归测试。
- [x] 为媒体 model run/ComfyUI submitted event 建立稳定关联和最近 20 条查询。
- [x] 更新 Core handler、BFF DTO、browser-client 类型、Pinia store 和 Diagnostics view。
- [x] 保持 redaction、owner authorization、retention、correlation filter 兼容。
- [x] 增加媒体提示词数量、关联、BFF 映射和敏感 data URL 脱敏路径测试；数据库排序由明确 SQL 顺序保证。

验证：Core targeted tests；BFF DTO/route tests；Web layout/store tests；必要时 Compose smoke。

回滚点：migration、Core diagnostics query/record、BFF DTO、Diagnostics view。

### 4. Redis Sorted Set LLM 调度

- [x] 设计并实现版本化 Redis key schema、score 编码和可恢复 job reference。
- [x] 实现原子 claim、processing lease（含续租）、ack、cancel、超时/requeue 和 stale-job 清理；PG polling fallback 保留。
- [x] 将现有 generated Provider queue 接入 Redis；embedding queue 保持独立。
- [x] 保留 PG diagnostic lifecycle、现有 priority mapping、context cancellation 和同步返回语义。
- [x] 增加 score/queue key 单元测试；Redis listener 集成测试在当前受限环境跳过本地监听，代码路径已通过全量 Go 编译测试。
- [x] Redis 故障自动回退现有 in-process queue；无需改变默认配置即可回滚。

验证：`go test ./apps/core-go/...` 中的 queue/provider/diagnostics/workflow targeted tests；Redis disposable Compose smoke；多 worker 并发检查。

回滚点：Redis queue adapter 与 Provider queue wiring；保留旧 queue 实现直到全量验证通过。

### 5. Redis TTL 反思/唤醒

- [x] 定义稳定 key、TTL 单位、配置读取和版本化 payload/reference。
- [x] 配置 `notify-keyspace-events Ex`，实现 listener 重连和 dispatcher nudge。
- [x] 在 user turn/reflection/wakeup 事务边界增加 due 状态与幂等检查；每个用户 turn 合并为一个延迟 reflection intent。
- [x] 保留/适配现有 Temporal WakeUpWorkflow、reflection evidence window、watermark、CAS 和 stable workflow ID。
- [x] 实现 Worker startup wake-up hint repair 与既有 PG due scan；listener 丢失/Redis 重启不阻塞 durable dispatcher。
- [x] 增加 key schema/score 单元覆盖；Redis/Compose 故障场景受当前沙箱无法监听本地端口限制，需在可用 Docker/Redis 环境补跑。

验证：Core workflow/reflection/wakeup targeted tests；Redis integration smoke；Compose worker restart/failure scenario。

回滚点：listener/TTL feature flag；关闭 Redis nudge 后 PG/Temporal dispatcher 仍可独立运行。

## 最终质量门禁

- [x] 运行 `trellis-check` 要求的 spec compliance、lint、type-check、tests、跨层数据流和一致性检查。
- [x] 前端 production build + 静态构建验证；关键页面静态测试通过。浏览器/Nginx 手工检查需在可用 UI 环境补跑。
- [x] 运行后端 targeted tests、全量 Go tests、BFF tests；必要的本地监听/Compose smoke 受当前沙箱限制并已记录。
- [x] 检查新增 Redis key/queue job 有 TTL、release 和 stale cleanup；PG/Temporal 记录保留。
- [x] 本次新增的 role/binding、media diagnostics、Redis queue/trigger 契约已同步到任务 design/research；未改变通用 spec，暂不追加规范文件。
- [ ] 用户确认第 4 项提示词重构的具体细节后，另建/另规划后续工作；本任务不提交该改动。
