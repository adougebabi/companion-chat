# 全站反馈 UX：原矩阵的增量验收计划

此为只读研究与合并建议，不是反馈验收结果。没有启动服务、运行浏览器或重审既有 697 测试，没有修改 coverage.json、产品或脚本。基于当前 coverage.json（schema=1，baseline=ef8e1f92，177 行）、verification 脚本和既有最新合并/增量 report。全部 177 行反馈仍为“待验收”；没有新增 N/A，没有把旧 passed 提升为反馈通过。

## 可机器合并结构

`research/feedback-coverage-proposal.json` 按 `rows[].id` 与原 177 行对应，唯一且完整。仅合并 `feedback_ux`，保留现有视觉/交互/业务状态。每行包含：

- `status`、`batch_ids`、八个独立 `scenarios`；每个场景分别保存 applicability/status/evidence/issues。
- `legacy_state_evidence` 保留原矩阵的精确状态键、记录断言和报告引用，避免整份报告的其它案例被算到此行；`legacy_report_index` 解引用到 record_index、case、原 checks、原截图、原 boundary。
- `legacy_evidence_candidates` 只是查找线索；原业务/交互 passed 不代表类型、位置、文案、去重或视觉正确。
- `missing_assertions` 为统一需要补验的维度，不能据此认定产品已有缺陷。
- `issue_records` 的规范见顶层 `feedback_issue_template`：源文件行号、矩阵 ID、触发与原始业务状态、旧缺陷、新类型/文案/位置、真实动作、恢复后变化、是否涉及后端、桌面与390前后截图、验证记录。

八场景：normal、empty_first、unconfigured、draft_restore、read_write_failure、async_progress、failed_unknown_recovery、complete_return_refresh。正常场景适用，其余先“待核对”；已有脚本只说明可复用的状态，不能替代逐子流程适用性判断。标 N/A 时必须填写真实入口/业务边界/实现出处/实际检查依据；本建议不生成 N/A。

## 优先复用：状态已具备，反馈断言仍需补齐

| 真实入口/既有案例 | 原矩阵 ID | 已记录的可复用事实 | 尚需验收的反馈语义 |
|---|---|---|---|
| library → 草稿/已激活/损坏/生命周期/详情读取恢复；entry-states consolidated，pass116 | library-hero、library-empty-errors | 继续创建保留原槽位，损坏草稿不覆盖目录，真实重读清掉详情读取错误，目录失败不伪造卡片 | 有效草稿是低干扰继续；损坏/未知/初始化失败不能共用一个错误提示；局部故障不挤下整个列表；恢复后过期提示清除 |
| auth → setup/status-recovery | auth-formal-init | 字段校验阻止POST并聚焦，checking/busy防重复，重读状态不提交账户初始化 | 字段错误贴近字段；读取故障靠状态区；普通进度语义；成功进入工作区后无旧错误 |
| profile → 编辑资料；profile-save-states pass111 | profile-edit-basic、profile-formal-recovery | 空名不提交并聚焦，saving阻止重复，冲突保留输入，明确rebase只GET | 冲突提示类型与文案、真实刷新动作、保存后清除；不得把普通保存成功置顶长期展示 |
| people → authored about；people-state-edges pass120 | people-about-edit、people-formal-management 等原精确映射 | 读取投影无效不当空；只读角色不显示管理操作；about冲突保留输入 | 局部失效与全局故障分层；管理动作说明及恢复位置；非阻断依据不要强警报 |
| feed → suggestion/readback/comment；feed-state-edges pass122 | feed-suggest、feed-detail-comments、feed-formal-npc | accepted/processing不等于实际发布；恢复同请求不重发；评论读取失败保留正文 | 同事件提示去重；进度/失败/拒绝/未知的真实类型；评论错误靠评论；恢复后清除错误 |
| settings → busy/unknown/conflict/partial；management-state-edges consolidated | management-navigation（各设置子行仍需逐行局部验收） | late receipt保留新草稿；unknown先核对旧回执；partial不重发已确认端点 | 普通成功低干扰；部分成功保留具体失败；未配置引导真实配置入口；诊断信息收进可展开详情 |
| settings → import/backup；pass125 | management-data-import、formal-data-import-pack、management-data-backups、management-data-restore | unknown恢复原pack/digest/key；导入只建独立草稿；running无伪造取消/重试 | accepted不是完成；进度与未知区分；状态只由真实数据更新；关闭/返回不改变任务 |
| studio/edit → apply pending/unknown/rejected；pass108 | resources.studio.apply | pending不显示回执/目标结果；unknown保留新草稿并恢复原body/key/CAS；目标刷新仍正确 | pending/progress、unknown/action-required、明确拒绝/error的类型与动作；回执局部、去重、过期清除 |
| studio各typed editor → invalid/adopt/saved-reopened；pass115 | 对应 resources.studio.editor.* 原精确映射 | JSON原文保留，显式采用清掉本地失败，保存不自动发布/应用 | JSON错误贴编辑器，禁用原因可理解；采用后旧提示确实移除；保存反馈不冒充发布 |
| wardrobe → failed/denied/no-change/unknown/readback-failure；pass114 | resources.wardrobe.wear | 保留已确认穿着；unknown恢复原请求；readback恢复GET-only；刷新无重新请求 | no-change/拒绝/任务失败/读取失败不能同一成功提示；错误与可执行恢复就地展示 |
| creation → structured/world-conflict/snapshot/rejected-copy；supplement consolidated | creation-structured-editor、creation-world-conflict、creation-complete-navigation、creation-start-snapshot、formal-creation-rejected-copy | 取消/错误编辑保留完整定义；世界冲突继续同角色；完成返回不激活第二次；拒绝预览可导出原文 | 正常草稿、损坏、未知、初始化失败、历史不兼容各状态独立；来源更新动作真实；完成后提示不残留 |
| creation → preparation/read awaiting-review/image-failure；preparation-read consolidated | creation-seven-stages、creation-inventory-recovery、creation-visual-review | 单源错误不抹除已完成阶段；原来源GET恢复不重激活；人工审阅有可达动作 | 等待人工审阅是action-required；阶段失败保留强调；图片失败局限于图片；恢复完成清旧提示 |

以上每个案例的精确 report 路径、case name、矩阵 ID 和候选八场景在 JSON `priority_scenario_reuse`，原 checks 和 screenshots 位于 `legacy_report_index`。不能将表中同一报告覆盖的所有矩阵行一起判定反馈通过。

## 用最少浏览器批次补共享语义，不扩展业务回归

按“同一入口/同一HTTP状态夹具/桌面与390”合并验收准备与截图，但输出仍逐矩阵 ID、逐子流程记录。JSON `batches` 已将177行完整映射为12组，无遗漏无重复：entry 13、creation 24、chat_inbox 16、profile_people 24、feed 6、schedule_goals 13、diary 6、memory_self 10、management 27、world_wardrobe 13、studio 15、media 10。

建议执行顺序：

1. 先 entry + creation。直接覆盖正常草稿、损坏、未知、初始化阶段恢复、未配置及允许空白创建，验证共享警报语义。未配置专用HTTP场景不足时在现有入口夹具中补精确缺失场景，不改真实配置。
2. profile_people + management。覆盖字段校验、操作失败、并发冲突、普通成功、部分成功、草稿保留、诊断折叠。
3. chat_inbox + feed + studio + world_wardrobe。复用旧 pending/unknown/rejected/readback 场景，验证任务详情/待处理/返回/刷新，保留原request、CAS、target、key/body。
4. schedule_goals + diary + memory_self + media。按现有深层脚本的详情/操作状态补本地反馈。真实媒体生成链由用户负责，本轮不增开生成回归。

每组可在单次运行内完成多矩阵子流程，但共享组件表现只能证明“该实例在该状态正确”；不能替代未打开的保留领域入口、不同组件或不同数据状态。12组是操作分组而不是12个截图或宣称最小已证明下界。

所有15个列表读态可复用 `verify-read-states.mjs:8-23` 的真实入口和 endpoint 映射：now、wardrobe、profile、settings、chat、schedule、goals、diary、memory、world、people、feed、media、studio、inbox；原脚本目前固定390px（:27-28）且仅验证存在读取/重试/无溢出（:36-44），不足以证明桌面、语义、布局位置、重复提示或恢复后清除。扩展它的反馈断言比另开15套业务回归更省。需保留“控制HTTP/production dist”的证据边界，不把受控数据说成实时Core。

## 必补断言与判定规则

每个适用场景至少核对：业务状态对应information/action-required/warning/error/success/progress；文案说明发生什么/是否要处理/去哪处理；提示紧靠字段、操作、草稿或任务；同事件只有一个主反馈；动作触达真实端点且作用域正确；authoritative恢复后清掉旧失败；技术原码/stack/请求身份收进明确展开诊断；桌面/390长文无溢出、按钮可达、详情可关闭返回、正常列表不过度下移；关闭/导航/刷新不删草稿、不换原请求、不重置任务；accepted不提前宣告completed。

结果的 evidence 应记录 `report_path + record_index/case + assertion names + desktop screenshot + mobile390 screenshot + captured state`。发现问题后再记录真实源文件行号和旧缺陷；本研究没有读一般组件实现，不能给出伪造的产品缺陷位置。旧交互断言可作为恢复事实证据，新增反馈断言和前后截图才是追加验收证据。

## 现有前后截图索引

JSON `legacy_screenshot_phase_index` 精确列出271张已经存在的带 before/after/final-pass 阶段图片；每张均标 `feedback_ux_verified=false`。60份原矩阵引用JSON的每条记录还保留全部 screenshots，覆盖其他旧深层状态；原截图只能用作对照候选，不能标为本次修复后的反馈截图。

重点可直接查找：

- `evidence/form-groups/{feed-compose,people-create,people-edit,people-contact}-{before,after}-{desktop,mobile}.png`：16张，前后表单布局对照；report本身不含screenshots，此次索引由实际文件枚举补齐。
- `evidence/studio-deep/before-pass28-*` / `after-pass29-*`：原编辑/发布/应用预览对照；不等于本次UX治理的前后结果。
- `evidence/preparation-read/before44-awaiting-review-*`：原人工审阅问题现场；最新对应状态见 consolidated report 的真实截图列表。
- `evidence/visual/<page>/before/actual-{desktop,mobile}.png` 与各页 `final-pass*/actual-*`：正常页面对照，精确最终phase按原矩阵引用，不能取全局最大pass代替。
- `evidence/entry-states-pass116/library-directory-failure-{desktop,mobile}-{failed,recovered}.png`、`profile-save-states-pass111/{validation-busy,conflict-rebase}-*`、`studio-runtime-states-pass108/unknown-*`、`management-state-edges-pass125/import-recovery-*` 是可复用的失败到恢复状态序列，属于状态前后而不是代码修复前后。

旧矩阵本身有宽泛绑定（例如某整报告跨多个状态/行）；本建议保留原 state_key 和记录断言作为缩小依据，不继承整报告通过范围。没有在本次检查中人工打开旧PNG，因此仅提供索引，不声称其反馈视觉已合格。
