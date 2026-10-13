# 巡检覆盖与证据索引

巡检目标：[Fluctlight](http://192.168.1.20:13001)，2026-10-13，Asia/Shanghai。

## 导航与交互覆盖

“访问”不等于所有操作均执行。具有业务副作用的操作停在编辑/确认前；空目标提交由 HTML required 拦截。设置密码不输入、不提交。首次进入已有登录态，登录页未覆盖。

| 工作面 | 路由 / 入口 | 实际执行 | 边界 |
| :--- | :--- | :--- | :--- |
| 聊天目录 | `/`、`/instances` | 左侧搜索无匹配与清空；打开创建、切换 JSON 导入、输入 `{}` 解析、关闭 | 未创建有效人格；搜索结果左右不同属语义观察 |
| 聊天会话 | `/chat` | 打开现有会话、查看底部失败反馈、打开人格详情 | 未发送、重试或忽略消息 |
| 人格详情 | 会话详情弹窗 | 基础资料、此刻状态、生活脉络、关系记忆四标签切换；进入治理 | 未立即唤醒 |
| 编辑与治理 | `/instances` 内治理工作面 | 刷新规划；创建目标；空值保存；关闭草稿；读取已有目标；打开编辑定义与期限并关闭 | 未保存目标、排序、依赖或背景；未执行暂停/复核/删除 |
| 治理折叠区 | 同上 | 展开日程与事件、关系与记忆、能力需求池、自治与修订、衣柜与物品管理 | 未创建事件、修正/遗忘记忆、回滚或修改衣柜 |
| 动态 | `/moments` | 切换显示已隐藏；检查评论空值禁用与作者显示 | 未回应、评论或隐藏动态 |
| 设置总览 | `/settings` | 桌面及手机导航 | 全部7项已访问 |
| Kev设置 | `/settings?section=kev` | 读取配置、测试连接；桌面用坐标点击安全测试按钮，确认 `:hover=true` 背景变化；手机测量 checkbox/label | 未保存配置；连接测试产生正常测试诊断记录 |
| 模型角色绑定 | `section=model-role` | 通用LLM / Embedding切换，观察模型列表读取失败回退 | 未保存绑定 |
| 模型Endpoint | `section=endpoint` | 打开标准Endpoint下拉，Escape关闭 | 未保存/修改服务地址或访问凭据 |
| 当前角色绑定 | `section=binding` | 读取两角色摘要 | 只读栏目，无新建/校验操作 |
| ComfyUI | `section=media` | 查看工作流、聚焦JSON编辑器 | 未保存；未触发图片生成 |
| 运行策略 | `section=operations` | 查看自治/唤醒/保留/队列四JSON编辑区，聚焦 | 未保存 |
| 所有者 | `section=owner` | 查看密码长度说明、空密码禁用 | 未输入密码、未修改密码、未退出登录 |
| 诊断总览 | `/diagnostics` | 桌面及手机导航 | 全部7项已访问 |
| Kev诊断 | `section=kev-decisions` | 应用过滤；展开首条原始响应；手机查看9项过滤布局 | 未导出，未逐条分页 |
| 生命周期 | `section=lifecycle` | 清除过滤；点击查看Workflow跳转 | 未执行工作流命令 |
| Agent运行 | `section=agent-runs` | 列表读取；点击首条查看运行步骤 | 未加载全部历史页 |
| 模型运行 | `section=model-runs` | 观察加载与结果切换；展开本次Tool (37) | 未调用模型 |
| 媒体提示词 | `section=media-prompts` | 展开提示词模型运行 | 未重试媒体生成 |
| 系统事件 | `section=events` | 读取事件payload；手机观察滚动；测量页面溢出 | 无提交型操作；未清空 |
| 工作流控制 | `section=workflows` | 生命周期关联跳转；查询状态并核实完成结果和列表 | 未暂停/恢复/取消/重启/Reset |

## 测量与证据边界

- 视口：1440×1000、390×844；恢复原视口。
- 手机治理：上下移约32×24px；普通规划按钮高32px；底部导航高48px。
- 手机Kev设置：checkbox为18px；首两个label高约20.375px，其余决策点label约20px。
- Kev保存按钮：背景 `oklch(0.205 0 0)`、文字 `oklch(0.145 0 0)`。均为无彩色，按相对亮度换算对比度约1.1:1。
- 实测Hover：测试连接从 `oklch(1 0 0)` 到 `oklch(0.97 0 0)`，`:hover=true`；未给每个按钮做独立纯悬浮测试，其他共享样式根据源码核对。
- 系统事件、治理、Kev设置手机 `documentElement.scrollWidth=innerWidth=390`；未发现页面级横向溢出，JSON自身有滚动容器。
- 模型/分析/Kev服务存在连接或分析失败；本报告评价反馈语言与状态处理，不把服务可用性直接计成视觉旧版缺陷。
- 源码审查：只读定位，部署commit未核验；`window.confirm`未实际触发。
- 未做网络节流、200%缩放、独立性能计时、完整a11y合规测试、所有行/所有历史详情测试。

## 截图索引

桌面捕获有缩放软化；关键控件问题优先参考窄屏图18–22及DOM测量。截图未裁剪、未修改，也没有将捕获软化作为网站字体缺陷。

| 编号 | 场景 | 文件 |
| :--- | :--- | :--- |
| 01 | 桌面入口与双列表 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/01-entry-desktop.jpg) |
| 02 | 创建解析服务错误 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/02-create-validation.jpg) |
| 03 | 搜索作用域与关闭后错误残留 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/03-chat-search-mismatch.jpg) |
| 04 | 人格基础资料 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/04-persona-details.jpg) |
| 05 | 治理黑色按钮与原生多选 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/05-governance-legacy-controls.jpg) |
| 06 | 桌面目标原生空值气泡 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/06-goal-empty-validation.jpg) |
| 07 | 动态内部ID | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/07-moments-internal-ids.jpg) |
| 08 | Kev决策点密集复选框 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/08-kev-dense-checkboxes.jpg) |
| 09 | ComfyUI JSON编辑器 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/09-media-json-editor.jpg) |
| 10 | 运行策略JSON主表单 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/10-operations-json-form.jpg) |
| 11 | Kev过滤与原始详情 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/11-kev-diagnostics.jpg) |
| 12 | 工作流查询结果 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/12-workflow-controls.jpg) |
| 13 | 模型运行详情 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/13-model-run-detail.jpg) |
| 14 | 桌面系统事件payload | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/14-system-events-json.jpg) |
| 15 | 手机事件嵌套滚动 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/15-mobile-events.jpg) |
| 16 | 手机Kev过滤首屏 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/16-mobile-kev-filters.jpg) |
| 17 | 手机聊天错误码 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/17-mobile-chat-error.jpg) |
| 18 | 手机治理按钮与上下移 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/18-mobile-governance.jpg) |
| 19 | 手机Kev约20px复选行 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/19-mobile-kev-settings.jpg) |
| 20 | 桌面保存按钮低对比度 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/20-kev-button-contrast.jpg) |
| 21 | 手机保存按钮低对比度 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/21-mobile-kev-button-contrast.jpg) |
| 22 | 手机目标原生空值校验 | [查看](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/22-mobile-goal-native-validation.jpg) |
