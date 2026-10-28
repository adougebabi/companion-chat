# 当前进度：全站复验仍在实施

活动任务10-28-demo-fullsite-audit，分支codex/demo-fullsite-audit-20261028，基线ef8e1f92。完整范围requirements.md，177行矩阵不是覆盖上限。用户已允许创建任务、直接实施及本地Playwright。主线为UI/UX；不再扩大后端回归，只处理页面暴露的问题。

## 当前实际结果

- Demo48文件、10-22任务108文件hash均与本轮基线一致：evidence/preservation-check.json。Demo只读，未改参考图或源码。
- 已登记F01–F36：消息/动态真实头像与目录元信息、say草稿、快速跳转、目标完整回读与阶段承诺、日程提案、共享宽度/导航/按钮/卡片/Badge/Portal、通知全局计数、图片读取/焦点、管理层级及CSS layer、初始化性格滑块/顺序/手机标签、媒体作用域操作区等。详见fixes.md。
- 普通17页及媒体3子页均有桌面1348×925、手机390×844正常配对截图；初始化5主题也有同内容成对截图。最终初始化稳定对照在evidence/visual/creation-preview-pass11；早期pass10参考toast等待超时明确保留失败，不代替最新证据。
- 真实生产dist的UI受控HTTP验收：45个窄屏空/加载/失败恢复场景、17组外壳/页面操作、管理设置2视口/9类资源编辑/4断点/账户校验共16组、初始化四来源与预览/异常/性格编辑11组、七阶段准备4组、视觉/头像/生成校验9组，另8个动态/人物完整表单宽度与取消检查。具体断言在各JSON；这些计数是场景数，不等于全站全部操作通过。
- 七阶段衣柜失败/恢复、创建未知与恢复未知均保持原body/key；pending不会凭前端计时完成。证据为原creation HTTP fixture，不是实时Core/Temporal证明。
- 性格滑块由真实浏览器键盘.72→.73，完整JSON/生活嵌套/关系/扩展保持，刷新仍.73。原误用profileFields导致所有slider disabled，已修。
- 最近完整Web检查642/642通过，无skip，日志evidence/web-tests-pass17.log且无ReferenceError；F26后62项定向通过，最终全量正在复跑。类型检查、构建、fixture排除和React-only守卫通过，build-pass13。清理测试DOM前等待Radix延后焦点回调，避免先前假绿日志里的document未定义。
- 两轮只读独立检查：共享UI找出重读按钮卸载失焦已修；管理layer/hidden/selected/disabled没有明确回归。子代理仅探索核验，未写代码。

## 证据边界与仍需继续

coverage.json的state_verification记录具体执行，行级状态未因单一场景而整行变通过。normal配对图、受控UI行为、实时业务证据保持独立。accepted-differences.md记录正式DTO不支持的Demo字段/演示能力，不将明显布局问题一概当“契约差异”。

下一批重点：
1. 媒体视觉：候选产生→人工审阅/拒绝/过期、设定图、头像实际保存/历史、上传与生成请求各异常；当前只完成正常读取、定义取消、头像草稿选择、必填门禁和全局/角色目标选择。
2. 创作九类已做正常编辑器实际浏览器检查；继续各类型具体字段编辑/保存/取消、发布/版本/复制归档、应用差异与沙盒详情，并与Demo对应弹窗配对。
3. 世界地点编辑/历史、物品/事件，人物创建/关于你编辑与认识依据，记忆纠正/遗忘/修订，目标证据/核对/状态，日程建议执行、衣柜搭配/换装、日记生成与留言等深层状态还未逐项全量浏览器复核。
4. 初始化renderer未配置、awaiting_review、来源变更冲突、7行自身unknown与深层结构编辑/未应用JSON等继续核验；当前preparation fixture的unknown主要是命令回执未知，不能说已覆盖阶段GET失败未知。
5. 管理诊断/数据/备份及账户初始设置完整流程继续；已测设置二级隔离/草稿保留和丢弃、密码校验/401，不能替代全部管理业务状态。
6. 继续逐张复核已有正常/异常截图，尤其扩展字段密度、语义结构、滚动和对话框；视觉未全面验完不能用640绿或截图数量收尾。

## 实时后端：保留已有证据，不新增无关回归

隔离Docker project fluctlight-demo-audit-20261028，Web/API18480，PG55438。九类真实资源保存/发布/版本/复制归档恢复、配置CAS；真实激活/世界绑定/一次NDJSON回复持久化、6项PG；L04 passed、L17 passed、L10自动断言通过但自治人工项review_required；MinIO上传/权限/元数据/头像专项；正式React组件/controller/provider/client经真实HTTP保存/发布/remount（JSDOM）。见real-*.json、postgres-results.json。没有后端业务/提示词/token策略修改，没有配置或调用Comfy生成图片。

隔离Docker Web已热同步生产dist，保留原runtime配置，未重启Core/PG/工作流。真实浏览器认证既有Owner后读取5页，记录到的API均200；evidence/real-browser/report.json含UI入口指纹。同内容配对图仍为受控HTTP，与真实数据读取证据分开。私有env/Owner密码/令牌/cookie仅在/private/tmp/fluctlight-demo-audit-20261028，不输出或提交。

## 当前活服务与脚本

- 只读Demo：http://127.0.0.1:4179/fluctlight-offline.html
- 正式最新dist+同内容测试HTTP：http://127.0.0.1:4381（fixture，不是实时后端）。修改fixture需重启任务自有该进程；改源码先build。
- 准备阶段fixture4383 wardrobe-failure、4384 recovery-unknown、4385 activation-unknown、4386 pending。各进程独立可变记录，不与原业务服务混用。
- verification/compare-pages.mjs、compare-creation-preview.mjs、verify-workspace-ui.mjs、verify-domain-ui.mjs、verify-read-states.mjs、verify-settings-studio.mjs、verify-creation-ui.mjs、verify-preparation-ui.mjs、verify-visual-ui.mjs。
- 用户已授权独立Playwright；临时依赖/private/tmp/fluctlight-demo-audit-20261028/browser-tools/node_modules，Chrome channel，独立profile。需沙箱外运行，不再尝试已断开的CUA/Pencil连接。

本任务继续in_progress，不能归档或声明全站完成。未推送、未发布。

## 检查点与后续批次
- 已创建本地检查点74b3f5c9；未推送，任务继续in_progress。之后F23–F26为新一批未提交修复。
- F23：实际目标详情在畸形证据结果中崩溃，已加采用前验证/原读取恢复，新增React回归通过。parity记录端点同时改为正确列表DTO，评估原因不再增加Demo未提供的文案。
- F24/F25：地点整组跨列、根作用域与布局同节点匹配、模态遮罩、普通输入值字重/文本选择颜色。profile/wardrobe/goals/world/people详情与编辑双视口成对证据在dialogs-pass17。
- 独立核验纠正了媒体分列判断：已有单列保护，后补的lp-wide无视觉作用，已撤掉并纠正清单。
- F26：确认动态发布/建议与人物新建/编辑/接触/应用完整组跨列缺失。修前桌面251.5/521px、手机138.5/295px；修后536/536或806/806，手机310/310，flex gap20。8个实际浏览器场景均通过，无写请求；form-groups/report-before.json和report-after.json。
- 下一步继续未覆盖的保存/版本/应用/错误恢复等深层流程，不能把这些几何与取消检查当全业务通过。


## 本次继续批次：F27–F36与深层UI（最新）

- 当前仍是codex/demo-fullsite-audit-20261028，活动任务in_progress。已有本地检查点74b3f5c9；F23–F36形成第二个本地检查点，准确提交见git log；任务继续实施。原Demo48文件和旧10-22任务108文件hash再次检查均未变。
- 最新正常配对：shared-pass31为16页×桌面/手机，全部无浏览器异常、h1坐标与Demo一致。主线程亲看Profile/Chat/Now/Wardrobe、Goal/Memory/Diary/Schedule等配对图；未把截图生成自动当视觉通过。memory分类测试映射纠正后另有memory-mapped-pass32。日记目录溢出修复已在diary-pass33双视口复拍，摘要省略恢复。
- F27/F28：原生四件方案换装作为Demo合法场景前置，正式HTTP快照同四件；穿着中点分隔、明确色词示意色、聊天生活进行中文案已核验。示意图无明确颜色时仍中性，不写Core颜色。
- F29：突出真实当前portrait_text，核对profile_id，错人格正文不冒称当前画像；12项Profile测试通过。完整档案四维只作为原Demo种子测试数据，不在生产回填。
- F30/F31：手机Profile18px gap、tag/status/progress生效颜色、Card默认行框、timeline p、chat meta/caption、穿着标签均修正。after-pass31 computed中Profile hero的桌面/手机宽高/Y一致，手机Now timeline几何一致；桌面Now内栏仍有2px宽差，Profile下方正式字段与披露有密度差异等，继续核验。
- F32：实际浏览器发现未知纠正回执在明确确认最新基线后取消仍回到旧记忆；已同步已校验当前对象到详情/目录/来源版本。F33说明块跨列表单修复。memory-deep/report-pass26.json共12场景通过，report-pass29.json再复验4个错误恢复状态和说明宽度：搜索/分类/历史来源/焦点、校验与取消、纠正/遗忘/补偿恢复、反思反馈至原请求终态、自我复核/停用/历史恢复、未知回执与受理后GET失败。只证明受控HTTP，不证明真实模型/PG治理。
- F34：实际Studio发布后版本列表不刷新，已将published_version纳入读scope。studio-deep/report-after-pass29.json世界/角色模板×两视口4组通过，含草稿/发布分开、发布取消、历史旧版、副本取消/创建、应用预览取消；世界另核对明确应用回执。模板解析和实际运行应用结果不由此fixture证明。
- World深层world-deep/report-pass31.json共6组通过：地点取消/保存/复杂字段保留/基准恢复/说明校验/刷新重新进入；物品取得范围关闭/开启与聊天草稿不发送；事件暂停/开启/必填/queued与applied/生活事件详情。早期pass29为定位器忽略数量标签、pass30漏刷新后重新进入世界，均为验收脚本失配，未据此改产品。
- F35：900px标题顶对齐、目标刷新归入搜索框后手机标题不再多一行、记忆卡正文400字重。F36：日记目录摘要受限宽度保证ellipsis，不修改正文。
- 质量：Web全量pass30为642/642、0skip；Memory12、Studio16、Goal11定向通过；typecheck-pass31/build-pass31和生产fixture/React-only守卫通过。F36仅CSS，最新build-pass33成功且两项生产守卫通过。独立只读复核Memory/Studio/Goal的scope、草稿与无重复mutation未发现明确问题。
- 最新UI静态文件已同步现有隔离Docker Web并恢复其runtime-config，未重启Core/PG/Worker/Temporal。real-browser/report.json最新5页读取通过、记录API均200，无发送/生成/新激活。
- coverage更新为94行有具体浏览器证据（177行初始矩阵）；每个行级仍可能有未验状态，不能用94/177换算完成率。新证据按size分别登记state_verification。

## 接下来继续

1. 创作沙盒的建议审阅/采用/丢弃/素材、9类编辑完整字段与每类错误恢复，模板解析需要正确的测试产物或明确真实依赖。
2. 日程建议采纳/未知、目标核对/状态/证据、衣柜搭配/换装、日记生成/留言及关联经历深层状态；继续修正常态视觉中发现的细节。
3. 视觉候选产生/拒绝/过期、头像保存/历史、上传生成异常；初始化renderer未配置/awaiting_review/阶段GET未知。
4. 世界current_definition、失败执行恢复、更多分页与空间连接；人物关于你保存、关系依据；管理诊断/数据/备份等未测状态。
5. 每批更新覆盖、修复和视觉证据。保留受控/真实数据证据边界，不扩大后端回归，不配置Comfy。

当前额外服务：4387 mutable growth正常（session88364），4388 unknown（session29616，当前未用于新验收）；4389 mutable studio（session22003，带私有audit路径）；4390 mutable world（session50742）。4381同内容parity已重启为session13730，含记忆经历/知识正确映射。每次改fixture要重启对应任务进程；改生产代码先build。


## 第三个继续批次（第二检查点之后）

- 第二本地检查点23becf23已保存F23–F36，未推送。当前新增F37（沙盒discarded文案）和F38（日程冲突复核可读摘要），尚未提交。
- studio-sandbox/report-pass35.json4/4通过；Diary report-consolidated.json12/12通过；Schedule report-pass39.json12/12通过，F38后item-conflict另在report-pass43.json2/2通过（可读原建议、完整协议折叠、明确重提保留原key）。Media report-consolidated.json16/16通过，覆盖定义保存、候选接受/重生成与设定图读取、头像保存/历史/未知、图片请求完成/失败/未知；均受控HTTP，无模型身份判断或Comfy证明。
- 真实上传UI evidence/real-ui-upload/report.json2/2通过：既有隔离Owner，桌面正常上传并读原PNG，手机实际POST保存后丢回执→刷新→by-key查询恢复，不重发bytes；实际filename/sha256/size/origin/readable一致，详情图片可读。没有新增后台审计/配置/重启，仅实际上传UI操作。
- 新测试工具问题已区分：Diary留言fixture补fid过滤（旧样本ID相同），过快ready由完成GET传输门控观察pending且不改服务结果，重复正文定位器已限定正文；Schedule初始失败为对隐藏背景标题/空态词定位失配。失败证据保留，不当产品修复。
- Web全量pass42为642/642、0skip；F38新增一项Schedule测试，14项Schedule定向pass43通过，当前总数应643，待最后全量。typecheck/build43通过及两项生产守卫。Studio16项pass35通过。
- coverage现101行有具体浏览器证据；新state不等于177行全部验完。还未以此归档或报告全站完成。
- 当前在新4398 creation=normal（session96831）测试初始化独立GET未知/待审阅/图片失败。verification/verify-preparation-read-ui.mjs，初次before44只跑awaiting-review（执行session见工具），目标是先确认已知状态是否有可执行的人类评审说明。不要改Demo或配置图片服务。
- 新服务：4391 diary normal重载session42429；4392 schedule normal88075；4393 rejected23075；4394 conflict最新67217；4395 conflict最新79491（F38已用过一次冲突，重测该场景需重启）；4396 media normal44773；4397 generation-failure71463。旧其它服务保持。全部只读探子已终态。
- 真实18480 Web最近同步为shared-pass31构建，F37/F38之后还未同步；若给用户查看最新，先仅docker cp前端dist并恢复私有runtime-config，不重启Core/PG/Worker。


## 本轮最新状态（至pass52，接着这里继续）

- F37–F40已经实施，尚未第三次提交；第二检查点仍23becf23。F38日程原建议可读摘要、F39待审阅说明/入口、F40共用portal主题/窄屏Sheet/普通导入主色/WorkflowRecovery共用Dialog与focus均已复验。
- Admin合并report-consolidated.json为22/22通过：来源/阶段/记录/信息、检查点与取消、实际控制表单POST参数及accepted不冒称完成、calls/readiness隔离、定义包校验/导入取消、备份只读与恢复取消、修订/回档边界、空/加载/失败重读，以及被拒绝复核Esc返回真正入口。全部受控HTTP，无真实维护操作。
- preparation-read合并20/20通过：8类独立GET未知、awaiting_review和image-failure×2。世界初次读取失败会中断绑定，重读后仍需要原world行恢复；已实测继续原准备且activate仅1次。旧pass45 world失败是测试误期望仅GET能完成未绑定世界，未当产品缺陷。F39只说明已知状态，不造图片，不降级已确认可读图。
- Wardrobe report-pass48.json6/6通过：新增/取消/借用和可用状态保存；存方案不穿着、取消方案编辑；显式换装pending→确认，Now和Chat同三件；未知服装原payload/key重试、新草稿保持。desktop4399和mobile4401独立首次换装样本，避免fixture计数混淆。
- Web全量pass51为644/644、0skip；类型/构建pass52成功，生产fixture排除和React-only374模块守卫通过。Creation22、Schedule14、Studio16、Portal11、旧端口/controls13定向均通过。最后仅静态复核指出的一行review入口ref已修，浏览器pass52两视口焦点通过。
- 独立静态review_portal_theme_batch未发现Owner/原body-key/accepted回读/忙时保护/Sheet几何明确回归；其轻微焦点意见已修。所有子代理已终态。
- 已亲看本批关键手机截图：沙盒确认/丢弃保留素材、日记pending、日程复核、媒体评审、真实上传回执恢复、初始化评审入口、衣柜确认、管理导入与任务恢复修前/修后。生成截图不等于全站视觉验收。
- 覆盖矩阵补上Admin与实际已执行但旧映射漏掉的JSON导入/日常世界/收藏/自我治理；3条纯Demo说明/模拟/重置入口明确不适用。全部其它行继续按具体state记录，未把有一次证据就整行改通过。
- Demo48文件、旧10-22的108文件再次hash未变。只有本任务、Web、对应前端spec发生修改。
- 真实18480 Web仍旧同步版本，下一步可仅docker cp最新dist及恢复私有runtime-config，保持Core/PG/Worker不动；真实5页read和真实上传均已有证据。勿把4381/439x/440x样本当实时后端。

新服务：4398 creation normal96831；4399 wardrobe desktop40394；4400 management normal43791；4401 wardrobe mobile81372。旧4391 diary42429、4392 schedule88075、4393 rejected23075、4394 conflict67217/4395 conflict79491（已用一次）、4396 media44773、4397失败71463仍可用。

下一批依完整矩阵继续，不新建任务：优先shell角色切换/空目录/账户、Profile人格/生活/来源/四类编辑/治理、People关于你保存/认识/依据/关系、Feed互动隐藏/发布/建议、Goal详情保存/状态。还有衣柜标签管理、世界设定/分类、Studio其它7类保存和跨初始化、媒体metadata/grant/lifecycle、初始化复杂编辑/来源冲突/终态导航/快照、正式额外工具等尚未逐项完整验收。先做第三个本地检查点再继续，任务in_progress，不能归档或声称全站完成。


### pass52收拢与第三检查点
- Admin22/22合并通过，最新overlay输入13px/400、根14px、窄屏Sheet可用宽度、普通导入primary、可见焦点、原表单关联、accepted不冒称完成均有实际断言。
- F40独立复核无协议/Owner/Sheet几何明确回归；唯一焦点小缺口已修并实际两视口通过。完整Web644/644、0skip，typecheck/build52及生产守卫通过。
- 122/177行具有具体浏览器证据，3条纯Demo模拟/种子重置NA；这不是完成率，行级仍有未验状态。视觉索引已指向shared-pass31、memory-mapped-pass32、diary-pass33最新配对，保留发现问题/待逐张完验，不制造全通过。
- 最新dist已经静态热同步隔离18480并恢复原runtime-config，最新五页只读通过、记录API均200、无发送/生成/新激活。第三个本地检查点准确hash见git log，不推送、不归档，继续上述剩余矩阵。


## 第三检查点后：档案/关系/动态/目标UI（当前执行）
- 第三本地检查点75ec4e7a已保存F37–F40及之前证据，未推送。此后暂无新生产修复，新增验证脚本/证据仍未提交。
- Profile/People report-consolidated.json18/18通过（pass54加pass55）：身份提议接受/历史恢复、能力接受/已交付分别复核；人格/生活/来源读取；expression/life/more三组取消→继续编辑→放弃→保存→重开；人格选择/原因/勾选/真实请求终态；关于你取消确认/保存/修订，自填资料与已有事实分开；关系依据和sunny指标。
- 这批使用夹具已知默认fid，Profile桌面4402、手机4406独立；People4403组合PROFILE+PEOPLE供有效人格读面，但不在此服务编辑Profile。初始pass53的别名角色缺seed名字/时区导致编辑正确阻止提交，且多处控件类型/确认文案假设错误；这些是验收场景失配，没有改产品来迁就。
- 服务4402 profile normal当前59326，4406 profile normal38054，4403 profile+people94477。默认身份提议已在桌面/手机各处理一次，重测其v4预置基线应重启对应服务。
- Feed/Goals桌面4404（25478）、手机4405（45967）正常。verification/verify-feed-goals-ui.mjs初轮pass56的suggest、Goal read/state/edit两视口已通过；失败主要是发布自动开详情、列表分页、相同条件不增criteria_version、恢复未等busy结束等脚本期望，未判产品缺陷。已调整后执行pass57精选case，当前运行session69989，需取结果。Feed全局状态已含本轮新动态，别假定post-one仍在首页。
- 下一步取pass57、检查失败真因与截图；补矩阵Profile/People与Feed/Goal最新state，然后再继续完整矩阵其它未验项。生产状态仍in_progress，不能结束/归档/声称全站完成。

## 2026-10-29 继续批次 F41–F43
- F41目标打开详情的证据/历史刷新、F42命令旧快照只读恢复：pass63双视口4场景通过，Goal13项通过。
- F43创建同页resume在URL改变后仍持有新草稿，pass65两视口复现；App按path/query重建控制器，pass66两视口恢复原历史且激活仅一次。
- 创建补验report-consolidated共16场景通过：精确/错误世界版本、结构编辑类型校验和扩展保留、世界源冲突重审、创建快照导出与完成导航、拒绝副本再分析、Studio世界和人物模板精确发布版本带入。只证明受控HTTP UI，不证明真实模型/激活。
- pass66 Web646/646、0skip，类型检查/构建及374模块生产守卫通过。没有改Demo或后端。任务仍in_progress；视觉逐张完验和剩余深层入口继续。
