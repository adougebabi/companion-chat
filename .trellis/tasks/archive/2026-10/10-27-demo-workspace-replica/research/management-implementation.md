# 第 16 组实施与核验（2026-10-28）

范围为 16a 通知、16b 配置、16c 诊断/任务、16d 数据、16e 账户。此记录不勾选任务、不代表最终跨页审计完成，没有提交或推送。1–15 组既有改动保留。

## 实施所有者与入口

- 新增 `react-app/pages/InboxPage.jsx`，BrowsePage 的通知分支进入此原生 React 页面。新共享 `DiagnosticsWorkspace.jsx`、`useManagementView.js` 和 `styles/replica-management.css`；由实际 `react.css` 引入。
- 改造 ManagementPage controller/view、TaskDetailPage、AuthPage；App 仅调整这些页面的页头归属、通知自主读取的反馈边界，保留其它对象工具的原页头。
- 数据继续使用已存在的 `v3-domain/data-tools.js`、`business-backups.js` 正式控制器及 DomainPrimitives 单根端口，修改其分区/导航/卡片投影。DomainPage 仅给 `/data` 家族添加样式作用域。没有新增 HTML 字符串控制器、旧页面路由或第二应用根。
- 经 main 明确追加授权，最小修复 `shared/DomainPrimitives.tsx` 的双字段 blur 焦点问题；OverlayHost 未改。
- 新增隔离 `test/fixtures/management-fixture.mjs`，在原 loopback ui-quality-server 的可选管理场景挂接。不会进入正式 bundle。

## 16a 通知与待处理

Demo：CompanionPages InboxPage / companion.css 1461+。采用页头、下划线阅读页签、角色范围、通知行和 320px 待处理侧栏，同页 WorkspaceDialog 展示正文与来源。没有编辑通知正文的接口；该页编辑态为阅读/角色筛选草稿与原来源操作入口。

正式读取通过 operationsClient，通知追加分页保留原 cursor；待处理总组及每组各自拥有读取状态和游标。失败组显示未知，不能按空组宣告已处理；能力/身份/应用是需决定的事项，owner_request 与服务异常各自展示。mark-all 使用本次服务器 `as_of`；单条已读不调用处理/恢复命令。来源链接保留 `returnTo=#/inbox`。已读失败仍可阅读正文。实际链接消息的预览仍由原 readNotificationPreviews 读取。

五态与交互：正常、空白、加载、失败、详情；关闭返回原触发按钮，来源导航不做本地 resolve。两项读回/独立分页测试和新增 Owner 筛选恢复测试覆盖追加失败仍保留原卡片、同 cursor 重试、实际组重读、server as_of 和已读/来源分离。Owner 范围内的显示筛选存于 sessionStorage，不存第二份记录；退出/Owner 改变时清理当前显示偏好。

证据 `evidence/inbox/`：normal-desktop/mobile、demo-desktop/mobile、detail-desktop、empty/failure/loading-mobile、geometry.json。1348×925 实测主列表 x252/w703、侧栏 x979/w320、gap24；Demo x249.77/w705.23、侧栏 w320，列表开始 y313.83，正式 y304.83（约 9px 的工具栏排版差）。390px 文档宽度=390，无横向溢出。

## 16b 管理配置

Demo：LifePages SettingsPage/ModelSettings、life.css 537+。180px 二级目录与28px栏间距；模型服务/四用途子页签；聊天/视觉、图片、自主/安静时段、通知/后台、高级分区；sticky 保存栏。四用途初始均能读到字段，仍可折叠，多个用途共享真实服务和目录/手动模型 ID。正式服务标识不可改名，只有未保存行有移除入口。

完整配置草稿仍在原 controller；所有分区保持挂载，切换不丢输入。原 configurationPlan 在任何写入前验证完整草稿，configuration-ack 更新每条回执的版本，较新输入和密钥处理、角色 quiet policy CAS、原配置 command 核对/停止/读取最新基线全部保留。无删除保存连接、自动转移用途、模拟同步或 token 上限修改。新增初始读取/失败重试，未读到目录时不能保存。初始加载不滚过页面页头。

浏览器验证：编辑服务 URL、放弃回到原值；用途超时120→121经正式形状 PUT + command receipt确认；全部配置分区可达；空服务、配置读取失败和加载独立展示。正式范围/默认值来自 configurationFields；fixture 的缺省字段保持空，不复制 Demo 数字。现有完整表单/较新输入/版本和原命令测试保留。

证据 `evidence/management/`：各分区 desktop、roles/edit/saved、normal/empty/loading/failure-mobile、Demo 对照和 geometry.json。1348×925 实测 nav x252/w180、main x460/w839、gap28。390px 文档宽度390；目录换行与单栏字段、44px按钮不造成横向溢出。

## 16c 诊断、任务和恢复

新增诊断组合展示实际 workflow-intents 目录、模型与媒体调用、正式 Core readiness/已保存配置连接测试。统计仅算已读取任务，未成功读取显示 —；筛选后无结果与读取失败分开。调用初次失败按来源保留独立错误与重读，不能显示为没有调用，某来源失败不会擦掉另一来源/原确认记录。原其它九类诊断记录、完整输入/响应/工具和来源深链接全部可达。

TaskDetailPage 使用执行阶段/执行记录/任务信息三页签，阶段只来自正式 steps。移除旧页面按 status 假定1/3、2/3的百分比。任务恢复仍由原 WorkflowRecovery 控制：allowed_actions、实际 run_id、intent revision、必填 reason、resettable history 决定动作；未知重试保持原 body/key。目录返回 `#/diag?view=tasks`，Owner 显示筛选草稿返回后保留。

浏览器验证：实际任务行→正式 task GET→workflow inspection→带理由 restart POST→原 task/inspection GET；accepted 后仍为失败状态，不宣告任务完成。任务信息、列表筛选返回、调用详情关闭焦点、readiness与真实形状失败连接回执、空白/加载/失败/390px均检查。原 unknown 工作流双次同 body/key 的 React 交互回归在 r11-flows 保留。详情不暴露状态推测的重试动作。

证据 `evidence/tasks/`：list/detail/edit/accepted/information/calls/call-detail/health/health-test desktop、list/detail/health/empty/failure/loading mobile、Demo 对照、request-audit.json（sourceRunId=run-fixture、expectedRevision=3、reason 与 key）。390px文档宽390。

## 16d 资源包、备份与修订边界

Demo DataPanel 的统计、分区、导入预览和备份卡片结构；正式语义覆盖 Demo 的本地快照。统计仅使用正式已读取角色/分组目录；另外两格是定义包与隔离恢复的范围说明，不伪造备份总数或已修改组数。

导出为完整正式资源定义包，文件读取3MiB，JSON草稿、精确校验/digest、预览、独立导入草稿、分组建议采用均沿原控制器。没有运行人物/存档覆盖、自动发布/应用、清空工作区、备份下载/删除或时间线回档。对象修订/回档说明以及 pack/backup 原路由仍可达。

浏览器完成：导出生成实际 JSON 下载链接（不做长下载等待）；原 JSON 含false/0→校验→理由确认→独立草稿详情；分组采用后读回显示“已采用”并禁用重复动作。未知导入503后刷新，明确“核对或重试原导入”，恢复成功；两次 POST pack 的完整 body（含expectedDigest/reason）及idempotencyKey严格相同，见 original-command-audit.json。

备份完成的 UI/API 链路：create→queued、queued cancel、failed retry→queued、verified/restorable source→显式隔离目标/checkbox/reason→restore queued。恢复请求 expectedRevision2 + confirm:true + fluctlight_restore_fixture2026；没有执行部署维护 operator。queued始终不是完成。

证据 `evidence/data/`：normal/Demo desktop/mobile、export/preview/import-confirm/pack/group-confirm-fixed/group-adopted、backup-create/queued/cancelled/retry、restore-confirm/queued desktop、restore/empty/failure/loading mobile、两份 request audit、geometry.json。桌面 nav180+gap28，390px文档宽390。五态的空白/失败/加载针对实际备份目录与资源包读写，已有角色目录不会被伪造清空。

### 双字段焦点修复

浏览器分组采用 Sheet 中，从名称用鼠标或Tab进入理由会立即跳回名称。隐藏 carrier 的 blur 原来以普通 Event 重发 focusout，丢掉 relatedTarget，让 Radix trap 认为焦点离开。FieldPort现在用 FocusEvent保留目标；blur选择范围只同步隐藏carrier，不转发到可见控件。控制器主动 source.focus()/setSelectionRange 的转发仍保留。实际 IAB 已复验分组采用与三字段隔离恢复；新增真实Sheet回归覆盖目标、carrier selection、鼠标/Tab焦点和主动恢复。shared门禁24/24通过。

## 16e 正式认证与账户

Demo App.tsx AuthPage + index.css1265/1398/1595。桌面1050px框、600px高、双栏、50px故事padding、32px标题与原WorldArt；未认证页位于自己的壳层外，有静态品牌，不读取/展示私有角色切换。<=900px按源码隐藏故事栏，单列表单保留600px框与35×24px内距。与Demo已认证侧栏/顶栏所在位置的差异是正式认证边界，不添加私有目录来追赶外壳。

保留正式setupStatus、setup/login/logout：12字符密码、>=16初始化令牌、完全相同确认。无邮箱注册、6位密码、直接体验、密码持久保存、改密/撤销全会话新增功能。新增明确checking状态，401显示中文拒绝。原密码/令牌仍为当前页面内存，成功后清空。

浏览器完成空输入聚焦密码、setup尾空格确认拒绝并聚焦确认字段、正确setup→角色库、管理页logout→空认证表单、login→角色库、401保留输入、setupStatus失败/重读、loading和submitting禁用。初始未认证 audit 的/api请求为0。390px宽度390，桌面frame1050，窄屏story display:none。

证据 `evidence/auth/`：login/validation desktop、login/setup/exact-confirmation/authenticated/logout/failure/read-failure/loading/submitting mobile、Demo对照、geometry.json、unauthenticated-request-audit.json。setup-mobile/exact-confirmation-mobile/setup-desktop已在最终900px隐藏规则下补拍；geometry.json记录实际尺寸。

## 检查与运行句柄

- TypeCheck通过：`pnpm --filter @fluctlight/web typecheck`，日志 `/private/tmp/demo-replica-management-typecheck.log`。
- Lint通过（同tsc脚本）：`/private/tmp/demo-replica-management-lint.log`。
- 全量Web tests **617/617**：`/private/tmp/demo-replica-management-tests.log`。新增7个测试：通知3，管理/任务/数据/共享焦点4。旧r11任务列表断言映射到全量实际steps/无伪百分比，r12章节断言映射新分类但仍测试折叠保留字段。
- Production build通过：`/private/tmp/demo-replica-management-build.log`，fixture exclusion与React-only **374 local modules**通过。既有大chunk提示仍存在。
- git diff --check通过。没有修改后端、BrowserClient、LLM/prompt/token策略、接受的1–15组页面、规格或任务完成标记。
- CUA browser2，单产品tab1；原Demo4178在线。已viewport.reset并markHandoff；保留4374的正式管理页。
- 新服务：4361/4362/4363/4364通知normal/empty/loading/failure；4365/4366/4367/4368配置normal/empty/failure/loading；4369/4370/4371/4372任务normal/empty/failure/loading；4373数据动态场景；4374最终数据/账户动态场景。全部loopback、无真实上游，未因观察超时重启服务。最终服务支持浏览器访问 `/fixture/management-scenario?value=normal|empty|failure|loading|unknown-import|auth-login|auth-setup|auth-failure|auth-read-failure|auth-loading` 变更隔离场景。

## 待主线程继续

本组是正式dist + 隔离API的表现层及协议验收。没有证明live PostgreSQL、Temporal、模型、NAS或部署operator实际执行。导出已核验生成下载链接/完整包预览，没有长等待下载文件认证。数据仍使用原领域控制器的有界端口；自定义页面表单未被冒充全部原生React迁移。全部对象深链接与全站跨页一致性审计仍由main执行，再决定任务完成状态。
