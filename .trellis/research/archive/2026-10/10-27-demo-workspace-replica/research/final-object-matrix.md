# 最终对象交互与跨页读回矩阵（2026-10-28）

本矩阵补充 `final-route-inventory.json` 的55种静态路由解析。所有浏览器样本均为正式生产dist + loopback隔离API，不是live Core/数据库/模型/存储验收。`F=fluctlight-fixture-one`，Owner为human-fixture-owner。证据路径均相对于本任务目录。

## 保留的对象工具抽验

|实际对象及路由|本轮实际操作与返回|失败/恢复依据|证据|
|---|---|---|---|
|记忆 `F-memory-one`；`/c/F/growth/memory/F-memory-one`|4375读取v2正文/来源/修订；“记错了”输入后取消，原正文未改；来源returnTo回growth?tab=memories&q=雨，搜索框与一条结果均恢复|demo-replica-memory：CAS取消、unknown current-object review、acknowledged GET-only、原revision来源与失败追加cursor；既有浏览器unknown审计|final/memory-detail-cancel-desktop.png、memory-source-filter-desktop.png；memory/governance-readback.json、cas-review-audit.json|
|反思 `F-reflection-one`；`/c/F/growth/reflection/F-reflection-one`|4375读取已应用/no_change原提案；提交“认同”反馈返回实际growth-feedback-1，不把认同当成长。原详情有明确反思时间线链接|demo-replica-memory的reflection feedback原提案/实际应用状态/OwnerRequest观察；初始失败用共享asyncPage重读|final/reflection-feedback-request-desktop.png；memory/reflection-desktop.png、feedback-pending-desktop.png|
|规划 `plan-fixture`、目标集v1；`/c/F/life/planning`|4375实际读到目标集与历史；修改原因“最终验收：保留未提交的规划草稿”刷新仍在；未点保存/立即规划；“目标列表”回正式goals目录|goal-planning保留stale基线草稿，显式采用；asyncPage初始重读；既有目标创建unknown同body/key与编辑CAS回归。此处未新触发规划任务|final/planning-draft-reload-desktop.png；goals/request-attempts.json。显式列表链接回根目录，不声称带任意目标筛选|
|衣物 `clothing-0`；`/c/F/life/wardrobe/item/clothing-0`|4375读取实际物品v1与库存；编辑描述后Escape，物品未改、焦点回编辑登记；发现返回q未消费并修复。4376复验返回tab=clothes&q=上衣，搜索框=上衣，仅青绿棉质上衣一张卡|demo-replica-wardrobe新增真实controller/view路由搜索与plans别名回归；原archive/cursor/unknown与v3-inventory-submission迟到回执回归保留|final/inventory-detail-cancel-desktop.png、inventory-source-filter-fixed-desktop.png；wardrobe/request-attempts.json。旧inventory-source-return图是修复前观察，不能作最终筛选证明|
|日记 `diary-fixture-one`；`/c/F/life/diary/diary-fixture-one`|4375实际读主观正文；输入Owner批注后Escape，服务端仍无批注，页面明确显示未保存草稿；关闭不会把草稿当已保存|demo-replica-diary取消/正文不变、409输入保留、acknowledged GET-only、独立分页cursor回归；原未知注记同body/key审计|final/diary-detail-cancel-desktop.png；diary/request-attempts.json|
|共同经历 `shared-fixture-message` / `conversation-fixture`；`/c/F/life/shared?view=records`|4375发现chat忽略来源ID并修复；4376读取原会话目录、精确message GET与sequence+1窗口，弹窗标出“来源消息/一起分享一段正在读的故事”；Escape回shared?view=records。390px弹窗356、document390|demo-replica-chat新增原anchor失败重试、更后目录页不替换当前会话、外来conversation拒绝；既有历史cursor/晚回复/草稿回归；source-request-audit无/select或/turn|final/shared-exact-message-{desktop,mobile}.png、shared-return-{desktop,mobile}.png、source-request-audit.json|
|地点 `F-room` / 世界 `fixture-world`；`/c/F/worlds/fixture-world/place/F-room/history`|4375独立路由读到首次保存、地点v1/默认基准v1与来源。页内明确历史不代表当前位置；有对象修订目录/当前世界目录链接|demo-replica-world地点CAS、接受后GET失败不重发、双版本默认恢复且保留引用；既有浏览器历史与实际恢复证据|final/place-history-desktop.png；world/place-history-desktop.png、place-restored-desktop.png、world-readback.json。目录链接是明确根目录，不冒称任意筛选恢复|
|诊断 `model-runs-fixture`；`/diag/record/model-runs/model-runs-fixture`|4374独立详情读实际模型失败记录及Prompt/Response入口；测试场景failure→点击实际重读→“读取失败”；恢复normal→同一“重新读取”回原记录；“返回分区”回diag/model-runs|本轮浏览器真实GET失败恢复；共享asyncPage、诊断applied-filter/append回归与group16独立检查|final/diagnostic-record-desktop.png、diagnostic-read-failure-desktop.png、diagnostic-read-recovered-desktop.png。显式链接回诊断分区，不宣称保留任意历史筛选|
|原请求 `growth-feedback-1`；`/diag/request/growth-feedback-1`|由本轮反思实际返回ID进入独立详情；展开原始内容，准确保留reflection_id、正文、stance=agree及实际结果，不重新提交反馈|request-reason-diary原始记录保持、ux-recovery-flows读失败保留状态/Owner切换；无重新提交动作|final/request-original-desktop.png、cross-page-readback.json；返回收件箱/全部待处理/角色此刻均为明确目的地|

原生媒体/人物/动态/创作/治理等对象已经由各组实现和浏览器证据覆盖，见 final-completion-audit 的逐组表。chat/history共享读者及未发送草稿已有 `evidence/chat/check-history-*`，本轮补的是普通chat的conversation/message来源参数。没有把55个正则解析成功当作55种对象所有分支的浏览器证明。

## 正面跨页更新链

服务4375组合WORLD/PROFILE/MEDIA/GROWTH/GOALS/DIARY/WARDROBE_WORLD正式dist。修复的只是测试dispatcher顺序：world仍先处理其世界端点，wardrobeWorld在profile静态库存fallback之前；media的externalWorld/externalInventory继续生效。夹具未进入生产包。4376是修正原消息夹具后的独立复验，不覆盖4375已保存状态。

|步骤|实际读回与浏览器结果|证据|
|---|---|---|
|A保存独立显示头像|选replica-unused，“保存并读回”；avatar revision2，asset revision3/version-replica-unused/SHA a×64，canonical保持独立|final/avatar-saved-desktop.png、cross-page-readback.json；此前media/avatar-saved-desktop.png及runtime-readback.json|
|保存两件方案→提交换装|仅clothing-0/clothing-1，plan-1；OwnerRequest request-wardrobe-fixture pending时仍显示原六件，completed后通过实际wardrobe GET变成两件。保存方案与换装分离|final/wear-pending-desktop.png、wear-completed-desktop.png、cross-page-readback.json；此前wardrobe/wear-result-desktop.png|
|A聊天/此刻/档案|三页均青绿棉质上衣、宽松长裤；聊天与档案、侧栏img实际src=/api/media/replica-unused且complete/naturalWidth成立；地点家中/阅读/平静与10:00自由安排一致|final/a-chat-updated-desktop.png、a-now-updated-desktop.png、a-profile-wearing-desktop.png；cross-page-readback.json|
|切B|实际profile/chat标题季川，白色衬衫profile-shirt；未带入A的两件穿着或replica-unused显示选择。B当时目录没有已投影头像，UI回退，不把它写成A头像或B新头像保存|final/b-profile-isolated-desktop.png、b-chat-isolated-desktop.png；cross-page-readback.json的B selection=null/mode=canonical|
|切回A|原头像与两件实际穿着仍在；390px聊天保持同一读回|final/a-return-chat-{desktop,mobile}.png|

Owner隔离采用各页既有Owner/epoch/冻结原命令与本轮logout→same-Owner reentry回归证明。此轮浏览器验证同Owner两个角色，未冒充真实第二Owner数据库联调。位置/日程沿用先前world/schedule协议证据和当前同快照一致性，没有以编辑世界设定或提交日程建议声称角色已实际移动/执行。
