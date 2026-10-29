# React 全站反馈调用点审计（静态，2026-10-29）

仅写本研究；未启动服务、未修改产品。LibraryPage / CharacterWizard 调用点登记但恢复语义深挖由另一探子负责。

## 全量检索范围与局限

扫描 `apps/web/src/react-app` 全部 JSX/TSX/JS/TS。JSON 保留每个反馈节点的完整原文、variant、语义候选、真实按钮原文及 catch 周边；候选表示需要结合状态生产方确认，不能作为直接批量换色指令。

共 361 个反馈 JSX 节点，185 个 catch 声明。节点分类：`{'LoadingState': 49, 'CardSurface': 1, 'ReadState': 4, 'InlineFeedback': 84, 'div': 6, 'p': 32, 'OperationStatus': 10, 'Alert': 19, 'small': 2, 'MessageContent': 1, 'WorkspaceInfo': 111, 'EmptyState': 40, 'span': 2}`。`AlertDialog` 为确认操作，不计作状态提醒。Toast 与独立 notice 见下文。

## 逐调用点清单

| 位置 | 符号 | 当前 variant | 六类语义候选（变量须核对生产方） | 动作 |
|---|---|---|---|---|
| apps/web/src/react-app/App.tsx:93 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/App.tsx:182 | CardSurface | implicit:n/a | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/App.tsx:192 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/App.tsx:207 | ReadState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/App.tsx:277 | LoadingState | implicit:n/a | progress, error, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/App.tsx:281 | InlineFeedback | implicit:destructive | error, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/App.tsx:286 | div | implicit:n/a | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/components/ChatTurnFeedback.tsx:29 | InlineFeedback | "default" | progress | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/components/ChatTurnFeedback.tsx:31 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/components.tsx:68 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/components.tsx:108 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/components.tsx:119 | Alert | "destructive" | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/components.tsx:127 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/components.tsx:138 | InlineFeedback | implicit:destructive | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/components.tsx:143 | InlineFeedback | implicit:destructive | action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/AuthPage.tsx:136 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/AvatarPage.tsx:36 | LoadingState | implicit:n/a | progress, action-required, error, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/AvatarPage.tsx:36 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/AvatarPage.tsx:36 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/BrowsePage.jsx:216 | small | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/CharacterToolsPage.tsx:199 | ReadState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/CharacterToolsPage.tsx:201 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/CharacterToolsPage.tsx:375 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/ChatPage.tsx:310 | MessageContent | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/ChatPage.tsx:326 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiagnosticsDetailPage.tsx:85 | ReadState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiagnosticsDetailPage.tsx:87 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:597 | WorkspaceInfo | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:705 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:706 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:708 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:710 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:752 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:754 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:774 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:847 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:849 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:926 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DiaryPage.jsx:934 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:936 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/DiaryPage.jsx:977 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/DomainPage.tsx:48 | InlineFeedback | implicit:destructive | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/GoalsPage.jsx:900 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/GoalsPage.jsx:904 | InlineFeedback | implicit:destructive | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/GoalsPage.jsx:915 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/GoalsPage.jsx:970 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/GoalsPage.jsx:1078 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/GoalsPage.jsx:1080 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/GoalsPage.jsx:1146 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/GoalsPage.jsx:1184 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/GoalsPage.jsx:1186 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/GovernancePage.tsx:36 | Alert | implicit:default | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/InboxPage.jsx:34 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/InboxPage.jsx:34 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/InboxPage.jsx:58 | InlineFeedback | "default" | warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/InboxPage.jsx:59 | InlineFeedback | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/InboxPage.jsx:60 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/InboxPage.jsx:61 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/LibraryPage.jsx:133 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/LibraryPage.jsx:134 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/LibraryPage.jsx:140 | LoadingState | implicit:n/a | progress | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/LibraryPage.jsx:153 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/ManagementPage.jsx:938 | OperationStatus | "default" | action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/MemoryPage.jsx:151 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:153 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/MemoryPage.jsx:499 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:697 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:761 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:764 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/MemoryPage.jsx:814 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/MemoryPage.jsx:936 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:1044 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:1185 | WorkspaceInfo | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:1290 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/MemoryPage.jsx:1375 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:768 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:780 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:824 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:827 | InlineFeedback | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:867 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:874 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:941 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:952 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1010 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1012 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:1081 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:1117 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1119 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1289 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1293 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1296 | p | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1408 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1414 | InlineFeedback | "default" | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/pages/SchedulePage.jsx:1447 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1494 | InlineFeedback | implicit:destructive | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1499 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SchedulePage.jsx:1553 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/SelfPage.tsx:22 | Alert | implicit:default | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/StudioMaterialPage.tsx:83 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/StudioMaterialPage.tsx:87 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/TaskDetailPage.tsx:31 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/TaskDetailPage.tsx:31 | ReadState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/pages/TaskDetailPage.tsx:34 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:222 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:268 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:324 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:326 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:478 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:509 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AboutYouPanel.jsx:557 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AvatarWorkspace.jsx:315 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AvatarWorkspace.jsx:332 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/AvatarWorkspace.jsx:339 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ChatAttachment.tsx:12 | span | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ChatAttachment.tsx:13 | span | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/CommandDialog.tsx:17 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/CommandDialog.tsx:17 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ConversationHistory.tsx:59 | LoadingState | implicit:n/a | progress, action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/ConversationHistory.tsx:60 | LoadingState | implicit:n/a | progress, action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/ConversationHistory.tsx:61 | InlineFeedback | implicit:destructive | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/ConversationHistory.tsx:62 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/CursorControls.tsx:10 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/CursorControls.tsx:12 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticRecords.tsx:28 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:24 | LoadingState | implicit:n/a | progress, error, success | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:24 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:24 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:25 | LoadingState | implicit:n/a | progress, error, success | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:25 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx:25 | InlineFeedback | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/DiaryGeneration.jsx:174 | OperationStatus | "default" | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/DiaryGeneration.jsx:193 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/DomainPrimitives.tsx:58 | Alert | {port.variant /  / 'default'} | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/DomainPrimitives.tsx:143 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedDetail.jsx:122 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedDetail.jsx:303 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedDetail.jsx:305 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedMediaPicker.jsx:84 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedParts.jsx:7 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/FeedParts.jsx:9 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/FeedPost.jsx:19 | div | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/FeedPost.jsx:60 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/LoadingState.tsx:2 | div | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaDetail.jsx:224 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:184 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:217 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/MediaGeneration.jsx:240 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:435 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:497 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:509 | WorkspaceInfo | implicit:n/a | success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:511 | p | implicit:n/a | action-required, warning, success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGeneration.jsx:513 | WorkspaceInfo | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGrants.jsx:228 | WorkspaceInfo | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaGrants.jsx:284 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaLifecycle.jsx:193 | WorkspaceInfo | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaParts.jsx:13 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/MediaParts.jsx:15 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/MediaParts.jsx:47 | div | implicit:n/a | error, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaParts.jsx:80 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/MediaPreview.tsx:4 | Alert | "destructive" | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/MediaUpload.tsx:31 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/MediaUpload.tsx:31 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ModelDirectory.jsx:6 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ModelDirectory.jsx:6 | div | implicit:n/a | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/PeopleParts.jsx:8 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/PeopleParts.jsx:10 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/PersonDefinition.jsx:260 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonDefinition.jsx:481 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonDefinition.jsx:501 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonDetail.jsx:424 | WorkspaceInfo | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonDetail.jsx:453 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonDetail.jsx:489 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PersonRecords.jsx:239 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/PreparationSteps.jsx:102 | InlineFeedback | {                           stage.state === "failed" ? "destructive" : "default"                         } | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/Presentation.tsx:8 | Alert | {variant} | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/Presentation.tsx:11 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:60 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:62 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:386 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:455 | WorkspaceInfo | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:507 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:523 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:553 | WorkspaceInfo | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:638 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/ProfileGovernance.jsx:668 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/RecordShell.tsx:5 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/RecordShell.tsx:5 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/RequestStatus.jsx:159 | LoadingState | implicit:n/a | progress, action-required, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/RequestStatus.jsx:191 | p | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/RequestStatus.jsx:224 | p | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/RequestStatus.jsx:228 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioApply.jsx:132 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioApply.jsx:134 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioApply.jsx:139 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioApply.jsx:167 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioApply.jsx:307 | WorkspaceInfo | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:61 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:116 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:134 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:222 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:234 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:239 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:289 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:455 | p | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:685 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioFields.jsx:709 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioLifecycle.tsx:280 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioLifecycle.tsx:311 | Alert | implicit:default | action-required, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/StudioLifecycle.tsx:430 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioLifecycle.tsx:436 | p | implicit:n/a | action-required, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/StudioLifecycle.tsx:495 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/VisualWorkspace.jsx:533 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/VisualWorkspace.jsx:568 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/VisualWorkspace.jsx:569 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/VisualWorkspace.jsx:571 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/VisualWorkspace.jsx:705 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WardrobePreparation.jsx:110 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WearingSummary.tsx:5 | p | implicit:n/a | error, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WearingSummary.tsx:5 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkflowRecovery.tsx:31 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkflowRecovery.tsx:31 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkflowRecovery.tsx:34 | Alert | implicit:default | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/WorkflowRecovery.tsx:39 | Alert | "destructive" | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkspaceCommandDialog.jsx:146 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkspaceCommandDialog.jsx:149 | InlineFeedback | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkspaceInfo.tsx:5 | div | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorkspaceQuickJump.tsx:18 | p | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldFields.jsx:33 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/WorldFields.jsx:35 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/shared/WorldFields.jsx:102 | WorkspaceInfo | implicit:n/a | action-required, success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldPlaceHistory.jsx:166 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldPlaceHistory.jsx:204 | WorkspaceInfo | implicit:n/a | success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:239 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:242 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:286 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:350 | WorkspaceInfo | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:635 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:691 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/shared/WorldRuntimePanels.jsx:867 | WorkspaceInfo | implicit:n/a | success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/BrowsePage.jsx:152 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/BrowsePage.jsx:313 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:358 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:361 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:369 | WorkspaceInfo | implicit:n/a | progress, success | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:390 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:402 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:405 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:626 | EmptyState | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:665 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:737 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:740 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:807 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:809 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:851 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:853 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:883 | EmptyState | implicit:n/a | error, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:1027 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:1165 | WorkspaceInfo | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:1168 | WorkspaceInfo | implicit:n/a | action-required, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:1196 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterDetailPage.jsx:1199 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:312 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:322 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:331 | WorkspaceInfo | implicit:n/a | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:416 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:419 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:424 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:445 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:540 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:549 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:557 | LoadingState | implicit:n/a | progress | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:705 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:710 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:724 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:873 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:878 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:891 | LoadingState | implicit:n/a | progress, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:915 | WorkspaceInfo | implicit:n/a | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:960 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:973 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1154 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1242 | WorkspaceInfo | implicit:n/a | action-required, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:1248 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1306 | WorkspaceInfo | implicit:n/a | action-required, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1310 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1317 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/CharacterWizard.jsx:1591 | WorkspaceInfo | implicit:n/a | warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/CharacterWizard.jsx:1684 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/FeedPage.jsx:117 | p | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/FeedPage.jsx:120 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/FeedPage.jsx:131 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/LifePage.jsx:175 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/LifePage.jsx:217 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/LifePage.jsx:303 | EmptyState | implicit:n/a | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/LifePage.jsx:317 | small | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/LifePage.jsx:377 | EmptyState | implicit:n/a | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/LifePage.jsx:420 | InlineFeedback | implicit:destructive | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:66 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:67 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/ManagementPage.jsx:170 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:424 | OperationStatus | "default" | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:431 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:435 | p | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:459 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:459 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/ManagementPage.jsx:460 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:460 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/ManagementPage.jsx:476 | p | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/ManagementPage.jsx:476 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/ManagementPage.jsx:502 | InlineFeedback | implicit:destructive | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/MediaWorkspace.jsx:243 | WorkspaceInfo | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/MediaWorkspace.jsx:250 | WorkspaceInfo | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/MediaWorkspace.jsx:257 | WorkspaceInfo | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/NowPage.jsx:21 | LoadingState | implicit:n/a | progress, error, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/NowPage.jsx:23 | LoadingState | implicit:n/a | progress, action-required, error, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/NowPage.jsx:23 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/NowPage.jsx:24 | EmptyState | implicit:n/a | progress | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/PeoplePage.jsx:25 | WorkspaceInfo | implicit:n/a | action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/PeoplePage.jsx:190 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/PeoplePage.jsx:208 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioDirectory.jsx:157 | WorkspaceInfo | implicit:n/a | action-required, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioDirectory.jsx:167 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioEditor.jsx:91 | InlineFeedback | implicit:destructive | action-required, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioEditor.jsx:102 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioEditor.jsx:103 | LoadingState | implicit:n/a | progress, action-required, warning | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioEditor.jsx:107 | WorkspaceInfo | implicit:n/a | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioEditor.jsx:122 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioJobs.jsx:63 | InlineFeedback | implicit:destructive | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:67 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:181 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:187 | InlineFeedback | implicit:destructive | action-required, error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:192 | WorkspaceInfo | implicit:n/a | action-required, warning | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioJobs.jsx:215 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/StudioJobs.jsx:226 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:278 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:329 | WorkspaceInfo | implicit:n/a | action-required | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/StudioJobs.jsx:353 | WorkspaceInfo | implicit:n/a | action-required, warning, success | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WardrobePage.jsx:119 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:121 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:335 | LoadingState | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WardrobePage.jsx:407 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:409 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:472 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:607 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WardrobePage.jsx:697 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WardrobePage.jsx:704 | InlineFeedback | implicit:destructive | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WardrobePage.jsx:707 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WardrobePage.jsx:1102 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:132 | LoadingState | implicit:n/a | progress, error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:134 | InlineFeedback | implicit:destructive | error | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:141 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:142 | InlineFeedback | implicit:destructive | error | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:144 | WorkspaceInfo | implicit:n/a | action-required | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:156 | WorkspaceInfo | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:176 | EmptyState | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:274 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:491 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:500 | EmptyState | implicit:n/a | information | 有按钮；见 JSON 原始 handler |
| apps/web/src/react-app/views/WorldPage.jsx:600 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:627 | WorkspaceInfo | implicit:n/a | progress | 本节点无按钮；可能使用邻近按钮 |
| apps/web/src/react-app/views/WorldPage.jsx:636 | WorkspaceInfo | implicit:n/a | information | 本节点无按钮；可能使用邻近按钮 |

## 已核实、可直接定位的具体缺陷（没有运行浏览器）

1. **默认值同时传播 destructive 外观和隐含 alert 语义。** `shared/Presentation.tsx:7` 的 `variant='destructive'`；底层 `apps/web/src/components/ui/alert.tsx:34` 默认 `role="alert"`。因此没写 role 的信息块也会强播报。具体误分：`components.tsx:138` 的“查看等待核对的原提交”只是带用户原输入的 Disclosure，应 information/action-required；`:143` 已明确被拒绝的 CAS 草稿核对应 action-required；`views/LifePage.jsx:420` 空记忆字段应 action-required 且需要与字段关联；`pages/SchedulePage.jsx:1494` 已有日程保护应 warning/action-required。不能全体改成 default：绝大多数读取失败仍是 error。组件现有 API 只有 default/destructive，无法完整表达六类状态。
2. **诊断调用页同事件两份错误与两份重试。** `views/ManagementPage.jsx:476` 渲染 `diagnosticPages[kind].error` 的 InlineFeedback，随后同一行 CursorControls 传同一个 error/retry；`shared/CursorControls.tsx:10` 又渲染一次。错误出现时确定重复；保留靠近对应列表的一处即可。两者 handler 都是真实 `loadMoreDiagnostics`，不能靠删除业务状态解决。
3. **成功基线核对进入 error 通道。** `shared/StudioLifecycle.tsx:328` 在 GET 最新资源、`releaseRejected()`、`onChanged(value)` 完成后调用 `setError("已采用最新管理版本，当前输入仍保留。")`，而 `:280` 将 error 画成 destructive Alert。实际是 success/information；失败 catch 继续是 error，必须拆状态，不能整体变色。核对 handler 真实，不应改 CAS/原 key。
4. **同一次连接目录失败双提示。** `pages/ManagementPage.jsx:712` 同时 `endpoint.feedback = errorMessage(cause)` 和 `endpoint.modelsError=endpoint.feedback`；`views/ManagementPage.jsx:160` ModelDirectory 显示 modelsError/重试，`:170` 又用 OperationStatus 显示 h.feedback。一个是真正 error，另一个中性 status，内容重复且语义冲突。非 endpoint 的连接测试也把测试中、成功、失败全存 `testResult.value` (`pages/ManagementPage.jsx:670,703,713`)；`views/ManagementPage.jsx:424` 与 `shared/DiagnosticsWorkspace.jsx:25` 一律 default/status。应按真实测试状态分类，不从错误文案匹配。
5. **上传完成提示在选择新文件后过期。** `shared/MediaUpload.tsx:22` 的 `select(next)` 只更新 file/controller/error，不清 status；`:24` 的“上传已保存”会与新选文件同时出现，直到 run() 开始。应清理上次操作提示（保留 controller.pending/lastAsset）。上传完成但列表刷新失败会同时显示 success/status 和 destructive/error (`:24,:31`)，属于两个事实；建议合成局部 warning“素材已保存，列表未刷新”并提供真实重新读取列表动作，不能重传文件代替 GET。
6. **通知 mark 失败提示可能过期且混合语义。** `pages/InboxPage.jsx:52` mark 成功只在 notificationCountError 为真时写 message，普通成功不清已有 message；此前失败 message 会保留。`:59` 所有 message 都 default/status，实际有“已读保存成功+计数刷新失败”(warning)、“已读状态暂未确认”(warning/action-required) 和 markAll catch 的操作失败(error)。当前 message 无独立重试；标为已读按钮本身是真实动作。`message` 还会隐藏 `:58` 自带重新读取计数按钮的状态块，导致计数故障信息变成无直接恢复动作的拼接句。
7. **目标评估状态混在 note，且终态提示不清理。** `pages/GoalsPage.jsx:413-455` evaluationNote 写入 progress、已结束、failed、读取失败、自动跟进暂停；`:900` 一律 WorkspaceInfo(role=note)。读失败拼 `errorMessage`，failed 只说查看记录，应有真实目标/评估入口与当前恢复按钮。终态 `setEvaluationPointer(null)` 不清 evaluationNote，因此成功/失败提示继续占当前列表顶部；应随相关目标 scope 和真实恢复更新，后台任务保持原样。这里不是删除持久化请求身份的建议。
8. **ActionFeedback 成功 toast 缺事件消费/去重，重新挂载会再次提示历史成功。** `components.tsx:103` useEffect 看到 latest.success/requestId 即 toast，没有事件消费或 toast id；切换页面/Tab/打开另一个 ActionFeedback 重新挂载时可重发。`views/CharacterDetailPage.jsx:676,1029,1201,1239` 多个使用点按面板条件挂载；有些同时局部 status+toast。`:108` showStatus 对所有非error状态持久显示，成功后的历史 response 不会自动过期。确定缺乏事件去重；实际重复可见次数需浏览器复现（组件 mount 时机受 Tabs/Dialog 影响）。错误 `latest.run?.()` 按钮还允许 handler 缺失时显示“重试”，是否真的存在缺 handler 的 operation 记录需补查 state/workspace 命令生产方；不能假定该按钮坏了。
9. **实际人物操作失败被当信息注释。** `shared/PersonDetail.jsx:306` `record()` catch 写 error，`:424` 用 WorkspaceInfo(role=note) 展示，属于 error，应靠近“记录实际接触”入口并有真实恢复动作。edit `:145` 清 error，但 record 开始/成功路径不清已存在错误；重复打开记录操作时旧错可能保留（需浏览器核实具体对话框时序）。
10. **衣柜准备读取错误确定在阶段与额外 Alert 重复。** `shared/WardrobePreparation.jsx:24` 将 readError 交 `wardrobePreparationStage`，`:110` 又显示同 readError；PreparationSteps `:102` 对 stage.note 也会展示。已点验 `approved-ui/wardrobe-preparation.mjs:6`：`note:readError || (...)`，故 `PreparationSteps:102` 与 `WardrobePreparation:110` 会显示完全同一个错误。不要删准备业务状态。请求失败还可能同时出 ActionFeedback `:114`。阶段失败应到真实准备任务/配置，读取失败留局部读重试。
11. **生成回执统一中性会弱化任务失败。** `shared/DiaryGeneration.jsx:174` `diaryRequestText(receipt)` 一律 default/status；已点验 `approved-ui/diary-generation.mjs:8` 明确映射 pending/running/completed/no_content/failed/cancelled；应读取实际 status 对 progress/success/error 分类。已有 workflow_id 链接是真实任务入口。`:193` 单独读取失败是 error；日期选择失败 `:77` 则 action-required。不能将“已受理”当“已完成”。`shared/RequestStatus.jsx:191` tasks_state==='failed' 用 status 显示关联任务读取不全，属于 warning/error（按是否有旧数据区分），刷新原请求按钮真实。
12. **提示层级正确但仍需复现/收口的全局项。** `App.tsx:286` directory/session 是全工作区故障，保留全局合理；`:281` 每页 readError 可能与页面本地 reader 反馈共存，但此刻已过滤 schedule (`:234`)，其余页是否同一错误需按 reading key 实测，不能静态认定全部重复。`App.tsx:169` 全局 state.toast 未消费也未 toast id，route 依赖变化时可能重新弹；需验证 navigate 引用稳定性。`App.tsx:79` toast adapter 只识别 err/ok/success，其他 type 归普通 toast；应审计调用方真实 type 后扩展。

## 应保留、不要误修的现有能力

- `components/ChatTurnFeedback.tsx:20-50` 区分部分已执行不可直接重试与 retryable；动作是真实 `retryMessage(message.id)` / `refreshActiveTurn(true)`，不能为了统一按钮而强制重发。外观 neutral 但 role=alert：error/取消/warning 需按原 `turnErrorCode` 分类。
- 各 Reader 的“重新读取原页”绑定 source.retry 或 cursor retry，是读恢复；错误清理由各 hook 的成功读负责。`shared/RequestStatus.jsx` 原请求JSON已在 Disclosure，技术记录不应直接删。
- `shared/PreparationSteps.jsx:102` 已按 stage.state 动态 variant，必须保留；stage.note 可能是技术失败原因，也可能含业务说明，需明确诊断展开，不能改写用户输入。
- `shared/StudioFields.jsx:289,455,685` JSON结构异常提示在编辑区域，原定义保留；action-required/warning，而非删除不可识别字段。邻近完整JSON入口是恢复路径，不能重置内容。
- `pages/SchedulePage.jsx:1414` 原规划待核对有真实 submitPlan(true)/reviewPlanOriginal；六类为 warning/action-required，不得把未知结果当失败后开新请求。已有记录保护 `:1494` 需要保留业务禁用。
- `shared/MediaGeneration.jsx:497,511` 实际失败与“记录完成但资产未确认可读”是不同状态，后者 warning/read recovery；`:509` 成功是实际资产已读回，可 success。不要按回执 completed 直接报成功。

## 测试与 fixture 抓手（已定位，未运行）

- `apps/web/test/demo-replica-management.test.mjs:10,28,46,63,80`：配置草稿、durable command accepted 后 GET恢复、晚回执、Owner切换、native history 原cursor/返回草稿；可新增双错误/旧成功重现断言。
- `apps/web/test/workspace-command-readback.test.mjs:6,66`：已受理 direct-write 只读恢复、旧A回执不覆盖新B输入。共享反馈修改必须保留。
- `apps/web/test/workspace-bootstrap-recovery.test.mjs:23,41`：bootstrap恢复只清自己的failure，未登录清启动旧错。
- `apps/web/test/demo-replica-goals.test.mjs:257,309,332,377,439,479,497`：initial loading/历史读取重试、pending不完成目标、未知结果保原body/key、accepted只GET、回执版本核对。
- `apps/web/test/goal-planning-recovery.test.mjs:24,27,30,44,53`：accepted只GET、lost response原key/body、CAS草稿、accepted规划不等于完成。
- `apps/web/test/schedule-receipt-recovery.test.mjs:5,94`：异常/外来回执保原身份、首次明确CAS拒绝才允许rebase。
- `apps/web/test/v3-media-upload.test.mjs:7,14,19,24,28`：上传丢回执按原key核对、准确同文件要求、Owner晚回执阻断、超限文件；不覆盖 MediaUpload.select 的过期status，可加入UI断言。
- fixtures：`apps/web/test/fixtures/{react-workspace,goals,goal-planning,schedule,diary,media,studio}-fixture.mjs`；`demo-replica-{chat,diary,schedule,media,studio}.test.mjs` 已有页面夹具。

## 静态与浏览器边界

本探子没有桌面/390px截图，没有替代主线程最终验证。上面“确定”的重复渲染/混合状态代码分支可直接修改；文字溢出、隐藏Tab实际挂载、toast重新出现次数、跨角色/返回过期提示需用现有fixture驱动浏览器。JSON的semantic_candidates为搜索辅助，最终业务分类以上述状态生产方判定为准；不能将其机器候选视为全量验收已经完成。

补充JSON中另有399行其他信号索引，覆盖toast/notice状态生产方、错误赋值、destructive操作和aria-live；它是源代码定位索引，不能把每个命中视为错误提示。JSON文件可能被仓库ignore规则过滤，交付时需要显式保留本研究文件。
