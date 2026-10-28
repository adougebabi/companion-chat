# 第16组独立检查（2026-10-28）

结论：第16组及本轮获授权的共享/历史路由窄修通过独立质量门禁。最终全范围交付仍由主线程根据 `final-completion-audit.md` 收敛；本报告不勾选任务、不修改规格、不提交、不推送。

## Findings (fixed)

1. `apps/web/src/v3/proto/components/durable-command-drawer.js:18–46`：原命令在收到回执时释放 pending，但后续 `onSaved` 读取失败后又允许新的确认提交，关闭重开也丢失已接受结果。先加入真实 OverlayHost 挂载回归，原实现明确失败于 send disabled 的断言（`/private/tmp/demo-replica-check-repro.log`）。现于 pending 释放前保存原输入、原幂等身份、回执及提交字段指纹；恢复按钮只执行 `onSaved` 原结果读取，不重发 mutation。新输入在提交和回执恢复期间均保留。关闭中的迟到回执可留给原范围恢复；连续 Owner 订阅与存储身份核对阻止 logout 后同 Owner 重入的旧回执复活/清除新槽。
2. `apps/web/src/v3-domain/data-tools.js:22`：原导入恢复入口只检测 pending。补充检测 accepted checkpoint，刷新后仍可进入原回执读取，而非要求重新校验并再次导入。
3. `apps/web/src/v3/proto/components/studio-draft-storage.js:7`：退出清理遗漏配置原命令、角色 quiet-policy 命令及已离页的管理显示偏好。补充精确 Owner 前缀；保留其它 Owner 键，普通离页不触发清理。未把配置密钥写入草稿。
4. `apps/web/src/styles/replica-management.css:2–4`：通知列表确有9px起点偏移。稳定 Demo 为 x252/y313.828，旧 x249.77 是动画过渡读数；源页面 gap23、toolbar55。改为相同桌面间距与工具栏高度，700px以下 gap18；正式与 Demo 的 heading/toolbar/list-start 坐标现一致。390px 实测 document width390。
5. `apps/web/src/approved-ui/feature-routes.mjs:33`、`react-app/App.tsx:158,213`、`pages/ChatPage.tsx:49,92,366`、`shared/ConversationHistory.tsx:12,34`：最终范围审计发现独立 `/c/:id/chat/history` 仍走另一套旧读取器，失败追加的“重读”会从第一页替换。经主线程追加授权，路由直接复用现有 ChatPage/ConversationHistory。保留旧读取器实际消费的 q，先确认角色目录再搜索；默认入口不变。App 与普通聊天使用相同 key，关闭后回安全 returnTo 或该角色 chat，保留未发送草稿和焦点；浏览不会 select/resume。旧域历史实现不再位于该生产路由导入图中。
6. `apps/web/test/ux-settings-support.test.mjs:66`：更新源码加载测试的 subscribe/storage 测试依赖，继续验证 originalOnly 无原请求时不能提交空命令。不是删除或放宽原断言。

## 检查依据与实际路径

- 读 PRD/design/implement、check.jsonl 所列规格/记录、management implementation/scope map、completion audit requirements、原 REPLICA/SOURCE_MAP 与 Demo 对应样式。
- Inbox 原生生产入口由 BrowsePage 分支进入 InboxPage。通知与 pending 各自 useWorkspaceRead，kind 子组独立游标/失败重试；初始失败组不按空处理。mark-all 使用本次 server as_of；readNotification 不 resolve 事项。Owner/fid筛选、来源 returnTo、详情关闭回原按钮有交互覆盖。
- Management controller 保留实际 configurationFields/schema/enums、完整表单 configurationPlan、顺序写入/command回执/versions ack、较新输入和凭据清理。quiet-policy 仍走实际角色revision/CAS；保存连接没有假删除/转移用途。初始配置失败不能保存。诊断调用来源与其它九个诊断目录/原始记录保持可达。
- TaskDetail 使用实际 steps/history；WorkflowRecovery 决定 allowed_actions/current run/revision/reason/resettable checkpoint。新页不按status拼百分比；accepted后不标 completed。现有 r11 unknown 同body/key回归继续通过；请求审计含 sourceRunId=run-fixture、expectedRevision=3、明确reason与原key。
- 数据仍为既有正式领域控制器的有界端口，不声称全原生React页。导入绑定原精确JSON/validation digest/reason；分组采用读回；备份create/restore/retry/cancel保留正式revision、显式隔离目标和确认。queued不是完成。原未知导入请求审计证明两次body/key相同。没有快照覆盖、备份假下载/删除或浏览器清库。
- 所有 durableCommandDrawer 直接调用者已查：pack import/backup导航或reload；group为refresh+reload；workflow/media恢复为reload；place-defaults为GET及UI草稿合并。没有 onSaved 再发mutation。共享回归覆盖这些保留端口所在测试集。
- DomainPrimitives focusout 保留 relatedTarget，blur selection只同步carrier；主动source.focus()/setSelectionRange仍转发。真实Sheet回归和IAB分组采用：名称→Tab→理由输入，activeElement为第二可见textarea；Escape回“核对并采用”，没有提交。共享其它控件/IME/命令回归纳入全量622项。
- Auth 保留正式 setupStatus/setup/login/logout、12字符密码、>=16初始化令牌、精确确认及失败输入。未认证请求审计为0个/api；未添加邮箱注册/游客绕过/密码持久化。<=900px隐藏故事栏是Demo源码规则，外部认证壳与已认证Demo壳的差异已记录。

## 浏览器与证据修正

使用现有loopback服务和CUA，未重启服务。正式构建重载后，1348×925 Inbox：header x252/y94/h91.828；toolbar x252/y235.828/h55；list x252/y313.828/w1047，与稳定Demo一致；正文条数/高度不同来自真实隔离数据。390×844无文档横向溢出。正式新版证据为 `evidence/inbox/check-normal-{desktop,mobile}.png`。

文件尺寸核查发现旧第16组 Demo 图中的若干文件实际为1333×915与375×812，因此不得拿它们声称严格同视口。已补拍 inbox/management/tasks/data/auth 五目录的 `check-demo-desktop.png`（1348×925）和 `check-demo-mobile.png`（390×844）；旧图保留为历史参考。Auth旧两张原图本就满足目标尺寸，新图用于本轮统一证据。

`evidence/data/check-group-focus-mobile.png` 是实际双字段Sheet焦点证据；`evidence/chat/check-history-deep-mobile.png` 与 `check-history-return-draft-mobile.png` 是真实App路由、q=sample/second、Escape返回及未发送草稿“独立验收：未发送草稿”的证据。最终浏览器停在4344衣柜，viewport已reset并markHandoff；所有fixture服务保留。

## Findings (not fixed)

- 未发现仍需阻断第16组门禁的确认代码缺陷。全范围证据/规格/交付收口不是本代理允许直接完成的工作，见最终审计报告。
- 规格建议：补录durable drawer已接受回执的GET-only恢复、精确Owner清理、历史深链接复用与通知稳定几何。按派发约束未改规格。
- 这里验证正式dist+隔离API；没有live Core/PostgreSQL/Temporal/模型/NAS/operator运行结论。实际部署维护、媒体字节备份核验不由fixture证明。

## Verification

- Lint：pass，`pnpm --filter @fluctlight/web lint`；`/private/tmp/demo-replica-check-lint.log`。
- TypeCheck：pass，单独运行typecheck；`/private/tmp/demo-replica-check-typecheck.log`。两者项目脚本均为tsc --noEmit。
- Tests：pass，完整Web **622/622**，0 fail/skip；`/private/tmp/demo-replica-check-tests.log`。本轮新增5项，总管理专项9/9。原回执问题先红后绿，最终测试日志晚于所有代码/测试修改。
- Build：pass，`/private/tmp/demo-replica-check-build.log`；fixture exclusion与React-only守卫通过，**372 local modules**（历史旧导入图退出后由374减少）。既有chunk大小提示仍在，不是失败。
- `git diff --check`：pass。
- 未改后端API、模型请求/提示词/token政策；未修改/清理10-22未跟踪材料；未提交/推送/部署。
