# r14 组件适配与基础体验修复

基线：`650291cfca3d714fa1ec3af9df815bbc539e04f3`，分支
`codex/fluctlight-functional-recovery`。本轮按用户提供的《摇光-UIUX走查-r14》、
《摇光-shadcn组件适配与对照》及 `r14-diag.tgz` 的截图核对实施，不创建新任务。
报告中的命令和环境建议仅作参考，本轮只修改正式 React 前端、回归测试和规范。

## 对照结果

| 报告问题 | 实施结果 | 主要归属 |
| --- | --- | --- |
| 侧栏下沉、无 Rail、折叠不完整 | 全高度 inset Sidebar + Rail + icon collapse，顶栏进入 SidebarInset；图标模式的生活子页可用鼠标和键盘访问 | `App.tsx`、`WorkspaceNavigation.tsx` |
| 图标模式透明标题挡点击 | 折叠时真正隐藏分组标题，布局测量检查按钮中心的实际命中元素 | 导航及 `helpers/r14-layout.mjs` |
| 右预览为手写 aside | 桌面接入独立右 Sidebar；窄屏保留受控 Sheet。独立 Provider 禁用重复的 Ctrl/Cmd+B 监听 | `ChatPage.tsx`、`ui/sidebar.tsx` |
| 抽屉双关闭 | 只留一个有中文名称的关闭按钮；头部固定，正文独立滚动；Escape 与焦点恢复保留 | `components.tsx` |
| 聊天手写消息/气泡/滚动 | 实际接入 MessageScroller、Message、Bubble、Marker、Spinner、Kbd。每个 transcript 直接子项都有 Item；删除竞争的滚底 effect 和旧气泡皮肤 | `ChatPage.tsx`、`ui/*` |
| 附件裸图/上传队列 | 对话使用 Attachment，可打开完整图片；上传文件用 Attachment 显示文件名/大小/状态 | Chat、MediaUpload |
| 记忆列表自扩行 | Item 列表进入只读详情 Sheet；编辑单独建草稿。遗忘确认取消、失败保留及深链接仍有效 | Life pages/views |
| 记忆编辑返回后丢焦点 | 明确传递 `returnFocus`，最终关闭返回原记忆行 | Life view、DetailDrawer |
| 衣柜/管理页签灰轨 | Tabs 换行并覆盖 primitive 的 group 固定高度；长衣柜页签适配窄屏 | Wardrobe/Management views |
| 衣柜未选标签仍显示滚动框 | 未选标签只显示说明，选择后才显示槽位；移除槽位内外的重复滚动 | Wardrobe view |
| 今日穿着截断 | 值列使用 `minmax(0,1fr)`，名称自然断行，窄屏摘要单列 | react-layout.css/react.css |
| 模型目录内联 Command | 可弹出的搜索目录，并明确“同步模型目录（已保存服务）”。模型用途使用可搜索且可显式采用自定义 ID 的 Combobox，真实写入草稿 | ModelDirectory、Management view |
| 长列表选择 | 角色、衣柜标签、Studio 服饰定义接入支持 `{value,label}` 的 Combobox；精确保留业务 ID 和空值；短枚举继续 Select | Combobox、Media/Studio/Wardrobe |
| 退出登录位置 | 账户操作与管理页签同一标题行 | Management page/view |
| 向导手写步骤/准备列表 | 步骤改为受控 Tabs，不允许跳过验证；世界来源表单使用 Questionnaire；实际准备状态用 Trace + Progress | CharacterWizard、PreparationSteps |
| Switch 被旧 CSS 覆盖 | 删除伪滑块、`:checked` 和 slot 尺寸/位移补丁，使用本地官方 Radix primitive 的状态与 token | react-layout.css/react.css |
| 表单字段结构 | SettingsField 接入 Field/FieldLabel/FieldSet，保留实际控件 ID 和值映射 | SettingsField |
| 801px 打开预览时内部裁切 | 删除旧网格的 280px 最小列宽；编辑器底栏可换行；测量右栏自身边界，避免只看 document overflow | chat layout、r14-layout helper |

## 与报告不同的已核实状态

- planning 挂载问题在基线已有修复，本轮保留实际 DomainPage 挂载回归并复测
  `#/c/:fid/life/schedule/planning`，不重复改写业务控制器。
- 日记日历已有 `min-w-0` 修复。本轮在 375px 实测 table 与所有日期按钮都位于
  日历框内，不以“document 没溢出”替代子元素边界验证。
- ModelDirectory 原来被默认关闭的 Disclosure 包裹；真实问题是打开后常驻只读
  Command。本轮分别明确浏览目录与配置模型的行为。
- 角色/世界/媒体/Studio 等实体卡继续采用 Card；Attachment 只承载实际文件。
  Chart、Resizable、Carousel 等可选增强没有为凑组件数量安装。

## 验证

- `pnpm --filter @fluctlight/web typecheck`：通过。
- `pnpm --filter @fluctlight/web test`：**407 / 407 通过，0 跳过**。
- `pnpm --filter @fluctlight/web build`：通过，生产 fixture 排除检查和 React-only
  import graph 检查通过。仍有既有的大于 500kB 主包提示。
- 新增五项 r14 行为回归：单一关闭与还焦、精确搜索选择/清空/自定义值/取消、
  图标生活菜单键盘与指针事件、Trace 独立失败/未知/恢复、左右 Provider 快捷键隔离。
- 更新现有回归：记忆深链接/取消/失败及编辑往返后的焦点、模型输入在保存读回期间
  保留新草稿、媒体初始和显式角色过滤、Questionnaire 选世界后实际进入下一步。
- 最终生产构建有 **31 组**浏览器布局测量，均无断言错误：衣柜、日记、记忆、管理、
  planning、角色库、世界分别测 375/1280/1440；展开预览测 801/850/1024/1280/1440；
  另测图标导航、320/375/800 聊天和 375 预览 Sheet。
- 额外浏览器操作核验：鼠标打开图标菜单；Ctrl+B 保持右预览；记忆编辑取消再关闭
  返回原列表行；衣柜清空标签与重新选择；Switch 开关单滑块与目录弹出搜索。
  最终矩阵验收时间段的控制台错误为 0。

浏览器使用 `ui-quality-server.mjs` 的隔离合成数据与正式 `dist`，服务仅绑定 loopback，
没有上游转发或模型调用。它证明渲染、布局和交互，不证明真实 NAS、模型、PostgreSQL
或 Temporal 的业务成功；未改动实际 LLM/媒体配置。

## 证据与规范

- [布局与交互数据](evidence/v62-r14/measurements.json)
- [桌面聊天](evidence/v62-r14/chat-1440.png)
- [图标导航与右侧预览](evidence/v62-r14/chat-icon-sidebar-1440.png)
- [375px 日历](evidence/v62-r14/diary-375.png)
- [375px 记忆详情](evidence/v62-r14/memory-detail-375.png)
- [375px 预览 Sheet](evidence/v62-r14/chat-preview-375.png)
- [衣柜空标签](evidence/v62-r14/wardrobe-empty-1440.png)
- [375px 衣柜](evidence/v62-r14/wardrobe-375.png)
- [管理中心](evidence/v62-r14/management-1440.png)
- `apps/web/test/helpers/r14-layout.mjs` 提供可复用的只读测量与断言。
- `.trellis/spec/frontend/react-shadcn-contract.md` 同步组件、滚动、还焦、快捷键和
  表单合同；approved-demo-contract 删除已经被本轮取代的 Switch 固定尺寸要求。
