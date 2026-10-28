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
