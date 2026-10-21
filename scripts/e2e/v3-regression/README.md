# v3 模型相关回归：L01—L19

这套脚本由用户在全部功能交付、隔离环境升级后逐个执行。开发期间可以执行 `--self-test` 和语法检查；不要提前运行 `--live`。后续新增模型相关功能必须增加对应脚本、断言、错误诊断及操作册条目。

脚本使用真实公共 API、原聊天 NDJSON 协议、实际 Worker 产品状态及持久化读回。复杂并发/中断场景调用已有隔离 Go 测试，组件恢复场景调用现有 Node 测试；这些受控测试与真实模型结果在报告中分别标注。没有伪造 API 响应、直接写数据库制造生活结果或替换模型为本地假回答的业务路径。

2026-10-19最终核验：24项脚本自测通过；真实 `http://127.0.0.1:18080` 仅运行环境/Owner预检通过，模型触发0。没有运行17项`--live`。匿名session的401与503已区分，私有Cookie在第一次session请求前导入。最终环境、私有Cookie路径和业务Worker启动说明见 [统一操作册](../../../docs/v3-llm-acceptance.md)。

2026-10-20 L04/L13 修复：空白角色不预设世界，原脚本只配置主角色，漏配第二角色。共享 `npc-isolation.mjs` 现先为第二角色创建/选用世界并读取双方实际绑定，再应用同一精确版本NPC定义，验证独立身份、历史和双向跨读拒绝。世界选用未持久化或创建失败会直接报前置错误，不能以422冒充隔离成功。保留原 `--run-dir` 即可续跑；脚本自测28项通过。真实PG定向测试已编译并尝试执行，但专用数据库未运行且OrbStack Docker socket不存在，未计通过；本轮未执行 `--live` 或模型调用。

## 准备

最终交付时需先启动最新 API、Worker、PostgreSQL、Redis、Temporal namespace，以及相应 Provider、Comfy 工作流和媒体存储。测试部署必须和用户正在使用的库、队列、bucket 隔离。

设置以下环境变量。密码和 Cookie 文件仅从环境读取，不写报告或命令参数：

| 变量 | 含义 |
| --- | --- |
| `FLUCTLIGHT_E2E_BASE_URL` | 实际 API，如 `http://127.0.0.1:8081` |
| `FLUCTLIGHT_E2E_ORIGIN` | API 配置的可信前端 Origin，必须精确匹配 |
| `FLUCTLIGHT_E2E_ISOLATED=1` | 明确选择任务专用的测试部署后设置；没有此项不会发送业务写操作 |
| `FLUCTLIGHT_E2E_PASSWORD` | 测试 Owner 密码；或者使用下面的 Cookie 文件 |
| `FLUCTLIGHT_E2E_COOKIE_FILE` | 私有 Netscape/curl Cookie jar，不能放进版本库 |
| `GO_CORE_TEST_DATABASE_URL` | 隔离测试数据库管理连接；Go fixture 会创建独立测试库，禁止填用户业务库 |
| `FLUCTLIGHT_E2E_TIMEOUT_MS` | 每次模型/任务等待上限，默认 600000 毫秒 |
| `FLUCTLIGHT_E2E_REQUEST_TIMEOUT_MS` | 普通 API 请求上限，默认 30000 毫秒 |
| `FLUCTLIGHT_E2E_POLL_MS` | 状态轮询间隔，默认 3000 毫秒 |
| `FLUCTLIGHT_E2E_MAX_MODEL_OPERATIONS` | 每脚本显式模型触发操作上限，默认 32；不是底层模型调用轮数或费用统计 |
| `FLUCTLIGHT_E2E_DIARY_DATE` | L06 使用的已结束业务日期；必须有该测试角色的真实素材，不补造过去记录 |
| `FLUCTLIGHT_E2E_REFLECTION_ID` | 可选，L07 已由真实生产者形成的反思 ID |

默认只允许 loopback 部署。确实选择远端隔离测试部署时才设置 `FLUCTLIGHT_E2E_ALLOW_REMOTE=1`。

## 执行

先检查脚本自身，不访问真实环境或模型：

```sh
pnpm regression:v3:self-test
pnpm regression:v3:list
```

统一使用一个私有结果目录。它保存本轮对象 ID、原请求身份、冻结输入及已确认的分析预览，便于后续脚本复用、失败定位及原身份恢复。仓库默认 `.v3-regression/` 已加入忽略列表。

```sh
# 只核对服务和登录；不运行场景或模型
node scripts/e2e/v3-regression/l01-initialization.mjs --preflight --run-dir .v3-regression/acceptance-01

# 完工后由用户逐个执行
node scripts/e2e/v3-regression/l01-initialization.mjs --live --run-dir .v3-regression/acceptance-01
node scripts/e2e/v3-regression/l02-about-you.mjs --live --run-dir .v3-regression/acceptance-01

# 也可由运行器严格串行执行；不会并行发模型请求
pnpm regression:v3 --live --run-dir .v3-regression/acceptance-01

# 只执行一个编号
node scripts/e2e/v3-regression/run.mjs --case L15 --live --run-dir .v3-regression/acceptance-01
```

L06 的正向日记需要已结束日期和真实素材；L07 反思需要实际窗口/生产任务；L13 相遇需要角色实际到达。缺少这些前置时脚本会明确阻塞，不能为了让脚本通过直接改时间、伪造在场或强制角色决定。L09 人工视觉评审需要用户观察真实候选，脚本不会填写假的观察或置信度。

每个入口支持 `--help`。`--no-unit` 只用于临时定位真实 HTTP 部分：被省略的受控测试标为未执行，不会被算作已通过。

## 17 个入口及证据边界

| 编号 | 文件 | 主要自动检查 |
| --- | --- | --- |
| L01 | `l01-initialization.mjs` | 初始化来源、层次、受控切换、同请求激活重放、有效状态保留 |
| L02 | `l02-about-you.mjs` | 关于你保存/清除、当前事实版本、真实 NDJSON、双模型诊断与当前字段消费 |
| L03 | `l03-turn-recovery.mjs` | 显式取消、原身份/原时钟重试、用户消息去重；断流/迟到回复的受控测试 |
| L04 | `l04-world-npc.mjs` | 世界/地点/NPC发布应用、身份隔离、浏览无副作用、跨角色拒绝 |
| L05 | `l05-goal-inventory.mjs` | 持有项、套装不穿衣、借入实际判定、世界目录→真实购物→结果及双来源读回、Goal承诺证据边界 |
| L06 | `l06-diary.mjs` | 无素材空态、真实日记、独立注记、回应任务、远程经历不变同场 |
| L07 | `l07-memory-reflection.mjs` | 真实记忆生产、修订/历史、重新召回、实际反思和反馈消费者 |
| L08 | `l08-studio-apply.mjs` | 发布版本、差异应用、实际运行对象、伤害拒绝、重放及生活保留 |
| L09 | `l09-visual-media.mjs` | 视觉参考版本、冻结会话、实际候选/图片字节、未登录授权拒绝 |
| L10 | `l10-autonomy-notifications.mjs` | 动态作者、禁止冒名、角色策略隔离、真实通知/已读；实际Tool策略测试 |
| L11 | `l11-foundation-self.mjs` | 提案不生效、实际编译接受、旧版本冲突、自我认识元数据、能力注册边界 |
| L12 | `l12-promises.mjs` | 原约定、真实判断/规划关联、改期/取消、证据履约与迟到请求隔离 |
| L13 | `l13-npc-runtime.mjs` | 实际NPC在场、真实Tool活动/机会、独立回应与事件来源、跨世界隔离 |
| L14 | `l14-world-events.mjs` | 事件默认关闭、显式执行、真实Life Event、认知消费、原队列重放 |
| L15 | `l15-studio-text.mjs` | 真实助手、needs_input、独立表达、持久文本素材和生活隔离 |
| L16 | `l16-studio-image.mjs` | 实际图片任务/字节、显式存图、受验证参考、授权、生活隔离 |
| L17 | `l17-creation-catalog.mjs` | 精确模板来源、新分析与激活、复制去重、归档/恢复、引用保护 |

原操作册中超出这些自动断言的真实浏览器行为、图像内容质量和概率性角色生产，保留在每份报告的 `reviews`/`coverageLimits` 中。脚本不把 API 回归、组件夹具或受控 Provider 用例称为整条浏览器/模型验收通过。尚未建成的领域消费者也不能因为已有其他类型的测试通过而自动补齐验收。

## 输出与定位

每例输出 `Lxx.json`；串行运行器追加 `summary.json`。内容包括步骤、明确预期、实得、错误码、失败栈、请求路径/状态/耗时、关联标识、模型触发次数及人工复核项。受控 Go/组件测试有独立日志。报告会脱敏密码、Cookie、密钥、数据库凭据以及原始模型 Prompt/推理。

| 状态 / 退出码 | 含义 |
| --- | --- |
| `preflight_passed` / 0 | 只通过环境/登录预检，场景和模型没有运行；串行汇总单独计数 |
| `passed` / 0 | 本脚本声明的自动断言已通过；`fullWorkflowAccepted` 仍为 false，不能自动代替统一交付验收 |
| `failed` / 1 | 业务/API/模型结果或断言失败；按 `steps[].error` 定位 |
| `blocked` / 2 | 环境、配置、依赖对象、真实素材/窗口或测试数据库不满足；未执行不算通过 |
| `review_required` / 3 | 自动断言通过，仍需查看报告列出的真实产物或浏览器行为 |

典型错误：`HTTP_TRANSPORT_ERROR`、`TASK_TIMEOUT`、真实 Provider 错误码、`RESOURCE_NOT_APPLIED`、`TESTS_SKIPPED`、`NONREPLAYABLE_OUTCOME_UNKNOWN`、`ASSERTION_FAILED`。失败时保留整个私有 run 目录；不要改掉原 `turnId`、请求 body、时间戳或幂等键来“修正”失败。

有幂等能力的写操作重用原请求。初始化分析等无幂等能力的操作保存已确认预览；若响应未知，停止并输出原命令，避免隐式再次消耗模型。新的一轮验收使用新目录，旧产物保留供比较。脚本不会清库，也不会删除既有用户角色。

## 后续功能接入

新增 LLM 流程时：在操作册加稳定编号，在 `cases.mjs` 登记入口与必要步骤，新增对应独立脚本及回归用例，更新本表。自测会核对操作册与脚本编号一致，防止只补文档漏掉脚本。新增 API/消费者完成后，应把现有 `review` 中可自动化的部分改成实际 API/持久化断言，保留来源、权限、冲突与恢复测试。

诊断关联要求最新服务支持只读 `parentRunId`：脚本沿服务端存储的父运行 ID 关联 `turn:` 与独立 `reply:`，以 `role` 判断用途、以 `bindingRole` 记录实际绑定，不从 Prompt 文本猜父子关系。

L05新增实际购物消费者：运行前设`FLUCTLIGHT_E2E_TIMEOUT_MS=1800000`，以覆盖至少15分钟的真实经过时间、模型判定及结果结算。脚本不会推进服务端时钟或直接改数据库。配置不足在purchase步骤报SHOPPING_WAIT_BUDGET_REQUIRED；角色未开始、失败或延期会保留实际任务/回合/来源错误，不冒充已取得。

0083扩展保持17个脚本入口：L08新增可复用搭配→实际库存套装映射，L15新增保留精确服饰引用的助手用例，L16新增按已核实服饰描述的真实图片预览并要求人工像素复核，L05新增独立整套换装请求。全部由用户在任务完工后逐项执行；任何新模型路径都随功能一起进入脚本和操作册。

2026-10-19结果断言补全：L10增加真实通知配置的expectedVersions/idempotencyKey保存、重放、旧版本及同key异内容拒绝；L11能力交付无实际fulfilled记录时明确blocked；L12改期核对原Promise/Commitment/Intention身份和完整时间窗口，新增独立不取消的通信约定、真实回复、Commitment评估与confirmed消息来源履约检查。纯验证器对失败终态、假完成、候选来源与错误消息引用均有负例。

备份/恢复不是LLM脚本：本轮实际整卷备份和仅新隔离目标恢复结果见 `docs/v3-functional-ledger.json` 的deliveryVerification及对应公开回执。用户若另做运维演练，应使用 `infra/backup/maintenance.py` 与同目录README，不在17个模型流程中隐式停止整站或恢复原卷。

2026-10-21新增L18（初始穿搭/场景标签/真实wear）和L19（世界框架/分类/地点物化），原L01—L17均保留。L01在穿搭初始化完成后才冻结人格切换的衣物基线，避免将合法后台准备误判为切换串状态。所有角色激活现在可能有穿搭后台模型调用，实际模型仍由用户执行。参见统一操作册L18/L19。
