# R34 B 修复与验证报告

完成时间：2026-11-02 23:16 CST（用时约 12 分钟，30 分钟限时内）。未提交、未暂存、未改 Git 配置；未修改 /workspace/accept、任何 *.env 或图片文件。

## 逐项结果

| 项目 | 已确认根因与修复 | 主要文件 | 回归 |
| --- | --- | --- | --- |
| P1-2 头像 | Core 目录原来只补 group_id、没有投影选定头像；详情读回也未同步共享目录，部分调用把归一化 string 当对象。补 existing projectAvatar、详情同步及 string/object 消费；avatar:null 明确清除旧图。未选候选图不冒充头像。 | apps/core-go/internal/core/owner_surface_read.go；apps/web/src/state/workspace.mjs；react-app/pages/CharacterDetailPage.jsx；shared/AvatarWorkspace.jsx；v3/proto/data/profile.js；packages/browser-client 生成源/产物 | Core R34 avatar/inventory；workspace-bootstrap-recovery；r34-avatar-wardrobe；客户端 directory GET；Chrome chat/menu |
| FT- 占位初字 | 原来直接从原始名字取首字。新增统一 avatarInitial，去除显示初字来源中的开头 FT-，普通显式 initial（+、岚）保持；业务姓名正文不改写。 | apps/web/src/lib/avatar-presentation.mjs；shared/CharacterAvatar.tsx；v3/proto/components/avatar.js；state/workspace.mjs | 前缀与显式初字正负例；Chrome FT-清雾 占位“清” |
| P1-3 通知键 | studio.generate 未注册中文名称，通知待处理卡回退原始键。补共用 domainName 映射，包含失败阶段。 | apps/web/src/v3/domain-presentation.mjs | r34b-data-display；Chrome notification-chinese |
| P2-1 技术详情 | 错误反馈内展开 pre，挤占聊天。改既有 DetailDrawer→shadcn 右 Sheet：状态、原因、关联标识和运行入口，原始 JSON 默认折叠。 | apps/web/src/react-app/components/ChatTurnFeedback.tsx | r34b-data-display：right side、无默认 pre、展开、Escape、返回原焦点、0重试/0读取；feedback-semantics；Chrome 默认与展开截图 |
| P2-2 动态图片与作者 | 固定高度 cover 裁掉图中脸部。改 centered contain 与有界高度，完整保留图片/脸部。作者按真实 publisher：user 显示“我”、fluctlight 显示角色名；不能把归属角色误作作者，原有语义正确，补回归保留。 | apps/web/src/styles/replica-feed.css；作者代码 apps/web/src/approved-ui/feed-presentation.mjs、shared/FeedPost.jsx（未修改） | r34b-data-display、demo-replica-feed；Chrome 600×900 双脸和底部边界完整、两种发布者 |
| P2-3 世界与角色封面 | WorldArt 原来只按 name/flavor 三选一，同类实体共享插画。改 own world.id / character.id / resource.id 稳定 seed 配色与几何，排序不改变，世界列表/详情一致。 | apps/web/src/react-app/shared/WorkspaceArtwork.tsx；views/WorldPage.jsx；pages/LibraryPage.jsx；views/CharacterWizard.jsx；views/StudioDirectory.jsx | demo-replica-world/library/creation/studio；Chrome 三个同名世界和角色库封面 |
| P2-3 午后阵雨事件 | Core 合法公共环境事件为 fluctlight_id:null，UI 却要求等于当前角色，导致列表/详情拒绝。新增严格 ambient 分支，保留异角色/异世界拒绝。未应用 Studio 底稿不会被误当发生事实。 | apps/web/src/approved-ui/world-event-scope.mjs；react-app/shared/WorldRuntimePanels.jsx | world-event-scope；demo-replica-world 的 FT-午后阵雨 列表/详情；Core shared_worlds 实际投影；Chrome 列表/详情 |
| P2-4 衣柜 | 页面忽略 Core item.media，始终绘制类别示意，即使已有关联图也不请求。现按每件自己的关联素材取图，保留 API origin；区分资料未知、无关联、实际取图与取图失败，详情可显式重试。 | apps/web/src/react-app/shared/WardrobeItemArt.tsx；views/WardrobePage.jsx；styles/replica-resources.css | r34-avatar-wardrobe、demo-replica-wardrobe、Core item reference isolation；Chrome 已关联/无关联/404及404→图片重试恢复 |

## 验证命令与结果

Node 使用 `/tmp/node-v24.12.0-linux-x64/bin`。系统 corepack 默认缓存只读，采用现成 `/tmp/r34-corepack/pnpm/11.0.7/bin/pnpm.cjs` 执行 package scripts。

- `pnpm --filter @fluctlight/web lint`：PASS。
- `pnpm --filter @fluctlight/web typecheck`：PASS。
- `pnpm --filter @fluctlight/browser-client typecheck`：PASS。
- `pnpm --filter @fluctlight/web build`：PASS，production fixture guard 与 React-only graph guard 通过。Vite 保留现有大 chunk 提示。
- Web targeted JS：115/115 PASS，0 skipped。文件：r34b-data-display、r34-avatar-wardrobe、workspace-bootstrap-recovery、world-event-scope、demo-replica-world/library/wardrobe/creation-ui/studio/feed、feedback-semantics、t00-profile、v3-display-avatar。
- BrowserClient `node --import tsx --test test/r34-avatar-directory.test.ts`：1/1 PASS。
- Go Core `go test ./internal/core -run '^(TestR34DirectoryDisplaysSelectedReadyAvatarAndWardrobeKeepsOwnMedia|TestWorldEventGenerationRequiresKevAndRunsOncePerWorldHour)$' -count=1`：PASS。
- Go Browser `go test ./internal/httpapi/browser -run '^(TestBrowserRouteMatrixMatchesOpenAPIArtifact|TestEveryBrowserOpenAPIRouteReachesTheInProcessBoundary)$' -count=1`：PASS。
- Go 使用用户提供的 GO_CORE_TEST_DATABASE_URL 和 GOCACHE；测试 helper CREATE/DROP 自有独立 `lac_core_*` 数据库，未清理共享库。只读复核主测试库仍有 8 条 fluctlights，0 worlds，0 moments。
- 生产 dist + 自有 loopback HTTP fixture + headless Google Chrome：1440×1000 和 390×844，28 项检查 PASS，26 张截图；0 pageerror，检查页面无横向溢出。截图目录 `/tmp/r34-shots`；脚本 `verify-browser.mjs`，结果 `browser-results.json`。图片均在仓库外。
- `git diff --check`：PASS；暂存区空；无图片/env 变更。

## 断言变更

既有测试断言修改/删除/放宽：**0**。既有测试文件仅追加场景；新文件新增断言。

运行时归属校验因合法共享事件新增受限分支，理由是 Core 明确允许环境事件没有角色 Life 写入。仅 `world_id` 精确匹配、`fluctlight_id===null`、`record_kind===world_event_execution`、`context_scope===ambient`、`source===world_execution`、`status===applied`、`life_write===false` 同时成立才通过；未知来源、排队事件、异角色、异世界仍拒绝。新增正负例及真实 Core 投影断言，不移除原校验保护。

## 复用组件

shadcn/ui：Avatar / AvatarImage / AvatarFallback、Sheet（经 DetailDrawer）、Collapsible（经 Disclosure）、Button、Card、Badge；既有页面继续使用 Tabs、Dialog。没有新增图片资产或替代原生工具协议。

## 实际数据验证边界

允许的 live GET `http://127.0.0.1:13001/api/fluctlights` 返回 unauthenticated；未登录。指定测试 PG 只有 test-wakeup-* 样本，没有截图中的青岚、午后阵雨及其素材。因此代码缺陷和隔离复现已确认，但截图原始行是否已选头像、衣物是否生成、FT-午后阵雨是否已发布/应用，无法在当前只读权限下核实。“无关联图”不等于“从未生成”，没有虚构这些事实或改写线上数据。
