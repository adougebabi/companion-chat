# R4 修复与验证结果

五项仓库修复已完成；未提交 Git，未暂存文件，未修改 `/workspace/accept`、`*.env` 或图片。

| 项目 | 实现文件（相对仓库） | 新增测试（均在 `apps/web/test/r4-ui-fixes.test.mjs`） | shadcn/ui |
| --- | --- | --- | --- |
| P1 部署后旧 chunk 404 白屏 | `apps/web/nginx.conf`、`apps/web/src/main.tsx`、`apps/web/src/react-app/shared/ChunkLoadBoundary.tsx` | HTML/SPA 缓存与资源 404；真实 React.lazy 拒绝后跨挂载只刷新一次；手动刷新；浏览器错误类型与 sessionStorage 不可用 | Alert（通过 Feedback）、Button |
| P2 D8 世界选择 | `apps/web/src/react-app/views/CharacterWizard.jsx`、`apps/web/src/styles/replica-creation.css` | 两个初始未选 radio；明确选择后才可继续；新建底稿阻止沿用旧选择前进；返回说明与链接；草稿保持；键盘选择 | RadioGroup、RadioGroupItem；复用 Button、Alert |
| P2 角色库模型未绑定 | `apps/web/src/react-app/pages/LibraryPage.jsx` | 无服务、服务已保存但用途未绑定、已绑定、配置读取失败四种状态与对应链接 | Empty、Button |
| P2 世界列表空态 | `apps/web/src/react-app/views/StudioDirectory.jsx` | 无数据只显示一个创建 CTA；搜索与归档筛选才显示筛选文案；清除筛选恢复；固定的 world 类型不算用户筛选 | Empty（通过 EmptyState）、Button |
| P2 诊断任务空态 | `apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx` | 无任务显示创建角色下一步链接；筛选无结果保留清除筛选操作 | Empty、Button |

组件均已存在，无需安装依赖。没有更改 backend、LLM 提示词、token 配额、API 或业务命令。

## 验证

- 指定 Node：`/tmp/node-v24.12.0-linux-x64/bin/node`，v24.12.0。
- 新增目标测试：8/8 通过。
- 既有相关测试：75/75 通过，仅运行以下八个文件：
  - `r20-creation.test.mjs`
  - `demo-replica-creation-ui.test.mjs`
  - `demo-replica-library.test.mjs`
  - `demo-replica-studio.test.mjs`
  - `diagnostic-workspace.test.mjs`
  - `d12-world-tasks.test.mjs`
  - `r23-ui.test.mjs`
  - `r22-reading-ui.test.mjs`
- 最后扩充旧的禁止创建文案断言后，`r20-creation.test.mjs` 再跑 10/10 通过。
- Web typecheck、lint、build 全部通过。build 同时通过生产 fixture 排除和 React 图检查（395 个本地模块），有 Vite 大 chunk 提示。
- `git diff --check` 通过。
- 生产 dist + 隔离 HTTP fixture 的目标 Chrome 检查：9 项记录通过。1920×1113 和 390×844 均无页面横向溢出；检查整个卡片点击、键盘选择、配置链接、向导返回、世界空态筛选、诊断下一步。
- 桌面世界卡片均为 566.5×293.484375px，窄屏均为 172×232.390625px。
- 浏览器让真实 `AuthPage-*.js` 连续返回两次 404，断言只有一次自动重载；恢复资源后手动重载成功进入登录表单。
- 浏览器证据：本任务 `browser-results.json`；截图仅位于 `/tmp/r4-ui-shots`，未进入仓库。

命令使用已缓存的 pnpm 11.0.7 入口运行，避免系统 Corepack 写入只读路径：

```sh
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node --test apps/web/test/r4-ui-fixes.test.mjs
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node --test apps/web/test/r20-creation.test.mjs apps/web/test/demo-replica-creation-ui.test.mjs apps/web/test/demo-replica-library.test.mjs apps/web/test/demo-replica-studio.test.mjs apps/web/test/diagnostic-workspace.test.mjs apps/web/test/d12-world-tasks.test.mjs apps/web/test/r23-ui.test.mjs apps/web/test/r22-reading-ui.test.mjs
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node /tmp/r23-corepack/pnpm/11.0.7/bin/pnpm.cjs --filter @fluctlight/web typecheck
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node /tmp/r23-corepack/pnpm/11.0.7/bin/pnpm.cjs --filter @fluctlight/web lint
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node /tmp/r23-corepack/pnpm/11.0.7/bin/pnpm.cjs --filter @fluctlight/web build
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node .trellis/tasks/11-02-r4-ui-fixes/verify-browser.mjs
```

验证边界：nginx 使用目标配置断言验证缓存指令；浏览器使用真实生产 bundle 和隔离 fixture，不代表已修改部署主机或验证真实数据库、模型服务。Dockerfile 已把修改的仓库 nginx.conf 复制到正式镜像。

## 改动的旧断言及原因（无削弱）

1. `apps/web/test/r20-creation.test.mjs`
   - 日常世界的存在性检查和点击目标由精确文本 button 改为有相同可访问名称的 `[role=radio]`。原因：交互按要求改为真实 RadioGroup；选择前后 Next 的 true/false、未选样式、无业务写入断言全部保留。
   - 初始“新建世界链接存在”改为“新建底稿 radio 存在且总共两个 radio”。原因：它现在是等权选择卡；选中后才显示导航 CTA。新增回归和浏览器测试继续验证原编辑链接、真实跳转、返回路径和草稿保留。
   - 三处禁止显示创建选项的文案检查保留原 `/使用日常生活世界|创建世界/` 分支，并增加 `创建新的世界底稿` 分支。原因：覆盖本次新文案，且不移除旧禁止项。
   - 精确发布版未读到、读到后、已有世界缺失这三处 `.disabled` 判断改查卡片内真实 radio。原因：外层现在是 label，禁用责任在 radio；期望值 true/false 原样保留。
   - 测试名称由“只有未选择的日常卡”改为“未选择的日常与新底稿 radio 卡”，与新增等权选项一致。
2. `apps/web/test/demo-replica-creation-ui.test.mjs`
   - 发布版本重试后的 `.disabled === false` 改查卡片内真实 radio。原因同上；卡片数量、发布版本重试、游标和去重断言不变。
3. `apps/web/test/demo-replica-studio.test.mjs`
   - 无筛选空目录的期望标题由 `没有找到相关资源` 改为 `还没有创作资源`。原因：本次明确要求无筛选使用首次空态文案。
   - 追加断言禁止出现筛选文案，并追加读取失败时 `.studio-empty === null`，加强区分无数据与读取失败；原无业务写入和重试 GET 断言保留。

## 知识与交付

在 `.trellis/spec/frontend/quality-guidelines.md` 记录 HTML/哈希资源缓存、404 不可长期缓存、全入口边界和一次性 sessionStorage 恢复契约。任务与 journal 仅保存文字；未调用会自动提交的 Trellis archive/add_session 脚本。
