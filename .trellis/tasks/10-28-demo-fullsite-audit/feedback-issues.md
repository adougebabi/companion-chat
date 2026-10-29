# 全站反馈与异常 UX 问题台账（增量）

承接原 F01–F69 与 177 行覆盖矩阵，未重新开始任务。本表记录本轮确认的缺陷；“代码/定向通过”不等于该模块全部桌面与手机子流程通过。原始失败报告保留，最终选择规则在 `verification/update-feedback-coverage.py`。所有 HTTP 夹具证据均运行生产组件，不认证真实 Core/数据库/模型执行。

## 已修复的问题与真实恢复边界

| ID / 页面及拥有者 | 原状态与缺陷 | 新提示类型、位置与实际操作 | 恢复后的界面 / 验证证据 |
| --- | --- | --- | --- |
| FG01 全站；`shared/Presentation.tsx` | 普通说明、受理、成功与实际错误共用隐式 destructive，技术详情混入正文 | 新 Feedback 必填六类 kind；旧 InlineFeedback 默认错误保留。页面选择业务语义；正文保留可理解的具体原因，诊断折叠 | Reader、Result、端口及具体调用分批迁移；六类角色/回调/诊断/旧事件到期回归，15 主页面读取浏览器实例通过。未因此宣称全部调用点通过 |
| FG02 主读取；`useWorkspaceRead.js`、`ReaderFeedback`、`App.tsx` | 初次失败、已有确认数据后的重读失败、加载混同，重复通用错误挤占正文 | 初次失败 error；保留数据时 warning；实际读取 progress；原 loader/cursor 重读 | 确认读回清除失败和诊断；`feedback-read-final/after.json` 120 条，before 同场景。子页面仍逐项验收 |
| FG03 角色库/向导；`creation-recovery-state.mjs`、`creation-draft.mjs`、`LibraryPage.jsx`、`CharacterWizardPage.jsx` | 有效草稿、JSON 损坏、版本不兼容、缺失嵌套、孤立/错配命令都指向继续创建 | 草稿 action-required + 单一继续；无效记录 warning + 真实本机重读/原记录下载；存储不可读 error；不提供伪造修复 | 原字节、requestId、body/key 保留；读取确认空记录后提示移除。entry-after-feedback3 26 条；旧构建 entry-before-feedback1/2；桌面/390图完整，代表图人工查看 |
| FG04 创建历史；`creation-recovery.mjs`、`LibraryPage.jsx` | 当前草稿损坏可影响独立历史；指定历史缺失可能回退到当前资料；目录晚到缺链接；旧 session 孤立命令漏检 | 精确 resume/Owner 身份、独立历史读取、后到目录链接；无有效记录则阻断，不替换为新创建 | historical-independent、legacy-orphan、mismatch、storage-getter 实际浏览器及单元回归；不删除历史或原命令 |
| FG05 角色库准备；`LibraryPage.jsx` | 实际 awaiting_review 仍显示持续准备中，等待不会推进 | action-required“候选形象等待你审阅”，在创建入口旁指向原准备进度和实际审阅入口 | entry-after-feedback4 双视口；只激活一次，返回不重激活。对应旧构建 before-feedback4；已亲看390卡片 |
| FG06 聊天；`state/conversation.ts`、`ChatPage.tsx` | localStorage getter 可在模块初始化崩溃；运行中 archive getItem 失败被当空对象写回，可能覆盖其他会话原记录 | 可恢复的本机记录读取错误与真实重读入口；发送前和写前阻断，输入保留，消息服务端状态不变 | getter 浏览器 entry 26 含双视口；运行中两种读取/写边界故障实际控制器回归，foreign 原 turnId/key/字节不变。运行中手机截图尚未补，不冒称已全验 |
| FG07 配置；`pages/ManagementPage.jsx`、`views/ManagementPage.jsx` | 未知原提交仅在相同 kind 拦截，其他端点可先写；刷新后角色策略未发现 | 保存入口和 form guard 统一阻断；发现当前 Owner 的 config/policy 原记录，原回执/原策略重试；保留 body/key/CAS | management-state-edges-feedback-after3 14 条；config-final 的 policy-restored 双视口；旧构建对应14+4图；原key精确相同，无额外新保存 |
| FG08 配置草稿/存储；同上 | 恢复 stale 连接草稿要求读基线却无按钮；运行中存储扫描抛错逸出 | stale 为待处理，实际“读取最新基线（保留输入）”；存储失败 error + 只读“重新读取本机提交记录” | 基线读成功才允许新保存；本机读取恢复才清错误。config-final stale/storage-runtime 四条，输入及原存储保留、无后端写入；已人工查看手机故障卡 |
| FG09 配置和账户字段；`configuration-plan.mjs`、`SettingsField.tsx`、`AuthPage.tsx` | 工作流结构/数值/密码等错误落页面级提示，修正字段仍残留；手机固定保存栏挡住错误字段 | 字段旁 FieldError、关联 aria-invalid/describedby、本地文案保留必要 JSON 键；选中对应分区、居中滚动并聚焦字段 | 输入纠正即清旧错误；配置原子验证不提交。config workflow 最新 feedback5 双视口测量输入/原因高于保存栏；Auth 定向行为回归，账户字段手机全流程待追加 |
| FG10 配置成功；`ManagementPage.jsx` | 原角色设置恢复成功仍写 error；失败 catch 遮蔽 error cell，留下正在核对；成功永久占位 | 重试时 progress、失败显示真实原因并结束进度；确认后 success，普通确认6000ms到期 | config-final policy/policy-restored 双视口：实际原请求两次相同 body/key；success 非 alert、到期移除；失败原key仍保留 |
| FG11 上传；`MediaUpload.tsx` | 新文件保留旧成功；未知记录与普通失败重复；损坏原记录仍可能发新上传；旧列表GET覆盖新上传提示 | 文件验证就地 error；未知 action-required 原上传核对；坏记录 error + 本机重读；已保存后列表失败 warning + 实际列表重读；共享操作代次隔离迟到结果 | accepted 不重传字节，选新文件清旧成功；旧GET失败不覆盖B上传 progress/success。r11-flows 8项含真实组件并发回归；真实生成由用户测试，双视口上传状态专项仍待补 |
| FG12 通知/日记/记忆/头像/目标；对应页面与 ResultFeedback | 普通保存确认长驻或重复 Toast；已解决失败未清；请求受理被当完成；旧计时器可清新通知 | 普通确认按事件身份短暂显示；需处理/任务不自动到期；独立未读计数、原目标进展与已读状态分别处理 | 页级定向与共享事件回归；inbox/diary/memory 主读取120中的各8条。更深层写后返回/刷新提示仍按各子流程待验 |
| FG13 任务；`WorkflowRecovery.tsx`、`TaskDetailPage.tsx` | 模态和父区域重复错误；受理即显示完成；runtime 缺失被误当新run而提前清处理中 | 局部失败/原提交核对，受理为 progress，重复控制禁止；只接受 intent revision 增长或实际 available run ID 改变 | r11 原body/key、同revision/runtime缺失仍保留进度、真正推进才移除。任务双视口专项提示验收待追加；不改变 allowed_actions |
| FG14 保留读取/命令；`async-page.js`、`durable-command-drawer.js`、`governance-command.js`、`versioned-fields.js`、`goal-editor.js` | 读取失败无真实重试、缺 JsonDetails 导入导致恢复分支再次崩溃、受理与回读状态混色、重复成功；未知记录不可读 | 显式 kind、实际原读重试、诊断折叠、坏记录阻断/本机重读；accepted 只读回；CAS待核对保留输入 | 实际模块导入+读失败重试回归、durable accepted GET-only/坏记录/迟到回执、goal latest GET failure 定向通过。管理备份/导入14条双视口，其他保留工具仍待逐状态浏览器补验 |
| FG15 保留端口；`DomainPrimitives.tsx` | 旧告警容器无法表达真实业务语义，MutationObserver 重新挂接可误覆盖角色 | 源拥有者显式 data-feedback-kind，端口保留原host/listener/FormData；标注 owner/generated 属性避免重入 | 独立只读审查未发现新增循环或源节点替换，端口/表单回归通过；不按文字猜状态，也不自动把所有旧alert当普通信息 |
| FG16 此刻；`App.tsx`、`state/workspace.mjs`、`LifePage.jsx` | 此刻读了无关目标/日记/记忆/约定，局部读取失败可污染此刻反馈 | 原拥有者的 currentOnly 路径只读当前资料/日程，保留衣柜及原 request observers | 主页面读取120中8条及Now定向；无后端修改，不全局跳过App加载 |
| FG17 深层弹窗；`WorkspaceDialog.tsx`、`replica-profile.css` | 手机标题/说明缺少 Demo 居中，档案标签颜色与gap漂移 | 原共享DialogHeader恢复手机居中/桌面左对齐；档案标签回原色/500/7px | dialogs-feedback-final1/2 五类×两视口10组配对，主线程逐张查看。保留固定操作区及正式额外字段；整窗不据此标像素一致 |

## 深层继续定位的 FG18

- 文件：`react-app/shared/WorkspaceCommandDialog.jsx`、`CommandDialog.tsx`、`react-app/components.tsx`、`styles/replica-chat.css`。
- 原触发：请求已确认但页面读回失败，同时显示普通受理确认和读回失败；聊天cancelled虽为information，旧alert选择器仍固定红色。
- 修改：读回失败优先一个主反馈；保留独立的实际失败原因、原请求入口和只读恢复，原内容收进同一披露，不隐藏真实失败；取消/处理中/实际错误各自用对应颜色。
- 清除：原请求/只读数据实际确认后更新；不通过关闭UI解除命令，不改body/key/CAS。
- 验证：共享去重23项、换装/反馈6项、全量718通过；Feed8和Wear10两视口原请求/恢复通过；聊天取消两视口语义与computed中性色核对。深层新增40条对应前后报告在feedback-deep-final，主线程查看了手机建议读回失败、AboutYou冲突、聊天取消及人物详情对照。
- 后端：无修改；上述为受控HTTP与真实组件。全站其余具体子流程仍按原矩阵继续。

## 修改前后证据索引

- 创建：`evidence/feedback-entry-before-feedback1/`、`before-feedback2/`、`before-feedback4/` 对应 `after-feedback3/`、`after-feedback4/`。
- 主页面：`evidence/feedback-read-final/{before,after}.json` 精确记录120场景源 report、record index 和截图。保留初轮失败与校正记录；不采用失败截图冒充最终结果。
- 配置/数据：`management-state-edges-feedback-before1/` 与 `after3/`；字段/草稿/本机读取/原角色策略：`feedback-config-final/{before,after}.json` 共10条，工作流字段以最新 feedback5 替换旧画面。
- 深层 Demo：`evidence/visual/dialogs-feedback-final1/` 与 `dialogs-feedback-final2/`；每类有独立 reference、actual、comparison，参考 Demo 未修改。

台账各条 backend 变更均为“无”；请求身份、持久化/回读边界依托现有业务实现。真实媒体生成链交给用户，不把上述图片夹具当模型产物。真实隔离Core的只读抽查另见 real-browser 报告。

## 继续验收的明确范围

177行均保留八场景反馈维度。当前只把上述精确实例写入 observations，未将整行或全站状态批量升级。下轮按 research/feedback-coverage-plan 的12批继续：创建七阶段异常、聊天流式/取消/队列与历史、人物/关于你与关系写入、动态评论和建议、日程/目标深层、日记生成与批注、记忆/反思/自我治理、世界/衣柜写入、九类创作编辑/发布/应用/沙盒、媒体上传/审阅/头像，以及任务/诊断/正式额外工具。

新发现的深层视觉差异需继续处理：人物详情仍为较疏的纵向资料块，与 Demo 的紧凑键值行不同；部分表单标签、说明、字段间距和披露区密度仍不一致。正式来源/权限/持久化状态与 Demo 不同的字段保留，不能为了截图一致虚构地点、认识事实、目标重要度或操作能力。
