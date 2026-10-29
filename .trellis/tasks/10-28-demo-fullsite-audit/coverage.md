# 全站覆盖矩阵

初始 177 行。行级视觉、交互、业务状态保持独立；本轮执行细节在每行 state_verification，不以单个正常页或一次测试覆盖整行全部状态。

|ID|页面/入口|视觉|交互|业务|本轮已登记具体核验项|
|---|---|---|---|---|---|
|shell-navigation|全部页面 / 侧栏与品牌|待检查|待检查|待检查|4|
|shell-switch|侧栏正在相处角色选择|待检查|待检查|待检查|4|
|shell-sidebar|顶栏收起/展开与移动遮罩|待检查|待检查|待检查|8|
|shell-jump|快速前往 / Cmd或Ctrl K|待检查|已修复待验证|待检查|3|
|shell-help-reset|交互演示/演示说明 → 帮助 → 重置演示数据|明确不适用|明确不适用|明确不适用|0|
|shell-account|侧栏账户与登录|待检查|待检查|待检查|4|
|library-hero|角色库 / 开启新的相识与继续相处|待检查|待检查|待检查|8|
|library-search-view|角色库 / 搜索、标签、网格列表|待检查|待检查|待检查|3|
|library-cards|角色库 / 角色卡片|待检查|待检查|待检查|5|
|library-empty-errors|角色库 / 空筛选和底部创作入口|待检查|已修复待验证|待检查|6|
|auth-demo-form|账户 / 登录或注册|待检查|待检查|待检查|6|
|auth-formal-init|Demo不适用：纯前端演示无需部署初始化|待检查|待检查|待检查|5|
|chat-room|聊天 / 页首与角色栏|待检查|待检查|待检查|9|
|chat-send|聊天 / 消息输入|待检查|待检查|待检查|7|
|chat-message-cards|聊天 / 消息流与图片|待检查|已修复待验证|待检查|10|
|chat-turn-recovery|聊天 / 原回应重试、停止与运行状态|待检查|待检查|待检查|6|
|chat-say-prefill|由其他页面say参数进入聊天 / 草稿冲突提示|待检查|已修复待验证|待检查|2|
|chat-now-preview|聊天 / 此刻侧栏|待检查|待检查|待检查|3|
|chat-history|会话历史 / 目录与阅读|待检查|待检查|待检查|3|
|chat-history-formal|Demo不适用：仅有静态历史阅读|待检查|待检查|待检查|2|
|chat-attachment|对话图片 / 窗边的一个片刻|待检查|待检查|待检查|2|
|profile-hero|角色档案 / Hero、四页签|待检查|待检查|待检查|8|
|profile-overview|身份与背景 / 表达与相处|待检查|待检查|待检查|8|
|profile-personas|性格与人格 / 共同性格、人格卡、协作、切换记录|待检查|待检查|待检查|4|
|profile-life|生活与此刻|待检查|待检查|待检查|2|
|profile-history-source|修订与来源 / 完整设定与来源弹窗|待检查|待检查|待检查|2|
|profile-edit-basic|编辑资料 / basic|待检查|待检查|待检查|5|
|profile-edit-expression|编辑资料 / expression|待检查|待检查|待检查|2|
|profile-edit-life|编辑资料 / life|待检查|待检查|待检查|2|
|profile-edit-more|编辑资料 / more|待检查|待检查|待检查|2|
|profile-switch-persona|人格切换确认弹窗|待检查|待检查|待检查|2|
|profile-foundation-proposal|身份与能力 / 身份核心提议|待检查|待检查|待检查|2|
|profile-capabilities|身份与能力 / 能力申请|待检查|待检查|待检查|2|
|profile-self-claims|身份与能力 / 发展中的自我|待检查|待检查|待检查|2|
|profile-formal-recovery|Demo不适用：本地同步编辑没有真实提交恢复|待检查|待检查|待检查|6|
|people-directory|人物关系 / 世界中的人|待检查|待检查|待检查|19|
|people-detail|人物详情弹窗|待检查|待检查|待检查|7|
|people-create|新增世界人物 / 编辑世界人物|待检查|待检查|待检查|3|
|people-about-display|关于你 / 自填资料|待检查|待检查|待检查|5|
|people-about-edit|关于你 / 编辑与修订|待检查|待检查|待检查|6|
|people-knowledge|关于你 / 对你的了解弹窗|待检查|待检查|待检查|2|
|people-relation-basis|关于你 / 关系与交往依据弹窗|待检查|待检查|待检查|2|
|people-authored-history|你的资料修订历史弹窗|待检查|待检查|待检查|4|
|people-relationship-tab|你们的关系 / 指标与共同片刻|待检查|待检查|待检查|2|
|people-formal-management|Demo不适用：仅人物定义编辑，没有真实接触与治理|待检查|待检查|待检查|2|
|feed-filter-list|动态 / 作者范围、搜索、展示状态|待检查|待检查|待检查|8|
|feed-post-actions|动态卡 / 回应与隐藏恢复|待检查|待检查|待检查|2|
|feed-detail-comments|动态详情 / 正文评论与来源|待检查|待检查|待检查|11|
|feed-publish|我来发动态 / 发布弹窗|待检查|待检查|待检查|5|
|feed-suggest|邀请分享近况 / 建议输入和结果|待检查|待检查|待检查|7|
|feed-formal-npc|Demo不适用：没有世界人物在动态中的真实互动|待检查|待检查|待检查|4|
|inbox-list-read|通知 / 全部未读、角色过滤与已读|待检查|待检查|待检查|16|
|inbox-detail-source|通知详情 / 返回与来源链接|待检查|待检查|待检查|4|
|inbox-pending|需要你决定 / 待处理卡|待检查|待检查|待检查|5|
|inbox-task-recover|失败任务通知 / 原任务恢复轨迹|待检查|待检查|待检查|2|
|inbox-back-chat|通知 / 回到日常|待检查|待检查|待检查|4|
|inbox-formal-groups|Demo不适用：只模拟几条通知与pending，没有真实多来源运行态|待检查|待检查|待检查|4|
|avatar-source-all|侧栏/角色库/聊天/档案/关系/动态头像|待检查|已修复待验证|待检查|2|
|shell-formal-routing|Demo不适用：Demo始终有本地种子角色与可用页面|待检查|待检查|待检查|2|
|life-now-current|now|待检查|待检查|待检查|9|
|life-now-schedule|now → schedule|待检查|待检查|待检查|8|
|life-schedule-timeline|schedule?tab=timeline&date=|待检查|待检查|待检查|23|
|life-schedule-detail|schedule → 这一段安排|待检查|待检查|待检查|11|
|life-schedule-suggestions|schedule?tab=suggestions|待检查|已修复待验证|待检查|8|
|life-schedule-plan|schedule → 为一天安排生活|待检查|待检查|待检查|4|
|life-schedule-habits|schedule?tab=habits|待检查|待检查|待检查|4|
|life-goals-list|goals|待检查|待检查|待检查|11|
|life-goals-detail|goals → 目标详情|待检查|待检查|待检查|14|
|life-goals-edit|goals → 写下新的期待/编辑目标|待检查|已修复待验证|待检查|5|
|life-goals-state|goals → 暂时放下确认|待检查|待检查|待检查|6|
|life-diary-read|diary?tab=diary|待检查|待检查|待检查|21|
|life-diary-annotation|diary → 写一点批注/修改批注|待检查|待检查|待检查|7|
|life-diary-reply|diary → 读完后想对她说|待检查|待检查|待检查|2|
|life-diary-shared|diary?tab=shared|待检查|待检查|待检查|3|
|life-diary-generate|diary → 整理过去一天的日记|待检查|待检查|待检查|2|
|life-memory-list|memory?tab=memory|待检查|待检查|待检查|12|
|life-memory-detail|memory → 记忆详情|待检查|待检查|待检查|9|
|life-memory-correct|memory → 纠正这段记忆|待检查|待检查|待检查|6|
|life-memory-forget|memory → 遗忘确认|待检查|待检查|待检查|2|
|life-memory-restore|memory → history → 恢复此版|待检查|待检查|待检查|2|
|life-reflection-list|memory?tab=reflection|待检查|待检查|待检查|3|
|life-reflection-detail|memory → 反思详情|待检查|待检查|待检查|2|
|life-self-list|memory?tab=self|待检查|待检查|待检查|3|
|life-self-detail|memory → 这条认识从哪里来|待检查|待检查|待检查|12|
|management-navigation|settings?tab=|待检查|待检查|待检查|22|
|management-model-services|settings?tab=models → services|待检查|待检查|待检查|12|
|management-model-roles|settings?tab=models → roles|待检查|待检查|待检查|4|
|management-conversation|settings?tab=conversation|待检查|待检查|待检查|2|
|management-vision|settings?tab=conversation → 图片理解|待检查|待检查|待检查|12|
|management-media|settings?tab=media|待检查|待检查|待检查|2|
|management-autonomy|settings?tab=autonomy|待检查|待检查|待检查|2|
|management-quiet|settings?tab=autonomy → quiet|待检查|待检查|待检查|2|
|management-notifications|settings?tab=runtime → notifications|待检查|待检查|待检查|12|
|management-runtime|settings?tab=runtime|待检查|待检查|待检查|2|
|management-advanced|settings?tab=advanced|待检查|待检查|待检查|4|
|management-diagnostic-tasks|settings?tab=diagnostics → tasks|待检查|待检查|待检查|8|
|management-diagnostic-detail|diagnostics → 任务详情|待检查|待检查|待检查|14|
|management-model-calls|diagnostics → calls|待检查|待检查|待检查|2|
|management-diagnostic-health|diagnostics → health|待检查|待检查|待检查|2|
|management-diagnostic-action-form|diagnostics → recovery confirmation|待检查|待检查|待检查|6|
|management-data-overview|settings?tab=data → 当前工作区|待检查|待检查|待检查|2|
|management-data-import|data → 导入工作区/合并确认|待检查|待检查|待检查|4|
|management-data-backups|data → 本地备份/为此刻留备份|待检查|待检查|待检查|4|
|management-data-restore|data → 恢复备份|待检查|待检查|待检查|4|
|management-data-demo-reset|data → 恢复最初演示状态|明确不适用|明确不适用|明确不适用|0|
|creation-world-existing|create → 生活世界 → 已有角色的世界|待检查|待检查|待检查|2|
|creation-world-published|create?worldResourceId=&worldVersion=|待检查|待检查|待检查|4|
|creation-world-daily|create → daily|待检查|待检查|待检查|20|
|creation-person-input|create → 认识角色|待检查|待检查|待检查|8|
|creation-blank|create → blank|待检查|待检查|待检查|22|
|creation-description|create → description|待检查|待检查|待检查|2|
|creation-json-import|create → json|待检查|待检查|待检查|2|
|creation-template|create?template=&version=|待检查|待检查|待检查|2|
|creation-analysis|create → 整理资料|待检查|待检查|待检查|8|
|creation-review-identity|create → preview identity|待检查|待检查|待检查|6|
|creation-review-personality|create → preview personality|待检查|待检查|待检查|7|
|creation-review-life|create → preview life|待检查|待检查|待检查|8|
|creation-review-relations|create → preview relationships|待检查|待检查|待检查|6|
|creation-review-json|create → preview json|待检查|待检查|待检查|9|
|creation-structured-editor|preview → 编辑分组|待检查|待检查|待检查|2|
|creation-activation|create → 准备相遇|待检查|待检查|待检查|24|
|creation-seven-stages|create → progress|待检查|待检查|待检查|24|
|creation-inventory-recovery|progress → 恢复原任务|待检查|待检查|待检查|2|
|creation-world-conflict|progress → 确认最新配置并重新应用|待检查|待检查|待检查|2|
|creation-visual-review|progress → 审阅候选参考图|待检查|待检查|待检查|2|
|creation-complete-navigation|progress → 去见角色/新相识|待检查|待检查|待检查|2|
|creation-start-snapshot|progress → 查看创建资料|待检查|待检查|待检查|2|
|creation-demo-scenarios|preview → 初始化演示场景|明确不适用|明确不适用|明确不适用|0|
|formal-life-timeline|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|formal-goal-planning|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|10|
|formal-diary-favorite|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|4|
|formal-self-governance|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|12|
|formal-config-extra|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|formal-diagnostic-export-clear|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|formal-data-import-pack|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|4|
|formal-data-revisions|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|formal-data-rollback-boundary|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|formal-backup-isolated-detail|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|4|
|formal-creation-rejected-copy|Demo不适用：正式已有能力，必须保留可达性|待检查|待检查|待检查|2|
|resources.world.directory|world → 世界目录|待检查|待检查|待检查|17|
|resources.world.settings|world → 详情 → 世界设定|待检查|待检查|待检查|4|
|resources.world.venues|world → 场地与地点|待检查|待检查|待检查|11|
|resources.world.place.editor|world → 地点详情/新建/编辑弹窗|待检查|待检查|待检查|2|
|resources.world.place.history|world → 地点 → 修订记录|待检查|待检查|待检查|10|
|resources.world.items|world → 物品目录 → 详情/配置|待检查|待检查|待检查|2|
|resources.world.events|world → 世界事件 → 详情|待检查|待检查|待检查|8|
|resources.wardrobe.garments|wardrobe → 全部衣服|待检查|待检查|待检查|13|
|resources.wardrobe.garment.editor|wardrobe → 服装详情/编辑|待检查|待检查|待检查|5|
|resources.wardrobe.outfits|wardrobe → 穿搭方案 → 预览|待检查|待检查|待检查|3|
|resources.wardrobe.compose|wardrobe → 搭配工作台|待检查|待检查|待检查|2|
|resources.wardrobe.tags|wardrobe → 管理穿搭场景弹窗|待检查|待检查|待检查|2|
|resources.wardrobe.wear|wardrobe → 方案 → 换装确认/结果条|待检查|待检查|待检查|12|
|resources.studio.directory|studio → 资源目录|待检查|待检查|待检查|13|
|resources.studio.editor.world|studio?kind=world → 新建/编辑|待检查|待检查|待检查|4|
|resources.studio.editor.character_template|studio?kind=character_template → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.item|studio?kind=item → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.outfit|studio?kind=outfit → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.place|studio?kind=place → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.npc|studio?kind=npc → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.event_template|studio?kind=event_template → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.persona_fragment|studio?kind=persona_fragment → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.editor.visual_style|studio?kind=visual_style → 新建/编辑|待检查|待检查|待检查|3|
|resources.studio.lifecycle|studio → 保存与发布/资源管理/发布版本历史|待检查|待检查|待检查|6|
|resources.studio.apply|studio → 应用弹窗/结果|待检查|待检查|待检查|12|
|resources.studio.creation|studio → 用于初始化/返回创建流程|待检查|待检查|待检查|4|
|resources.studio.sandbox|studio?mode=sandbox → 任务/结果/采用|待检查|待检查|待检查|4|
|resources.media.gallery|media → 媒体库|待检查|待检查|待检查|9|
|resources.media.detail|media → 图片详情弹窗|待检查|待检查|待检查|11|
|resources.media.upload|media → 上传图片|待检查|待检查|待检查|4|
|resources.media.visual.brief|media?tab=identity → 视觉定义编辑/修订|待检查|待检查|待检查|4|
|resources.media.visual.candidate|media?tab=identity → 候选/规范参考|待检查|待检查|待检查|6|
|resources.media.visual.review|media?tab=identity → 评审弹窗|待检查|待检查|待检查|6|
|resources.media.visual.sheet|Demo 视觉身份示例区(无独立 sheet 产物)|待检查|待检查|待检查|2|
|resources.media.generation|media?tab=generate → 表单/任务记录/结果|待检查|待检查|待检查|8|
|resources.media.avatar|media?tab=avatar → 显示头像|待检查|待检查|待检查|6|
|resources.formal.media.lifecycle|Demo 无完整媒体生命周期界面|待检查|待检查|待检查|6|
|resources.formal.studio.packs|Demo 无资源包完整工作流|待检查|待检查|待检查|4|

完整字段、状态、调用链、问题及证据路径见 [coverage.json](coverage.json)。


## 追加：反馈与异常状态UX维度（2026-10-29）

原177行与既有视觉/交互/业务记录保持；coverage.json每行feedback_ux含正常、空/首次、配置缺失、草稿恢复、读取/提交失败、异步进行中、失败/未知恢复、完成返回/刷新八场景。当前42行新增精确observations，212条受控浏览器实例；每行完整状态仍逐项验收，不批量标通过或N/A。

- [反馈增量验收报告与代表性前后截图](feedback-acceptance.md)
- [问题、实际恢复操作及验证台账](feedback-issues.md)
- [完整矩阵数据](coverage.json)
- [12批后续验收计划](research/feedback-coverage-plan.md)

任务继续in_progress，真实媒体生成由用户测试。


## r18增量

逐条结果见[r18-acceptance.md](r18-acceptance.md)；[64条处置](r18-issues.json)与[90条受控浏览器记录](evidence/r18-browser-index.json)已追加到coverage.json的74行具体观察中。原177行所有未验收状态与历史证据保留，没有批量提升整行或真实业务状态。
