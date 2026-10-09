# Kev 主动接入交付报告

日期：2026-10-09。开发分支：`codex/kev-decision-integration`。基线：`63be0df`。

本轮完成七个决策点的代码接入、动态设置、独立能力发现、可靠逐题记录、诊断界面和本地验证。按用户最后确认“本地代码过了就行，我去正式环境验收”，正式环境迁移、真实 Kev 推理、运行效果及浏览器真实服务验收由用户执行；本报告不把受控模型/跳过测试当作真实模型通过。

## 交付与入口

| 决策点 | 实际入口与共用边界 |
| --- | --- |
| runtime.wakeup | ProcessWakeUp 硬资格/睡眠后、EnsureDirectConversation 前；原 WakeUp Agent 执行体共用；durable 延后保留 cycle；人工执行可打断延后 |
| runtime.reflection | ProcessReflection 非空真实证据后；no 释放 window lease、不推进 watermark；原 ReflectionProposalTask 共用 |
| goal.completion_check | Goal eligibility/memo 后逐 Goal gate；保留 deferred remainder 和 sources；原 GoalEvaluationTask/commit 共用；人工复核旁路 |
| goal.replenish_plan | Planner claim/capacity/policy 后；Owner 请求旁路；原 GoalPlannerAgent/commit 共用；独立 deferral 不耗执行重试 |
| tools.select | Prompt assembly 中形成 request-local visible set；原授权 executor catalog 保留；每次 Generate/Stream 的实际 schema 与预算对齐 |
| persona.switch | 原已安装 persona.switch 且有 refresh 的 Agent 中，完整 A 响应释放前 gate；原 ExecuteTool/persona domain 提交；整批 A 丢弃后 B 生成 |
| context.select | WorkingMemory resolver 前，仅 optional fragments；required facts、活动目标目录及协议历史保留；原预算/总结共用 |

`internal/ai/decision` 使用 `POST /v1/systemone`。合法 choice 默认直接采用；unclear/异常/预算耗尽回原路径，父请求取消不启动 fallback。旧模型不为 fallback 提前执行。配置撤销后仍保存原始 yes/no 和概率，将实际应用记录为 original/discarded。

新增独立 Tool `capability.discover`：当前 Agent 内只公开其原安装集合，先校验整次 load 再扩张；独立调用报告授权 catalog，不声称修改不存在的 Agent。补载后的 Tool schema 在下轮真实 HTTP 请求增长，保留 Eino v0.7.37 与原生 Runner。

人格接管的内层模型回调曾让丢弃 A 的调用进入运行结果。现已只对 gated proposal 的内层 callback 做隔离；外层原生 loop 收到 B，A 的原输出仍保留在 Owner 模型诊断中。Generate/Stream 回归均断言 A 不执行且不进入结果 ToolCalls。

## 设置和诊断操作

设置 → Kev 决策，可保存全局/七点开关、System One endpoint、超时、阶段预算、choice/guarded 策略、最大延后和保留天数。凭据只写入加密 `setting_secrets`，留空不覆盖已有值。显示已保存配置版本；动态修改不需要重新编译前端。

诊断中心 → Kev 决策，可过滤 Actor、Agent、决策点、策略、调用/应用状态和时间。每个 question 保留 request ID、raw response/answer、完整分布、策略、配置/状态版本及实际动作关联。列表按 `started_at DESC,id DESC` 排序，snapshot cursor 分页；显示选定时区。导出使用同一过滤和 snapshot，超过 10000 条时明确 `complete=false` 并保留 next_cursor。

手动“测试连接”只执行协议问题，不执行 Agent 或业务动作；即使全局关闭也能由 Owner 显式测试。正常关闭状态不发 Kev 健康探测或 shadow 请求。

关闭全局：取消“启用 Kev 判定”并保存。单点关闭：只取消对应点并保存。已经发送的请求/已提交事实不回滚、不重放。可选上下文与工具集由原来源恢复。

## 迁移与持久日志

Schema head：`0056_kev_decisions`，支持从 `0055_goal_planner_cadence` 增量升级，新增 `kev_decisions`、`kev_deferrals` 与 settings revision。迁移不覆盖已有 Owner 的 Kev 设置，也不启用未知生产地址。

DB 审计写入失败时，先落 bounded fsync journal，再采用结果；两种存储都失败则 original + 告警。Core/Worker 分别使用 `fluctlight_kev_core` / `fluctlight_kev_worker` named volumes。Core 每 30 秒维护自身 journal，Worker 复用已有维护 tick。跨进程文件锁、尾部半条写入隔离、按 recorded_at 幂等导入防止旧记录覆盖新应用结果；导入不重放业务动作。活动 journal 限制 32 MiB，最多另保留一份 bounded incomplete-tail 隔离文件。

## 正式环境启用

先使用包含本次代码的新 Core/Worker/Web 镜像，并按现有备份流程保护数据库。执行项目的显式 migrate 服务，确认 head 为 0056；再启用配置。不要把 Mac 的 loopback 当成 NAS/容器的 Kev 地址。

已有 Compose 私有环境配置时，可运行：

```sh
docker compose --env-file infra/compose/fluctlight.env -f infra/compose/fluctlight.compose.yml run --rm migrate

docker compose --env-file infra/compose/fluctlight.env -f infra/compose/fluctlight.compose.yml run --rm core /usr/local/bin/fluctlight-kev-config-go --endpoint http://MAC_LAN_ADDRESS:8010/v1/systemone
```

将 MAC_LAN_ADDRESS 替换为 Core/Worker 实际可达、受访问控制的地址。第二条是实际写入原 settings 服务的配置命令，会启用全局与七点，使用 choice_argmax；没有修改生产配置的自动启动脚本。配置凭据使用 Owner 设置入口。CLI 使用现有部署环境变量，不读取额外云密钥，不下载/升级权重。

同主机原生 Go 进程可以在对应环境变量就绪后运行：

```sh
go -C apps/core-go run ./cmd/kev-config --endpoint http://127.0.0.1:8010/v1/systemone
```

一键关闭也可在同一命令加 `--disable`；endpoint 仍需明确提供。命令已纳入 Core Docker 镜像构建。

正式验收建议依次检查：

1. 关闭全局，确认业务仍能运行、Kev HTTP 为零；逐项关闭只改变对应点。
2. 测试连接，核实真实 runtime/模型 revision；当前配置 revision 未知时记录 unknown。
3. 隔离测试 Actor 上覆盖自动运行、Tool 选择/发现、人格接管、上下文筛选四类 active 行为；从诊断中同时核对模型判断和真实动作。
4. 查看连续 no 的延后恢复、人工立即执行、目标证据/容量、开关中途关闭与上下文恢复。
5. 对隔离测试存储注入诊断故障，核实 spool/恢复和未留痕不采用；避免对真实用户发送消息、支付或发布动态。

项目现有独立 Tool / Agent 验收命令可按已有部署参数使用 `infra/acceptance/run-go-live-provider-smoke.sh --suite all`。它是既有 Provider/Tool 验收，不等同于七点真实 Kev 四场景验收。

## 本地验证证据

| 命令 | 结果 | exit |
| --- | --- | --- |
| go -C apps/core-go test -race -json ./... -count=1 | 1363 个 PASS、489 个 SKIP、0 FAIL（含子用例事件） | 0 |
| go -C apps/core-go vet ./... | 通过 | 0 |
| go -C apps/core-go build ./... | 全命令编译通过，含 kev-config | 0 |
| pnpm generate | Core/browser OpenAPI 与客户端同步 | 0 |
| pnpm typecheck | 三个工作区包通过 | 0 |
| pnpm test | Browser client 20 + Core client 2 + Web 72，共 94 PASS、0 FAIL | 0 |
| pnpm build | Web production build 通过 | 0 |
| git diff --check | 通过 | 0 |

Go SKIP 来自未配置隔离 PostgreSQL/真实 Provider 等 opt-in 环境，不能算验证通过。受控 HTTP/SSE 测试走项目锁定 OpenAI component、生产模型适配器和原生 Runner；它们验证工程分支/协议，不证明真实 Kev 的判断正确率。

前端 build 有 minified chunk 超过 500 kB 的提示，不影响构建退出码；本轮没有扩展为打包优化任务。

## 未执行与实测限制

- 未执行正式 PostgreSQL empty/0055→0056 升级、真实数据库业务分支、Redis/Temporal/Compose 联合环境验收。
- 未调用真实 Kev，也未核验 NAS/Core/Worker 到 Mac 的连通性、已测权重 revision 或真实模型业务效果。
- Downloads 定向搜索未发现完整 kev_fluctlight_results(1).json / benchmark；未使用完整 82 用例回放，也没有修改其已知误判 expected。
- 未在正式服务上进行浏览器/移动视口验收。
- 并发配置限制同一个 DecisionService 的在途调用，不宣称它是多个进程/实例的 Kev 服务总并发限额。

以上由用户按本轮验收约定在正式环境检查。当前交付结论是代码实现和本地检查通过；真实模型行为与业务效果仍待正式验收。
