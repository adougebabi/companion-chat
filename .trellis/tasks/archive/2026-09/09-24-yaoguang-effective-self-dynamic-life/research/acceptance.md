# 有效自我与动态生活验收证据（2026-09-24）

## 正式链路及权威

权威表和作用域见 `design.md` §1，规划前能力接线/初始化语义去向见 `research/baseline.md`。实施后的对应关系：Foundation/analysis 保留原始及历史；`fluctlight_appearance_states` 和 wardrobe/worn 表为共享身体和物品的当前值；`fluctlight_profile_habits`、Goal/Intention 为 profile 当前值；已接受 Schedule 是计划，`life_events` 与 activity run 是发生的活动/结果。`working-persona.compilation.v2` 仅编译稳定人格和有效习惯。Prompt 和图像消费这些投影，不写当前权威。

正式 Main `RunConversationCognitionAgent` 与 WakeUp `ProcessWakeUp` 的脚本化 Provider + PostgreSQL 测试分别验证了原生 Eino ToolCall → `ExecuteTool` receipt → 原生 ToolResult → 下一次模型请求。独立 Tool 测试使用相同的领域事务，涵盖衣柜查询/换装、搭配、习惯、意愿、临时发型及活动开始/推进。`TestFormalDueNativeAgentStartsActivityAndResultSettlesAttempt` 证明 due 事实、scoped 引用、异步待结算 Outcome、经过时间的购物结果、意愿 attempt 成功。`TestFormalDueActivityResultPreservesPausedOrCancelledIntention` 与 `TestFormalDueActivityResultCannotSettleSupersededAttempt` 证明暂停、取消、更新和暂停→恢复之后，结果仍落成真实物品和事件，但不复活旧意愿尝试。`TestFormalDueStartSurvivesAgentFailureAndSettlesOnce` 注入 Tool start 后最终 Provider 失败，确认 pending Outcome 在活动事务内已提交，稍后结果幂等结算：活动、Outcome 和 attempt 各一条。完成前不会入柜或自动穿上；失败购物不会入柜或完成意愿。

关键断点修复：原始 `life_profile.appearance`/衣着不再逐轮冒充有效值；`SlotAppearance` 读取共享身体；普通习惯更新触发画像来源版本，穿着/剪发不触发人格重编译；`intention.due` 的 `candidate` 引用在 due 修订后按实体、revision、attempt、目标和本轮投影重新绑定，Provider influence 通过受控 index 校验；模型调用前在 due inbox 冻结参照快照；Tool start 与其 pending Outcome/receipt 原子提交，最终模型失败不遗失此审计事实；活动结果原子推进 Outcome 和仍然有效的 attempt。摘要和详情保留当前/历史时间语义，嵌套扩展的旧外观从当前画像及详情过滤，但 `historical_foundation` 保留旧值；媒体冻结当前身体及衣着版本，以两条 authority 行锁保证 stale 判定与完成提交连续。普通结构化和流式请求超预算均在 HTTP 前拒绝。没有添加场景/日程自动换衣的写路径。

## 脱敏最终请求摘录

捕获位置为 fake Provider HTTP transport 收到的**最终请求**，不是中间 PromptAssembly。受控样例：

| 请求 | 最终请求字符/字节 | Tool schema 数 | 关键事实与结果 |
| --- | ---: | ---: | --- |
| Main 首轮 | 23,041 / 24,045 | 22 | System 有有效画像；Runtime 有 hair_length=long、body_revision=0、已穿白衬衫（ownership=unknown）；真实输入“我想看你穿靴子”一次。 |
| Main 查询续接 | 24,310 / 25,322 | 22 | 原生 Tool role 包含 `wardrobe.inspect` 的 lost 靴子和 `inventory_complete=false`；未把未命中断言为完全不存在。 |
| Main 意愿续接 | 25,653 / 26,737 | 22 | 原生 Tool role 增加 `intention.decide` 的已提交 candidate receipt。 |
| WakeUp 活动首轮 | 21,338 / 22,534 | 25 | Runtime 有活动 ID/当前身体、穿着；触发事实不是伪造的用户新消息。 |
| WakeUp 活动续接 | 22,638 / 23,846 | 25 | 原生 Tool role 有剪发结果的 body_revision，后续模型选择 no_op；未产生可见消息。 |

媒体正式压缩输入摘录：`context_binding.appearance.body_fields.hair_length.value="短发"`、`body_revision=2`、`wardrobe_revision=3`、`worn_items=[{slot:"top",description:"深色上衣"}]`，`visual_identity.available=true` 仅为身份参考。相关测试在拍摄后状态又改变时将生成结果标为 stale。上述均为受控 fixture，不能证明真实生图准确性。完整本地脱敏 JSON 抓取文件被仓库 `.gitignore` 的 `research/*.json` 规则排除；本页是可审查的持久化摘录。

## 请求成本

`testdata/turn_path_cost_report.json` 由脚本化物理请求捕获；每次请求各自使用 `EstimatePromptTokens`（约每 Unicode rune 1.25 Token），累加工具续接。字符/字节按等价内容重新序列化后的 JSON 计数。Tool schema 和 response format 包含在最终请求估算中；本次新增 Tool 后两者没有可靠的单项旧版分解，因此不假报分项差值。真实 Provider usage、缓存命中和延迟未测得。

| 路径 | 物理调用 | 估算输入 Token 旧→新 | 字符旧→新 | 字节旧→新 |
| --- | ---: | ---: | ---: | ---: |
| 自然完成 | 1→1 | 24,250→32,174（+32.7%） | 19,400→25,739 | 21,248→27,587 |
| 回复 Tool 后完成 | 2→2 | 49,977→65,824（+31.7%） | 39,981→52,659 | 43,697→56,375 |
| 三轮接管/回复 | 3→3 | 79,170→102,941（+30.0%） | 63,335→82,352 | 68,927→87,944 |

一次性 migration/backfill 无模型调用；有画像重编译的存量任务必须单独显式执行，其 Provider 费用未估算为每轮开销。Prompt trace 对 section、版本、来源、入选/预算排除及字符、字节、估算 Token 留下结构化诊断；必要内容超预算会明确失败。

## 存量、测试及阻塞

在本机可丢弃 PostgreSQL 上，存量 `effective-life-backfill` 以限定 Owner/Fluctlight 顺序实际执行 preview→apply→rerun，结果依次为 `would_initialize`、`initialized`、`skipped`；核验 1 件初始衣物、1 条当前穿着、发长 `long` 和“喜欢散步”习惯。preview 不写入，重跑不会恢复随后改变的发长或不可用物品。`TestPostgresEffectiveLifeMigrationUpgrades0035WithoutChangingFoundation` 验证 0035→0036、再次 Apply 和 Foundation 保留；完整测试覆盖 empty→head。未连接生产数据库，未触发付费画像批处理。失败后可对同一限定范围重新 `--apply`，只补缺失基线；诊断中的无法判定外观和所有权保持 unknown。

命令及真实退出码：

| 命令 | 退出码 | 说明 |
| --- | ---: | --- |
| `GO_CORE_TEST_DATABASE_URL=<disposable PG> go -C apps/core-go test -race ./...` | 0 | 首次复跑发现历史详情过滤回归；修复与审计事实保留修正后最终复跑 `internal/core` 203.840 秒，包含迁移、正式 E2E 与独立 Tool 数据库测试。 |
| `go -C apps/core-go vet ./...`；`go -C apps/core-go build ./...`；`git diff --check` | 0 / 0 / 0 | 最终实现后执行。 |
| `pnpm generate`；`pnpm typecheck`；`pnpm test`；`pnpm build` | 0 / 0 / 0 / 0 | 前端、生成客户端和构建通过。 |
| `node --test infra/acceptance/run-go-live-provider-smoke.test.mjs` | 0 | 10/10，0 SKIP；已更新 29 Tool/19 Agent 的清单断言。 |
| `docker compose --env-file infra/compose/fluctlight.env.example -f infra/compose/fluctlight.compose.yml config --quiet` | 0 | Compose 结构解析通过，未启动完整栈。 |

**BLOCKED：** 当前环境未提供 `FLUCTLIGHT_LIVE_PROVIDER_URL`/`FLUCTLIGHT_LIVE_PROVIDER_MODEL`、ComfyUI 与视觉对象存储的真实配置；因此真实模型多 profile/多样本行为评估、实际图片视觉准确性和 live acceptance runner 均未执行，不能标为通过。脚本化模型只证明协议/事务/最终请求。完整 Compose 服务烟测也没有所需平台依赖配置，不能以静态 `config` 代替运行验收。
