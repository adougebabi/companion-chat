# v6.2-r6 换装链路复盘与验证

基线 `66712f04`。本轮按用户强调，集中处理长期未落地的换装：
Owner 方案请求、普通私聊动作、实际穿着持久化、页面读回与请求反馈。
不创建任务；不改变单次初始化、并发 1、无应用 Token 上限和串行准备约定。

## 为什么前几轮没有解决

根因类别是跨层契约与测试覆盖缺口。衣柜初始化能生成并穿上，不能证明
Owner 的套装请求能获得工具；直接让测试 Provider 返回 wardrobe.wear，
也不能证明真实物理请求里存在该工具。此前的 fixture 没有检查这件事。

`outfit-request-native-cognition.json` 中两次初始 NativeCognition 请求的
`metrics.prompt_budget.section_counts.tools` 都是 0；一次超时，一次返回普通中文。
第三条是工具关闭的最终格式修复，零工具在这个修复阶段符合设计。
最终 JSON 中的 accepted/committed 只是模型声明，实际没有原生 ToolCall。

`ownerLifeRegistry` 只安装了检查工具和严格绑定目标的 wardrobe.wear，
却漏装 capability.catalog。Kev 默认隐藏未加载的可选工具，又无法从该 scoped
registry 找到 catalog，结果模型既看不到 wear，也无法发现它。

## 本次修复

1. 受限 Owner registry 安装只读 capability.catalog。目录只返回该 registry，
   换装动作仍经原 exact-target、profile、revision、ownership/availability 和授权检查。
   同时明确最终 decision/reason 契约；不解析自然语言或伪 JSON 来生成 ToolCall。
2. 纠正 r5 的过窄收尾：Moment 完成后只排除已经满足的动态发布，不再强制关闭
   catalog/inspect/wear 等其他合法动作。重复 Moment 仍在执行前拒绝，稳定发布标识仍保留。
3. 同批返回 wardrobe.wear 与 conversation.reply 时，回复必须等待实际穿着回执，
   在下一轮读取刷新后的状态再生成；兼容真实 underscore Tool 名称。
4. 私聊 Writer 即使支持 persona.detail，也保留必要的权威 wearing_state/worn_items；
   其他无关外貌仍按需查询。人设、表达文本和历史原文不被改写。
5. `WardrobeItems` 统一提供实际 worn_items/used_items；内部 HTTP 与浏览器 BFF 使用
   同一结果。读回失败返回错误，不给出成功但缺字段的当前状态。
6. 前端收到缺失/畸形 worn_items 时保留上次确认的值，显示读取失败；真正的空数组
   才清空显示。已穿物品不在当前目录中时，优先使用穿着读面自带的描述。
7. 页面复核另抓到确定遗漏：直接刷新“此刻”页原来根本不调用穿着接口。
   现在生活资料和穿着独立读取，某个生活模块失败也不会阻止穿着读取。
8. 请求按 operation 关联至具体套装，受理、成功、失败在卡片旁显示；后续其他衣柜
   操作不会覆盖该套装失败。保留原请求入口，区分实际执行成功但页面读回失败。
   复用既有 requestDecisionText 本地化，不改写原始理由。衣物选择按钮补充可读名称。
9. 独立复核发现并补齐两条失败分支：已受理但查询断网时保留 requestUnresolved，
   原请求确认终态前禁止重复提交；已确认的业务终态、失败原因和诊断入口不会被
   onSaved 读回异常覆盖。额外读回错误单独展示，重读按钮只读页面结果。
   旧版无 operation 的 checkpoint 重新创建 workspace 后仍保留未决身份，
   查询恢复后从真实 outfit payload 补回卡片关联；未关联卡片前在页面显示查询错误。

## 验证标准与结果

### 真实 PostgreSQL 与原生工具协议

Owner outfit 八种情形：success/rejected/deferred/pretend_success/wrong_target/stale/
no_change/final_failure。Provider 只有在实际收到相应原生 schema 后才允许发调用：

- 首轮必须实际有 scoped capability.catalog，且没有无关 mutation。
- 目录加载后必须实际有 wardrobe.wear，才返回 full + exact frozen member IDs。
- 验证实际 worn table、wardrobe revision、history 和 durable Tool receipt。
- accepted JSON 无真实工具、错误目标、过期版本不写入；拒绝/延迟不被强迫接受。
- no_change 由真实工具给出 handled receipt，不假称产生 mutation。
- 实际换装完成后 final JSON 失败，效果仍完成；同一请求重放不再运行模型。

普通私聊 Generate/Stream 回归包括：目录发现→披露→真实 inspect 标识→wear→reply，
先 Moment 再换装、同批 wear/reply 的两种顺序、工具不可用、同批失败与自主拒绝。
检查 Writer 发请求时数据库已经是最终穿着，成功路径的物理 Writer 上下文确有当前短靴。
不是由 Writer 文字替换穿着。

跨午夜回归读取相同真实穿着，无 mutation 时 IDs 和记录保持。
没有找到数据库“到午夜自动脱衣”的路径。本次修复的是已证实的读回和冷启动缺口，
不能仅由 r6 截图认定数据库曾被清空。

### HTTP 与页面

真实内部 HTTP 和浏览器 HTTP 都读到同一 worn_items / used_items；
在隔离测试库制造 worn table 不可读，两条路径均返回失败，没有伪空列表。
Workspace/SSR 回归覆盖直接 load 此刻、缺字段保留、合法空数组、目录缺项的穿着描述、
受理→失败与另一项衣柜操作后仍能看到原套装错误及诊断入口。
还覆盖畸形 worn_items 元素、状态查询断网后调用原 inventoryClient.action 不增加、
生产 WardrobePage 中换装按钮禁用、从旧 sessionStorage 重建 workspace、
失败/成功终态各自遇到读回异常时保留原结果，以及重读不再次发出 mutation。

浏览器使用最终生产资源 + 受控 HTTP，验证一次失败直接回写套装卡片、
另一次成功后读到短靴、读面缺字段时保留短靴并报错、直接刷新此刻后仍有短靴。
最终生产资源另实际验证：状态查询 503 后主按钮禁用并显示恢复查询原请求；
恢复查询但穿着缺字段时，同时保留原工具未执行原因和单独的读回错误；
只重读结果后原业务错误仍在，再次明确发出新请求成功，冷刷新此刻仍是短靴。
最后一次刷新后的控制台 warning/error 为空。
这只证明页面契约与交互，真实写入证据来自独立 PostgreSQL 测试，不代替用户模型验收。

画面：[失败直接回写套装](evidence/v62-r6/v62-r6-wear-failed.png)、
[不完整读回保留上次穿着](evidence/v62-r6/v62-r6-wear-retained.png)、
[直接刷新此刻读回短靴](evidence/v62-r6/v62-r6-wear-refresh.png)、
[状态未确认时阻止重复提交](evidence/v62-r6/v62-r6-wear-unknown.png)、
[业务原因与读回错误独立展示](evidence/v62-r6/v62-r6-wear-result-readback.png)。

最终检查结果：

| 检查 | 结果 |
| --- | --- |
| PostgreSQL + `go test -race ./internal/core ./internal/httpapi -run '^TestR6\|^TestOwnerLifeRequest\|^TestR5Moment\|^TestPrivateChatPersonaAppearance' -count=1 -v` | PASS；无 SKIP，Owner 八种情形、私聊 Generate/Stream 各七种情形、HTTP 读面和 R5 防重复均实际执行 |
| `pnpm test` | PASS，Web 400 + browser-client 40 + core-client 3，共 443 项 |
| `pnpm typecheck` | PASS |
| `pnpm generate` | PASS，packages 无生成漂移 |
| `pnpm build` | PASS，生产 fixture 排除检查通过 |
| 独立 `VITE_PRODUCT_MODE=mock` 构建 | PASS，输出仅临时目录 |
| `go vet ./...` / `go build ./...` / Go 格式 / `git diff --check` | PASS |
| `go test ./...` | 与基线相同的六项失败，未作为全量通过 |

六项既有失败：TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground、
TestInitializationOwnerBackgroundKeepsStrictScalarTypes、
TestNoArgCatalogSchemaValidationAndCatalogParity、
TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase、
TestBrowserRouteMatrixMatchesOpenAPIArtifact、TestSurfaceRoutesMatchCoreAndMatrix。
分别涉及既有初始化类型、目录 schema、CI gate 与 shared-experiences 路由矩阵，
本轮不扩展修改这些范围。Go 全量默认未启用 SQL fixture，SQL 正向证据以单独
开启隔离 PostgreSQL 的上述 race 回归为准。

后端与前端分别进行了只读独立复核；前端发现的两条失败分支已由主线程修复，
再交另一名只读复核者核验，无剩余可操作发现。最终验证均由主线程完成。

## 本轮范围与后续复测

优先复测“方案库请换上”、普通温和换装请求、发动态后再换装、刷新此刻。
角色可以真实拒绝；成功必须能查到 wardrobe.wear 回执及实际穿着改变。
遇到拒绝、不可穿或超时应看到原因，不能将其伪装成功。

r6 的记忆、初始日程阻塞、世界场地和购物等问题没有在本轮换装提交中标为已关闭。
没有调用用户的真实 Provider、修改其数据库或编造通用衣柜。
附件原始私人内容仅本机复放，没有放入仓库。
