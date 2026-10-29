# 2026-10-29 剩余确定状态补验清单（pass116之后）

这些是读过相邻脚本后定位的适用分支缺口，不是已发现bug；也不声称清单以外所有状态已逐张验收。已补齐前轮八组缺口（entry16、profile4、Studio类型14、wear10、Studio运行异常6），见对应report。仅用受控HTTP与实际生产组件，不增加真实模型或Comfy回归。

|组|矩阵ID|确定缺口|相邻脚本与生产定位|
|---|---|---|---|
|1 Shell|shell-account、shell-jump|退出等待/失败重试；快速跳转无结果|verify-shell-deep-ui.mjs；App.tsx accountBusy；WorkspaceQuickJump空结果|
|2 Chat|chat-send、chat-turn-recovery、chat-message-cards|composition输入、slow/settling与实际cancelled状态|verify-workspace-ui、verify-chat-inbox-avatar-ui；ChatPage/ChatTurnFeedback/chatWaitingPresentation|
|3 People|people-directory、people-detail/create、people-about-edit|目录200但分页/世界/关系不完整；只读角色；AboutYou invalid/saving/facts revision conflict|verify-profile-people-ui；PeoplePage71–94、AboutYouPanel144–188|
|4 Feed|feed-suggest、feed-formal-npc、feed-detail-comments|建议declined/failed/running与原回执恢复；NPC scope变化/completed；评论列表初读失败|verify-feed-goals-ui、verify-formal-edges-ui；FeedPage/FeedDetail|
|5 Inbox|inbox-list-read、inbox-back-chat|批量已读busy；空目录不显示返回角色聊天|verify-domain-ui、verify-chat-inbox-avatar-ui；InboxPage markAll/current role|
|6 Management|management-navigation/conversation/runtime、management-data-backups/restore/import、formal-backup-isolated-detail|共享settings save等待/失败/未知/冲突/部分确认；备份running/cancelled；导入校验中草稿变化、unknown与成功详情|verify-settings-studio、verify-retained-tools-ui、verify-admin-deep-ui；ManagementPage save/backup/import|

以上各组预计按共享行为压缩成约20个case，两视口约40条结果，不按每个设置字段机械复制。真实模型/Comfy候选到规范图/设定图/生成产物仍缺本轮完整证据，另在business-verification中保留，不用fixture代替。
