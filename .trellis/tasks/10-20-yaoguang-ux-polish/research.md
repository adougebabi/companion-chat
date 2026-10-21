# 初查与运行定位（2026-10-20）

## 主代理真实浏览器观察

通过 CUA 打开 http://192.168.1.20:13001/ ，登录态可直接使用。进入角色库、现有专用回归人物对话、日程、建议日程抽屉、设置、模型连接；实际取消抽屉后返回原页，实际切换交流→管理。未提交业务写入。

- 角色库：进入会话的视觉目标不突出，分组和生命周期控件与主内容竞争；出现全局读取提示，需区分暂态与持久异常。
- 对话：现场仍有破图头像、内部错误码、独立重试区域；页面标题/范围与会话头重复占位。先对照最新本地源码，不能直接认定仍需重复修。
- 日程：重复 scheduled/建议调整/建议取消/尚无执行结果；建议抽屉要求手写带偏移的时间字符串。改进输入需保持时区及原提交契约。
- 设置：八个入口复用“读取、编辑和保存真实配置”，缺乏任务导向说明。模型连接表单长，地址、凭据、用途和多组保存需要更清楚的分组与完成反馈。
- 请求 viewport=390×844 后，DOM 实际 innerWidth/clientWidth/scrollWidth 均为390；截图紧接 viewport 变更，不能以此宣称已完成窄屏画面检查。临时 viewport 已 reset。
- CUA 的原生 Mac 控制因锁屏不可用，但 IAB 真实网页正常；无需为本轮网页检查要求解锁。

## 已定位的运行边界

- apps/web/vite.config.ts:6 提供 VITE_DEV_API_PROXY_TARGET/VITE_DEV_API_PROXY_ORIGIN 两项显式代理设置。
- apps/web/scripts/serve-preview.mjs:9 只修改跨源 runtime API origin，不解决 NAS Cookie/CSRF/Trusted Origin；不得直接宣称正式dist可真实联调。
- apps/web/nginx.conf:32 已有同源 /api、/auth、/health 代理，可参考其流式请求处理。
- apps/core-go/internal/httpapi/browser/auth.go:87/122/138 与 packages/browser-client/src/index.ts:551 为Cookie、Origin、CSRF关键证据。
- 本地监听未找到当前 Web/Core 的常见端口，旧18080证据不代表现在在运行。
- .github/workflows/fluctlight-ci.yml:124 镜像有revision label，NAS实际镜像ID/digest需具备相应访问后核对，前端未发现公开SHA展示。
- 104个路由模式来源为四个modules文件加service-metadata动态注册；必须记录别名，不用旧89通过/15阻塞替代本轮实际覆盖。

## 并行变动保护

初始三项WIP之外，规划结束前 git status 又出现旧任务prd/design、kev_tools.go、components/chat.js、providers/life.ts、chat-feedback.mjs、avatar.js及nas-additions-1020变化。这些不由本轮规划生成，按并行工作保留；实施前重新核对确切代码与差异，避免重复修改或覆盖。当前本轮仅写新任务规划文件，未改产品源码。
