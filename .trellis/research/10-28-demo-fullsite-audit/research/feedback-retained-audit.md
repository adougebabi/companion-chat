# 保留领域 renderer / proto 反馈调用点审计

日期：2026-10-29。只读静态审计；未修改产品代码、启动服务或运行截图。伴随清单 `feedback-retained-callsites.json` 扫描 134 个领域/proto JS 文件、DomainPrimitives/OverlayHost 两座桥及 App 的 configureToast 注册，共 137 文件、100 个命中文件、516 个反馈相关源码行。清单保留完整源码行（长模板中的多个条件分支没有裁短），分类是该行各分支的候选并集，不能把一行的所有分类当成同一事件。普通字段名称和说明段落不是缺陷；catch 也不等于一定展示故障。

## 实际所有者与在用边界

- 原命令、草稿、key/body、CAS 及业务结果由各 `v3-domain` controller / proto component 及其注入 provider 持有。`DomainPrimitives` 只映射显式 `data-domain-ui` 标记的外观和交互；不要在桥里推断中文业务状态。
- `apps/web/src/react-app/shared/DomainPrimitives.tsx:48-57` 的 PresentationPort 把 alert 包装为 Alert；`:50-52` 根据初始 role / banner err 计算 destructive/default，`:53` 剥去旧 banner/info/warn/ok token。因此 warn/ok/info 并没有六态语义，桥也无法仅凭文本区分“有效草稿”“故障恢复”。
- `apps/web/src/react-app/shared/OverlayHost.tsx:10` 将保留 DOM 装入 React Sheet/Dialog，再挂 DomainPrimitives；`:20-23` 在 Owner 变化/卸载关闭浮层。关闭只影响浮层，持久命令仍由命令 owner 管理。
- `apps/web/src/v3/proto/components/toast.js:1-3` 仅转发 `(text,type)`。实际 toast owner 是 `apps/web/src/react-app/App.tsx:79`：err→error，ok/success→success，其余普通 toast。没有事件 key、scope 或去重身份。
- 不要把所有旧文件算作当前正式页面。`apps/web/src/approved-ui/feature-routes.mjs:9-22` 的 Studio 编辑/预览/素材、Media、Governance、Self 已转 React native；相应旧 `v3-domain/studio.js`、`visual.js`、`media.js`、`history.js` 等仍有调用点，但该文件存在不足以证明默认路由会执行。当前明确保留的 route 包括该文件 `:23-31`（memory/reflection/goal/planning/schedule/diary），`:36-45`（世界/地点/事件/物品），`:51,54,56-63`（关系修订、请求详情、诊断、数据工具）。React 普通页面由其他审计者核验，本报告不重复。

## 已确定的实现缺陷（静态可证明；视觉仍需 desktop / 390px）

| ID | 位置 / 实际 owner | 触发与关键原文 | 当前真实类型 / 缺陷 | 修正应保留的真实动作与恢复规则 |
|---|---|---|---|---|
| RF-D1 | `proto/components/versioned-fields.js:15,21,24`，mountVersionedFields | 同一个 `data-error role="alert"` 被写入“原提交处理中”“已持久保存”“版本已变化”；成功另有 toast“定义已保存” | progress / success / action-required 都被 destructive alert；成功还同时局部+toast | owner 应显式传递状态和对应语义；成功选一处反馈，进度局部；真实版本冲突继续保留草稿/核对最新版本。该组件旧视觉编辑接入见 `v3-domain/visual.js:20`，当前默认媒体路由已 native，需确认残留宿主是否在用 |
| RF-D2 | `proto/components/governance-command.js:18,26-29`，governanceCommand | “原提交正在处理”“原提交结果待核对”“原提交已保存，等待期间的新理由已保留”均写入固定 role alert | progress / action-required / success 共享 destructive 区 | 不可把未知受理伪装成功；继续重试原 key/body、冲突才可采用新基线；需要语义 owner 改造，不能统一在桥默认全部变灰 |
| RF-D3 | `proto/components/durable-command-drawer.js:26,43`，durableCommandDrawer | `data-command-error role="alert"` 显示“原操作已保存；等待期间的新输入仍保留。” | success + information 被当 error | 保留新输入，accepted GET 读回后才移除 accepted checkpoint；局部成功/信息即可 |
| RF-D4 | `proto/components/durable-command-drawer.js:28-29,33,35,48`，durableCommandDrawer | `accepted()` 的 getItem/JSON.parse/版本校验抛错；初始 `paint()`、send 中 accepted()/paint() 在 try 外 | 草稿/回执读取 error 没有可用的局部恢复态；初始 paint 失败时 drawer 已打开但按钮状态未完成（send/retry 已绑定） | 区分存储读取、JSON/版本不兼容、原回执未知；阻断新提交，保留记录，给真实重新读取/原结果核对。禁止删存储来让表单恢复。不是建议清除旧 key |
| RF-D5 | `proto/components/durable-command-drawer.js:43-46`，durableCommandDrawer | 失败后保留 data-command-error；随后重新读取成功且新输入未变会关闭，若新输入改变覆盖为成功；但正常重新尝试开始不清旧错误 | progress 期间陈旧 error 继续展示，视觉状态不随下一次真实读回开始更新 | 保留故障记录/诊断，同时当前进度明确“正在核对原结果”；成功更新当前状态，不改变原业务任务 |
| RF-D6 | `proto/components/versioned-fields.js:21-23`，mountVersionedFields | settled 的无 failure/saved/pending/conflict 分支不清 data-error；采用基线后写“已明确采用新基线，草稿保持。”再调用 settled() | 已恢复后陈旧 error 或旧成功会残留；恢复信息仍 role alert | 每次完整派生当前 feedback，恢复后去掉过期故障；这不等于清业务草稿 |
| RF-D7 | `v3-domain/goal-editor.js:21`，renderRealGoalEditor/read | 初次读取失败只有错误 banner +“返回目标列表”，没有重新读取按钮 | error，缺真实就地重试（当前该 route 确实保留） | 给 read 的真实重试，保留创建/编辑草稿。`:37` 保存409后的二次 GET 也在 catch 内直接 await，GET 再失败会抛出且失去局部故障更新，需单独覆盖 |
| RF-D8 | `proto/components/media-metadata.js:22`、`display-avatar.js:27` | 直接 `${error.message}${error.code?'（'+error.code+'）':''}` | error / 已受理读回 warning，后端原码直接暴露主反馈；没有可展开诊断容器 | 用用户摘要+明确原始诊断披露；保留错误原文。旧组件默认 route 已 native，先确认具体复用宿主 |
| RF-D9 | `proto/components/visual-review.js:15`、`governance-command.js:44`、`versioned-fields.js:24` | progress/saved 文案写 role alert；“操作已提交；重新读取失败：${error.message}” toast 展示原始 message | progress/success/error 混用；readback 故障可能只剩短 toast，无法长期核对结果 | 持久状态到对应详情，短 toast 只提醒；保存完成与 GET 失败分开、原 key/body 不变。原始诊断进 disclosure |

D4 是真实代码路径缺口，但尚未注入破损存储在 UI 复现；“哪些按钮当时仍可点击”需专门测试确认。D1/D2/D3 的视觉具体颜色取决当前主题；固定 role=alert 和桥 destructive 判定是确定事实。

## 其他需复现 / 不应直接定为 bug

1. `proto/components/async-page.js:36` 的 requestReceipt 将 pending/processing 以外都视作 observation terminal，accepted / awaiting_goal / deferred 停止自动轮询，且 `:42` 调 onTerminal。`:27` 文案明确 accepted 是“她已答应，等待实际行动”，没有声称完成；不能直接判 accepted 业务完成 bug。应验证所有 callback 是否仅刷新 read model，并且实际行动/任务能从详情或通知继续观察；未完成状态要 progress 或 action-required。原请求详情与任务链接在 `:39` 保留，手动刷新在 `:53`。
2. `async-page.js:39,47-48`：tasks_state=failed 的 warning 与读失败 error 可并存，分别是关联任务读取和原请求轮询；不应粗暴去掉其一。达150次以后 finally 调无参数 paint() 会清掉最后一次失败摘要（原状态/暂停说明还在），需复现最后一次恰失败的场景，避免把仍未恢复的读取问题静默清掉。
3. `cursor-records.js:20-29`：后续读取失败保留上次 page，只有一块 data-panel-failure，真实 retry 重试 requestedAt；成功 paint 完整替换清除过期故障。已满足去重/恢复基本要求。恢复时没有明显“正在更新上次快照”的反馈、旧页无时间标记，可作为 warning/progress 体验核验，而非立即断言数据错误。`:12` scope 按 Owner+scope，最多40条内存缓存；不要为UX清缓存或换分页协议。
4. `async-page.js:15-18`：整页 reload 会 dispose 并用 skeleton 替换旧页，读失败可重新读取，backHref 仅调用方传入才有；此 owner 支持 epoch 保护。页面级 GET 失败不是工作区全局故障，不应接全局顶部警报。
5. `v3-domain/history.js:21,31` / `npc-management.js:96,98,120,130` / `proto/components/resource-lifecycle.js:34,62` / `runtime-world-framework.js:30,40` / `world-framework-generator.js:45-47` / `initialization-progress.js:26` 等有 raw `.message` 用户通路。完整候选均在 JSON 的 raw-error-candidate flags；其中 history 等已被 native route 替代，需调用图确认后再投入修改。
6. `proto/components/states.js:16-28` 含演示“快照10-15 09:40”“4项3项成功”“重试（幂等）”只切 data-demo-state。属于 prototype 支持；需验证正式 retained host 是否还能 render stateView，不能把静态演示按钮当成真实恢复。不要修改业务任务以配合演示态。
7. toast 总线和 App 适配无身份去重，但“无API去重”并不意味着每次实际发生重复。确定重复示例为 versioned-fields 成功局部文案+toast；其余要按事件逐页确认，不能全站压掉故障 toast。
8. `proto/components/media-assets.js:24-28` 媒体读取错误在原预览处替换为 role alert，明确“重新查询素材状态或检查存储连接”，但该片段自身没有重试按钮，需要确认所属 MediaPage/retained detail 有可达的查询控件。

## 六类真实状态及 N/A 边界

| 类型 | 已核实调用点 / 当前动作 |
|---|---|
| information | durable-command-drawer:29 的无原提交说明；resource-lifecycle:40 的已归档解释；cursor-records:14 的真实无记录。非阻断说明留对应入口，不应全局横幅 |
| action-required | governance-command:29 原提交未知/版本拒绝；goal-planning:45 原提交待核对与新基线；世界/目标 conflict 面板。真实原提交重试/核对和明确采用新版本，不能新造请求 |
| warning | requestReceipt:39 tasks_state failed；cursor-records:24-27 保留旧页但读取失败；partial/readback failure；与业务失败分开 |
| error | asyncPage:18 初读失败；goal-editor:21；原命令存储/JSON失败；media-assets:25 字节读取失败。保留故障强调，近操作/预览展示 |
| success | durable:43 原操作保存且新输入保留；versioned-fields:21 持久保存；普通短toast。accepted request 不归入 completed success |
| progress | requestReceipt:27-39 pending/processing/accepted后等待实际行动；durable 的原结果核对；async/cursor skeleton；不应 destructive alert |

读者组件不会创建业务任务，因此“提交结果未知恢复”在 cursor reader 本身 N/A；它只读原页/游标。toast 总线本身不拥有后台状态/草稿，因此不能要求关闭 toast 终止任务；任务恢复必须由原 controller owner 做。OverlayHost 不拥有业务六态，只负责容器、关闭及焦点。

## 可复用测试 / fixture（此次未运行）

- `apps/web/test/ux-settings-support.test.mjs:29-45`：cursor 初次失败→重试、后续失败保留成功页；asyncPage 真实 backHref 与 retry；`:65` originalOnly 无 pending 不创建空命令。可扩展六态语义、坏存储不放开新提交、读取恢复清旧提示。
- `apps/web/test/demo-replica-management.test.mjs:28-76`：真实 OverlayHost + durableCommandDrawer fixture；accepted读回失败→关闭重开→只GET、迟到回执、同Owner重登录、等待中新输入保留。可加 malformed accepted JSON、storage.getItem 抛错及成功/进度 role 测试。
- `apps/web/test/r11-domain-primitives.test.mjs` / `r2-presentation.test.mjs`：DomainPrimitives React harness；可加入 default/info/warn/success/error/progress 等显式语义映射、可变状态同节点更新（不要只测CSS换色）。
- `apps/web/test/r11-overlays.test.mjs` / `demo-replica-studio.test.mjs`：浮层关闭和焦点恢复；覆盖390px按钮可达须用现有浏览器验收，不由静态审计断言。
- `apps/web/test/ux-recovery-flows.test.mjs:64`、`request-reason-diary.test.mjs:69`：requestReceipt 原请求查询与 reason 来源；可验证 accepted 不显示completed、轮询暂停保留故障、onTerminal callback 不新建命令。
- `apps/web/test/v3-provider-async.test.mjs:34`：asyncPage stale response；`v3-inventory-submission.test.mjs:37` requestReceipt stub 可复用原建议成功与读回分流。

所有建议只改反馈语义、位置、真实恢复入口、诊断披露和当前状态更新。保留原请求/草稿/storage 内容、Owner/fid/world/epoch、CAS、权限、accepted后GET以及既有业务能力；不隐藏故障、不重置存储、不更改模型/后端。
