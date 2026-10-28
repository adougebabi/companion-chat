# 当前进度：全站复验仍在实施

活动任务10-28-demo-fullsite-audit，分支codex/demo-fullsite-audit-20261028，基线ef8e1f92。完整范围requirements.md，177行矩阵不是覆盖上限。用户已允许创建任务、直接实施及本地Playwright。主线为UI/UX；不再扩大后端回归，只处理页面暴露的问题。

## 当前实际结果

- Demo48文件、10-22任务108文件hash均与本轮基线一致：evidence/preservation-check.json。Demo只读，未改参考图或源码。
- 已登记F01–F22：消息/动态真实头像与目录元信息、say草稿、快速跳转、目标完整回读与阶段承诺、日程提案、共享宽度/导航/按钮/卡片/Badge/Portal、通知全局计数、图片读取/焦点、管理层级及CSS layer、初始化性格滑块/顺序/手机标签、媒体作用域操作区等。详见fixes.md。
- 普通17页及媒体3子页均有桌面1348×925、手机390×844正常配对截图；初始化5主题也有同内容成对截图。最终初始化稳定对照在evidence/visual/creation-preview-pass11；早期pass10参考toast等待超时明确保留失败，不代替最新证据。
- 真实生产dist的UI受控HTTP验收：45个窄屏空/加载/失败恢复场景、17组外壳/页面操作、管理设置2视口/9类资源编辑/4断点/账户校验共16组、初始化四来源与预览/异常/性格编辑11组、七阶段准备4组、视觉/头像/生成校验7组。具体断言在各JSON；这些计数是场景数，不等于全站全部操作通过。
- 七阶段衣柜失败/恢复、创建未知与恢复未知均保持原body/key；pending不会凭前端计时完成。证据为原creation HTTP fixture，不是实时Core/Temporal证明。
- 性格滑块由真实浏览器键盘.72→.73，完整JSON/生活嵌套/关系/扩展保持，刷新仍.73。原误用profileFields导致所有slider disabled，已修。
- 当前完整Web检查640/640通过，无skip，最新日志evidence/web-tests-pass13.log且无ReferenceError。类型检查、构建、fixture排除和React-only守卫通过，build-pass13。清理测试DOM前等待Radix延后焦点回调，避免先前假绿日志里的document未定义。
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

Docker Web仍为早期镜像，源码最新UI尚未更新到该镜像。当前截图用最新dist和同内容HTTP fixture，不冒称实时后端视觉验收。私有env/Owner密码/令牌/cookie仅在/private/tmp/fluctlight-demo-audit-20261028，不输出或提交。

## 当前活服务与脚本

- 只读Demo：http://127.0.0.1:4179/fluctlight-offline.html
- 正式最新dist+同内容测试HTTP：http://127.0.0.1:4381（fixture，不是实时后端）。修改fixture需重启任务自有该进程；改源码先build。
- 准备阶段fixture4383 wardrobe-failure、4384 recovery-unknown、4385 activation-unknown、4386 pending。各进程独立可变记录，不与原业务服务混用。
- verification/compare-pages.mjs、compare-creation-preview.mjs、verify-workspace-ui.mjs、verify-domain-ui.mjs、verify-read-states.mjs、verify-settings-studio.mjs、verify-creation-ui.mjs、verify-preparation-ui.mjs、verify-visual-ui.mjs。
- 用户已授权独立Playwright；临时依赖/private/tmp/fluctlight-demo-audit-20261028/browser-tools/node_modules，Chrome channel，独立profile。需沙箱外运行，不再尝试已断开的CUA/Pencil连接。

本任务继续in_progress，不能归档或声明全站完成。未推送、未发布。
