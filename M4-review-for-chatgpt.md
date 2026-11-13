# M4 管理闭环与原始总验收回执

2026-11-13。**M4 状态为 PARTIAL。** 本轮九个限定单元完成实现或验收并达到各自 VERIFIED 范围。原 T01～T12 总验收继续保留真实模型、真实媒体和已登记换衣回退失败，父 T02、T04、T05、T06、T07、T08、T09、T10、T11、T12 保持原验收要求。

Owner 本轮明确选择“本轮先保留真实模型未验收”。本次新增的模型响应均标记为受控 Provider，真实 HTTP、PostgreSQL、正式任务/工具和浏览器操作分别留证。

## 1. 现场、受检版本与保全

- 分支为 `codex/fluctlight-meromero-plugin-humanization`，M4 起点为 `39068f2aaf225a5615a8906895bf4223ef03d1a0`。
- 最终 Go 全包和扩展隔离 Core 回归受检提交为 `0ece567a3b6fab54b9dc31d3a8eed06a9a29ed21`。最终 HTTP/Browser 与管理跨层回归受检提交为 `160cc3a26a8728759b44b46b36f403f9fd56db99`，其后该部分生产实现与测试实现一致，差异为原 Core 插件用例的局部前置和文字记录。
- 新增生产实现集中于 `8ca56aa5` 的诊断读取授权与 `0c0e259b` 的只读记录回放。后续提交为测试、夹具、状态和证据。
- 后端测试使用干净 Git 导出及明确记录的本单元覆盖文件。前端使用匹配的已提交生产组件及真实 BrowserClient。各场景创建、核对并清理专用 PostgreSQL 容器中的随机隔离库，内容为合成数据。
- 四处原有草稿持续保留未暂存，SHA-256 与起点一致，完整值见 [交付清单](docs/verification/meromero-m4-evidence/delivery-manifest.json)。R0 已有持久保全继续沿用。

| 原有草稿 | SHA-256 |
| --- | --- |
| `agent_model_routing_test.go` | `d344d7a5d2aa75212391ea70c36148b63fdccf4e8955ed6c88c0c5ddcf4eb857` |
| `media_capture_plan.go` | `c0bfe6111a5e8ef6da497c5a6887fb48ccc271a18d9bfa96df21d2ee79fabb75` |
| `media_prompt_branches.go` | `f24c2101f051e82f73052d21a9c80b398d6473c0c10ec656afffe7114a764279` |
| `task_prompt_sources_test.go` | `b8a39afa9e3e6b740d60bd73fac1bceadcd972e4c54553e2c50ecbac1ca89fa7` |

上表文件位于 `apps/core-go/internal/core/`。本轮提交的图片数量为 0，迁移文件变更数量为 0。截图保存在仓库外。

## 2. 已关闭的限定单元

完整实施、失败和提交后结果见 [M4 进度](docs/verification/meromero-m4-progress-20261113.md)。VERIFIED 指表内限定行为和验证层次。

| 单元 | 原要求与唯一目标 | 依赖 | 实际交付及验证 | 状态 |
| --- | --- | --- | --- | --- |
| M4-01 | T11 14.3，原始诊断读取与搜索需要明确调试授权 | 现有 Owner 与启动配置 | Core 同时校验 Operator 开关和 Owner，覆盖新旧列表/详情、模型、媒体、事件、Kev、工作流原始内容及导出/清理全文搜索。提交后 74 项 Core 与 124 项 HTTP/Browser/配置回归通过 | VERIFIED，权限合同 |
| M4-02 | T11 14.4，单次运行已存记录的只读回放 | M4-01、现有诊断 GET | 正式调用详情新增回放入口，九类来源独立快照分页，保留失败、后续页、缺失提示与稳定选中。13 项 React 和真实 HTTP 来源测试通过 | VERIFIED，记录回放 |
| M4-03 | T11 14.3，验证已有留存边界及业务证据保留 | 专用 PostgreSQL | 四类诊断各 10,002 条，默认清理后各 10,000 条，再次清理删除 0，业务 Agent 恢复整行一致。提交后通过 | VERIFIED，清理函数 |
| M4-04 | T11 14.1，模型覆盖页面保存、重载、正式任务消费及撤销 | 既有 T02 路由与匹配隔离 App | 真实 React → BrowserClient → HTTP → PG，重新挂载回读，正式 BuildContextProjectionFor / RunReflectionProposalTask 请求覆盖模型，撤销后恢复默认。两轮新库、新页面进程通过 | VERIFIED，跨层集成，受控 Provider |
| M4-05 | T11/E15，原生浏览器模型管理闭环 | M4-04、正式 Web 构建 | IAB 操作正式管理页，保存覆盖、真实 reload、回读来源、执行正式 reflection，撤销后重复。三阶段合同通过 | VERIFIED，原生 UI 与正式任务，受控 Provider |
| M4-06 | T11 14.1，插件页面设置影响独立工具 | 已有插件与正式 ExecuteTool | 页面关闭两个插件并重载后工具拒绝；只开启 Moment 后独立 Writer 正式发布一条文字动态，Image 仍拒绝，reflection 继续成功。两轮提交后通过 | VERIFIED，跨层集成，受控 Provider |
| M4-07 | T11/T08，表达审核启停影响新 direct 请求 | 真实受理来源与已准备人格 | 正式面板开启学习/参考、提取、审核、重载回读；新 HandleTurn 请求含候选，停用重载后新请求退出参考。两轮提交后通过 | VERIFIED，Owner 审核与请求消费 |
| M4-08 | T11/T09，记忆纠错、版本来源和新请求消费 | 已有记忆治理 | 正式 RecordMemory，页面 CAS 纠错、重载当前及旧版来源，新 HandleTurn 与 memory.recall 消费纠错，原消息逐字保留。两轮提交后通过 | VERIFIED，Owner 纠错与请求消费 |
| M4-09 | T11 14.2/14.4，实际已存正式回合的回放及零新增效果 | M4-01/02、已提交正式回合 | 13 条实际记录关联模型、绑定版本、Prompt Slots、工具和提交消息。面板 9 次请求全部 GET，模型请求增加 0，五类业务表完整行集相等；原生浏览器逐条查看 1～13 | VERIFIED，正式记录与原生 UI |

### 诊断读取授权的实际决定

采用启动级 `FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ`，默认 `false`，API 将其注入 App，权限同时要求当前 Owner 身份。开启后需要 API 重启，Compose 需要按现有方式重建 Core 容器。该实现提供进程级 Operator 授权，用户角色体系沿用 Owner。

默认关闭时，完整诊断及部分使用原始记录的诊断/任务详情显示明确 403 和开启提示。元数据白名单流式导出、既有安全 Agent 摘要仍可读取。全文搜索会在投影之前匹配私密内容，因此 q/qAny/q_any 在导出及清理计数中也要求调试授权。

原始采集继续采用既有诊断持久化及周期清理。Worker 接线为每 15 分钟、30 天、每表最新 10,000 行目标；清理间隔内记录量可以超过目标。本轮对清理函数真实执行，15 分钟调度接线为源码核查。

### 回放的事实边界

回放展示实际已存记录。各来源使用自己的创建成员快照，可变状态按读取时状态展示。时间相同和缺失时间的记录继续提示顺序待核对。已清理或缺失的输入、工具输出及跨异步关联继续显示缺口。

重放控件只移动记录选择及读取后续页。正式恢复、媒体核对和工作流控制仍属于各自领域命令。实际证据覆盖已存输入与回执的查阅，隔离重新推理和缺失历史重建继续列在保留项。

## 3. 生产落点及运行链

| 落点 | 实际作用 |
| --- | --- |
| `internal/core/diagnostic_debug_access.go`、`app.go` | Owner 与 Operator 双重授权，全文检索权限复用 |
| `internal/config/config.go`、`cmd/api/main.go`、Compose env | 启动读取、非法布尔值拒绝及正式 API 接线 |
| `diagnostic_records.go`、`diagnostic_model_runs_filter.go`、`diagnostic_page.go`、`operations.go`、`kev_*`、`workflow_controls.go` | 新旧正式读取入口中的一致授权 |
| `internal/httpapi/browser_backend.go`、`domain.go`、`browser/routes.go` | 权限状态与稳定错误码透传 |
| `apps/web/src/react-app/shared/DiagnosticReplayPanel.tsx`、`ModelCallDetails.tsx` | 正式调用详情中的只读回放、分页、迟到响应隔离和记录导航 |
| `apps/web/src/v3/domain-presentation.mjs` | 调试读取关闭时的明确反馈 |

Core 路径以 `apps/core-go/` 为根。新增模型任务、生产模型路由、Prompt 构造、采样、token 上限、数据库结构、Temporal、媒体发布和默认聊天模式的本轮生产 diff 均为 0。管理验证复用现有正式调用与领域处理。

默认私聊继续读取设置中的文本模型，缺省 direct，已有显式 `fact_only` 等模式按既有设置运行。普通聊天的同一主 Agent 可以产生原生工具续轮和有界修复，物理请求数按每次任务记录。独立 reflection、Moment、Diary 等任务保留独立职责和配置。

## 4. 测试、命令及真实层次

| 层次 | 命令/环境 | 实际结果 | 证据 |
| --- | --- | --- | --- |
| UNIT / CONTRACT，全 Go | 干净 `0ece567a`，清除 DB/live/Temporal 夹具开关，`go test -json ./... -count=1 -timeout=10m` | 退出 0，18 个有测试包通过；含子测试事件 3,181 PASS、0 FAIL、1,292 SKIP。SKIP 单独保留 | [完整汇总、SKIP 明细及原始 JSONL 校验](docs/verification/meromero-m4-evidence/unit-all-summary.json) |
| 隔离 PG 与受控 Provider，直接影响 Core | `go test ./internal/core -count=1 -run '<日志首行完整表达式>' -v -timeout=300s`，新隔离库 | 81 个顶层 PASS，0 FAIL，0 SKIP | [最终 Core](docs/verification/meromero-m4-evidence/final-core-postfixture.log) |
| 真实 HTTP/PG 与正式任务，受控身份/Provider | `FLUCTLIGHT_M4_WEB_ROOT=<匹配Web源码> go test ./internal/httpapi ./internal/httpapi/browser ./internal/config ./cmd/api -count=1 -run Test -v` | 125 个顶层 PASS，0 SKIP；API 包编译 | [最终 HTTP](docs/verification/meromero-m4-evidence/final-http.log) |
| 两轮完整管理跨层场景 | 同一正式测试 `TestModelManagementReactHTTPPersistenceAndRuntime`，`-count=2`，每轮新库及新 Node 页面进程 | 模型、插件、表达、记忆及已存回放均实际执行，退出 0、无 SKIP | [提交后两轮](docs/verification/meromero-m4-evidence/replay-stored-run-postcommit.log) |
| React 组件/交互 | `node --test` 的 replay/model-calls/react-flows/plugin-health/expression-panel/expression-recovery/demo-replica-memory 七文件 | 57 PASS，0 SKIP，退出 0 | [Web 日志](docs/verification/meromero-m4-evidence/final-web.log) |
| Web typecheck/build | `pnpm --filter @fluctlight/web typecheck`、`build` | 均退出 0；生产构建扫描通过，React-only 图 407 本地模块；附 chunk 体积警告 | [类型检查](docs/verification/meromero-m4-evidence/final-typecheck.log)、[构建](docs/verification/meromero-m4-evidence/final-build.log) |
| UI-E2E，原生浏览器 | 正式生产构建 → 隔离后端/PG，模型覆盖保存/reload/撤销；实际回合逐条回放 | 通过；身份及 Provider 为受控夹具。模型三阶段有正式任务合同，回放后模型调用增量 0、完整业务行集相等 | [模型管理](docs/verification/meromero-m4-evidence/management-native.log)、[实际回放](docs/verification/meromero-m4-evidence/replay-native-ui.log) |
| REAL_MODEL | Owner 本轮明确暂缓；此前使用的两个 loopback 端口预检连接拒绝，GUI 锁屏 | 本轮未验收。历史 R1/M1 实模证据按原提交与场景保留 | [环境记录](docs/verification/meromero-m4-evidence/model-environment-review.txt)、[预检](docs/verification/meromero-m4-evidence/provider-preflight.log) |
| REAL_MEDIA | 继承 M2 用户工作流真实出图 smoke，完整产品动态 Prompt/视觉/交付仍按原状态 | 本轮未新增实媒体验收 | [M2 回执](docs/verification/meromero-m2-review-20261113.md) |

截图位于仓库外 `fluctlight-m4-native-override.jpg`、`fluctlight-m4-native-restored.jpg`、`fluctlight-m4-native-replay.jpg`，目录为 `/private/tmp/`。Git 保留文字、检查和必要的几何/校验数据。

## 5. 失败及处理

所有样本按阶段独立留存于 [M4 证据目录](docs/verification/meromero-m4-evidence/)。主要过程如下。

1. 权限红测在起点真实放行原始读取。Core 修复后，旧 BFF 错误先丢成 502/通用 403，首次透传修正又二次包装了已转换错误。后续按 SurfaceError 类型修正，跨入口及回归通过。
2. Core 原诊断断言补显式授权。HTTP 干净导出补两份既有跨目录测试输入。失败原日志保留。
3. 回放首轮 typecheck 快照类型缺失、后续页选中项漂移均在本单元修正；分页、快照变化、权限和 Owner/范围切换测试通过。
4. 管理跨层夹具先后缺少完整上下文、affect/profile habits/compiled persona 前置，改用正式上下文和人格准备入口。生产配置 facade 的通知计时器造成一次 90 秒子进程超时，测试正确释放后通过。另一次提交后异步 catalog 选项尚未返回，改测试等待真实选项，随后两轮通过。
5. 插件首轮测试使用 legacy text 参数，当前独立 Moment 合同按无正文参数接收，测试按现行输入及受控 Writer 纯文本适配，原模式和合同保留。
6. 表达首轮复合字面量编译错误、表单定位及 wire 名错误均属于测试。实际公布的 `conversation_reply` wire 名已作为响应名，正式 direct 合同通过。
7. 记忆首轮附加要求某次检索必选原发言，该次“听雨”没有选出原始发言。原文逐字及旧版来源页承担历史保留核验，当前记忆和新请求继续严格核对纠错。日志保留 `historical_statement_selected_for_this_query=false`。
8. 最终原插件回归的旧夹具缺少发布核验文本模型，在 M4 起点同环境复现。仅补该测试前置，原正文和全部原断言保留；起点与当前同样通过。详见 baseline-plugin-text / plugin-fixture-* / final-core-postfixture。
9. **换衣回退仍有两个真实执行的失败子场景**，当前与 M4 起点相同。`wear_unavailable_fallback_repair/stream=false` 和 `/stream=true` 均 FAIL，其他同组 24 个子场景 PASS。既有工单 `reply-wear-fallback-correction-regression` 继续保留，根因与原策略兼容仍待该工单处理。本轮回退策略生产 diff 为 0。早期实施曾将它记录为相对原始实施基线的回归，M4 起点已存在的事实同时保留。[当前与起点对照](docs/verification/meromero-m4-evidence/known-failure-comparison.txt)
10. 两次 AEL 长执行租约过期均在代码提交、相关写入停止后恢复同一任务。状态记录与技术证据独立，父任务保持原要求。

## 6. 原 T01～T12 验收条目逐项映射

编号 a～d 与各原工作项的四条 acceptance 顺序一致。原详细业务要求继续由对应 `.ael/work-items/meromero-t*.md` 管理。以下标记提供当前证据边界。

| 原任务 | a | b | c | d | 整体状态及原要求保留项 |
| --- | --- | --- | --- | --- | --- |
| T01 入口盘点 | 31 个正式 Agent 与变体、生产/未启用区分沿用清单 | 非 Formal/插件/旁路/后台/重试/独立 Tool 沿用清单 | caller、配置、Prompt、Slot、校验/副作用/渠道沿用定位 | 原盘点不确定项沿原记录 | VERIFIED，盘点范围；[清单](docs/verification/meromero-agent-inventory-20261109.md) |
| T02 任务模型 | 三级优先级、独立视觉有合同和历史实模 | 私聊/多类后台快照已有实现与受控证据；全类型真实恢复保留 | M4 模型页面及继承来源、敏感读取闭环通过 | R1 真实两轮沿用；本轮实模暂缓 | PARTIAL；每类恢复及完整物理配置证据继续保留；历史 completed 通过既有反馈说明缺口 |
| T03 输入契约 | 声明及输入范围已验收 | 可信边界及类型投影已验收 | 原生结构/整组历史/无 token cap 合同已验收 | 45 个原 T03 提交后场景沿用，M4 又观测实际 Slot | VERIFIED，原输入契约；[原验收](docs/verification/meromero-m2-evidence/t03-acceptance-20261113.md) |
| T04 单主链 | 配置选模、default direct、原生循环有实现/受控证据 | 原子回复、持久效果和恢复有局部证据；旧回退两失败保留 | claims/influences/偏好/Goal/关系消费者在 M2/M3 收口局部证据 | 本轮实模暂缓；M1 历史 direct 样本按原范围 | PARTIAL；真实默认主链、完整认知效果与实时增量流继续按原条目 |
| T05 插件 | 注册/依赖/健康/启停已有实现，M4 配置消费通过 | 独立/Agent 同实现与权限有证据 | M2 恢复/幂等及 M4 独立文字动态通过 | 完整真实图像/视觉/交付保留 | PARTIAL；Studio/视觉身份不确定提交后的完整接回能力仍见原工单 |
| T06 发布可靠性 | 状态、质量、交付分离有合同与受控证据 | M1 direct 完成声明及 M2 Moment 局部证据；自然措辞/画面语义保留 | 重复/恢复/新消息和真实 Temporal 特定暂停场景有证据 | 第二次 reject、目录权限/发现有局部证据 | PARTIAL；同 ID 拒绝重放、完整取消/实媒体及旧回退失败保留 |
| T07 节奏 | 连续受理、superseded、旧候选栅栏已实测 | WakeUp 条件、义务、退避、重置在 M3 实测 | 受控正向/静默通过；真实自主选择保留 | 隔离并发有证据；主观效果待 Owner/真实模型 | PARTIAL；[M3 回执](M3-review-for-chatgpt.md) |
| T08 表达 | 真实来源、审核、启停/撤销有实现与证据 | 独立开关、情境选择与来源重查有证据 | M3 约束与人格/关系对照已实测，广泛语义保留 | M4 页面→新请求启用/停用完整通过 | PARTIAL；真实自然采用与关系称呼语义继续验收 |
| T09 记忆 | 既有来源/作用域与发言事实区分有证据 | 摘要/画像/派生/inflight/时效在 M3 实测 | 旧来源暂停与 Owner 恢复、锁序/Actor 场景有证据 | M4 页面纠错/来源/新请求/原文保留通过；原 ticket 回执再验通过 | PARTIAL；自动识别纠错与真实模型语义保留 |
| T10 领域保护 | Goal 原成功/去重/上限/暂停在 M3 复验 | 日程/到场/结算、衣柜各状态有关键回归 | 人格/NPC/世界/Studio/日记等沿 M3 证据 | 旧 Goal 时间前置已修正；当前完整实模型/媒体领域矩阵保留 | PARTIAL；原 Goal 失败不再沿旧回执误记成未修，新增真实能力证明仍需执行 |
| T11 管理/Trace | 九个限定单元覆盖模型、插件、表达、记忆、诊断 | 页面与正式运行跨层闭环通过；原生模型/回放通过 | 默认调试权限、实际13记录关联及隐私合同通过 | 只读回放/留存通过，隔离重新推理与跨异步完整性保留 | PARTIAL；真实服务/自然模型与完整媒体健康链按原要求 |
| T12 总验收 | E01～E18 逐项分层见下表 | 失败、SKIP、Owner暂缓与环境状态分开保留 | 本回执、主报告入口、需求/代码/测试映射和脱敏证据已交付 | 迁移、旧在途兼容、回滚及原始自检见本回执 | PARTIAL，回执已交付，原总验收保留 |

## 7. E01～E18 总矩阵

| 场景 | 已有证据与本轮新增 | 当前保留项 | 状态 |
| --- | --- | --- | --- |
| E01 模型与快照 | R1 两轮真实覆盖/隔离/撤销，M1 快照，M4 两轮 UI/正式 reflection 及原生保存恢复 | 当前默认主链实模、全异步真实恢复；请求 model 与后端加载实现分开 | PARTIAL |
| E02 任务输入隔离 | T03 原45项提交后验收与声明，M4 实际请求 Slot/来源 | 真实自然理解继续随模型场景 | CONTRACT VERIFIED；语义未验收 |
| E03 原生与独立 Tool | 现有发现/load/回执续轮，M4 独立 Moment/Image 门禁及正式 direct 发布 | 实模完整领域选择、全部权限组合和同 ID 拒绝重放 | PARTIAL |
| E04 无人物图片 | M2 主体/时间/衣柜隔离、SDXL 正向边界受控回归 | 真实产品无人物渲染、质量与交付 | PARTIAL |
| E05 换衣自拍 | M2 正式换衣/冻结/受控交付，当前同组24子场景通过 | 两个 fallback_repair 当前及 M4 起点 FAIL；真实完整链 | PARTIAL / 已知 FAIL |
| E06 缺失紫色大衣 | 不可用物品拒绝与真实穿着约束有证据 | 真实模型拒绝/未来购买选择及真实图片 | PARTIAL |
| E07 异步状态 | 生成/质量/交付/取消合同，第二次 reject 保持拒绝 | 完整真实生命周期、自然声明和取消组合 | PARTIAL |
| E08 幂等恢复 | M1/M2/M3 新消息/旧效果/接管；真实 Temporal 的特定暂停 | 全崩溃窗口、真实媒体/多端组合 | PARTIAL |
| E09 插件隔离 | M2 独立插件正反例，M4 页面重载及正式 Tool 两轮 | 真实完整图像/NPC及在途链 | PARTIAL |
| E10 表达 | M3 来源/约束，M4 页面提取审核启停→实际 direct 输入 | 真实模型自然采用、广泛注入语义与关系称呼 | PARTIAL |
| E11 记忆 | M3 旧源/派生/锁/时效，M4 页面与原文保留，当前 ticket 原发言合同通过 | 真实自动纠错与查询；本轮“听雨”检索未选择旧发言如实保留 | PARTIAL |
| E12 主动与静默 | M3 正反受控场景、持久20→40→60分钟退避/重置 | 多次真实模型主动/静默及自然节奏 | PARTIAL |
| E13 多人格交流 | 人格状态与条件合同证据 | 两种人格的普通/分歧/玩笑/认真/分享实模多样本与 Owner 体验 | 未验收，Owner 本轮暂缓实模 |
| E14 原领域 | M3 206 Core+1迁移及后续6 Goal 回归；M4 直接影响回归 | 完整真实生活/视觉/多端矩阵及已登记基线流问题 | PARTIAL |
| E15 管理消费 | M4 四类管理跨层真实HTTP/PG、正式任务，两轮及模型原生浏览器 | Provider 身份/响应为夹具；真实服务能力保留 | UI/CONTRACT VERIFIED，整体 PARTIAL |
| E16 发布/质量 | M1 零依据声明守卫及实模检查器，M2 第二拒绝/附件门禁 | 当前实模广泛措辞、历史动态主体及真实媒体 | PARTIAL |
| E17 未知与来源 | 原伪造引用/Actor边界、M3 Goal真实接受条件，M4原文与当前记忆分离 | 真实模型“没有检索到”及承诺接受/拒绝完整自然场景 | PARTIAL |
| E18 实际能力 | 历史真实合成 Tool、M2 单独 ComfyUI 工作流出图、特定真实Temporal暂停 | 真实视觉/产品生图/规划/自主选择与全链 | PARTIAL |

## 8. 剩余实现、证据与阻塞归属

| 缺口 | 分类及已有依据 | 归属与恢复条件 |
| --- | --- | --- |
| 当前实模型默认聊天、自然主动/静默、多人格、自然表达与纠错 | EXTERNAL_BLOCKED / EVIDENCE_MISSING，Owner 本轮暂缓，预检和模型环境记录 | 原 T02/T04/T07～T10/T12；选定本机模型服务与安全配置就绪后按原场景执行，多次新任务留存失败 |
| Studio/视觉身份提交不确定后完整人工核对接回 | IMPLEMENTATION_MISSING / POLICY_REQUIRED，M2 已有 `studio-unknown-submission-manual-recovery`、`visual-identity-unknown-submission-manual-recovery` | 原 T05/T06/T10；同源结果归属与原工作流接管规则继续由原工单明确。普通媒体暂停/核对已有局部实现 |
| 同一无权限 ToolCall ID 重放 | IMPLEMENTATION_MISSING，M1-03b 原红测与 Owner 暂缓 | 原 T06；既有 `meromero-m1-03b-replayed-denial`，保留完整原验收 |
| 原生实时 token 增量 | BASELINE_FAILURE / EVIDENCE_MISSING，M2 同环境起点及当前仅已提交流证明 | 原 T04/T12；`durable-chat-missing-live-token-stream`。已提交消息/终态流证据沿用 |
| 换衣回退两子场景 | M4 BASELINE_FAILURE，早期改造登记的 REGRESSION 状态同时保留 | 原 `reply-wear-fallback-correction-regression`；按原工单核对现行终止4xx及可回退错误的预期与实现 |
| 产品真实媒体整链 | EVIDENCE_MISSING，M2 单独出图通过，动态替换兼容另有工单并经 Owner 暂缓 | 原 T05/T06/T12；`image-krea2-multi-lora-policy-compatibility` 及可证明匹配的隔离工作流/视觉/存储 |
| 任意历史完整输入、逐请求全部版本字段与通用隔离重新推理 | EVIDENCE_MISSING，旧记录存在缺失/图片脱敏/可变状态，当前提供已存记录只读回放 | 原 T11/T12；按已有具体记录核实，缺失不能补造，重新推理需要独立隔离副作用环境 |

已登记工单原范围保留。本轮新增生产代码的事实边界、已知失败和 Owner 暂缓条目共同决定 M4 的 PARTIAL 状态。

## 9. 恢复验证与回滚说明

真实默认聊天的现成入口仍为 `TestLiveDirectConversationDoesNotRequireASecondWriter`。安全注入一次性 `GO_CORE_TEST_DATABASE_URL`，确认本机服务实际提供的准确模型 ID 后设置 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`、`FLUCTLIGHT_LIVE_PROVIDER_URL`、`FLUCTLIGHT_LIVE_PROVIDER_MODEL` 及仓库外 evidence 目录，执行以下命令。预期实际运行、无 SKIP、正式回复与认知结算满足既有合同，失败样本逐次保留。

```sh
go -C apps/core-go test ./internal/core -count=1 -run '^TestLiveDirectConversationDoesNotRequireASecondWriter$' -v -timeout=15m
```

本轮新增迁移数量为 0。回滚按单元审阅提交和依赖，先保全用户工作区，再逐个反向处理测试/原生夹具、回放 UI、诊断读取授权。主要生产提交为 `0c0e259b` 和 `8ca56aa5`，后续测试存在对其公开字段/组件的依赖。诊断授权回退会恢复旧 Owner 读取边界，应在发布核对中明确记录。

业务数据与已提交领域效果继续保留，回滚操作采用代码版本变更。实际回滚尚待单独操作授权，当前交付为本地仓库提交与可复核回执。

## 10. 原始最终自检

| 原自检 | 当前答复与范围 |
| --- | --- |
| 只更换模型 | 已有任务路由、输入治理、插件、发布、生活系统和本轮管理/调试交付，分别见 T01～T12 映射 |
| 固定 Qwen 或中介 | 默认配置文本主链的受控执行使用一个主 Agent；独立任务、可选 fact_only 与历史兼容路径按各自配置保留，实模型默认链本轮暂缓 |
| 万能 Agent | 沿用31个正式 Agent 和既有变体登记，M4 生产模型入口新增数为0 |
| 插件仅接口 | 正式领域与独立 Tool 已实测；完整真实媒体和手工接回缺口保留 |
| HTTP/JSON 冒充能力 | HTTP、合同、数据库、受控模型、历史实模、真实媒体和原生 UI 按第4节分别记录 |
| 认知/Goal/人格保护 | M2/M3 原消费者与关键领域回归沿用，当前换衣回退两失败保留，完整真实矩阵待验 |
| 全任务完整人格 | T03 输入契约和45项映射沿用；当前实际13条记录再次提供 Slot 和来源证据 |
| 表达/WakeUp/记忆遗漏 | 核心与管理闭环有对应证据，自然模型、主观体验、未决兼容在第8节保留 |
| 发布只靠Prompt/正则 | 正式事务、真实资源、幂等及有界语义核验有对应测试；自然措辞全覆盖继续待验 |
| 停用仅隐藏按钮 | 页面设置后正式独立工具拒绝，文字动态启用后真实持久化且Image仍拒绝，普通正式任务继续成功 |
| 全部 Tool 权限已证明 | 已有受控目录/独立工具范围，完整权限与真实模型组合继续原验收 |
| 第二套Runtime | 生产沿现有 Go/Eino/ADK/Capability/Outbox；新增原生服务夹具位于测试构建 |
| 人格/括号动作压平 | 角色表达规则及偏好机制沿用；真实双人格各语境体验继续E13 |
| 强迫答应或降低条件 | 技术夹具显式标记受控回复；原Goal成功条件、Actor授权、业务拒绝和发布断言继续使用 |
| 未执行真实测试写成通过 | 本轮REAL_MODEL为Owner明确暂缓；1,292个SKIP、两个回退FAIL及真实媒体保留项逐项可查 |

## 11. 状态维护与停止点

T11 已通过既有 AEL submit 记录局部结果和保留项，状态为 pending/wait_environment。T12 复用原工作项整理总验收。T02 历史完成记录及既有反馈继续保留。本回执、原工作项、证据及台账共同提供真实验收状态。

当前交付停在 M4 回执。真实模型按 Owner 本轮选择保留未验收，原始未完成项继续保留在对应父任务和已登记工单中。
