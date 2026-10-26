# 组件替换清单 R2：处理与验收

基线：03431f18。继续在当前 React/shadcn、Valtio、Tailwind 4 前端直接修复，
不创建任务；不修改提示词、模型配置、后端协议或另一 session 的文件。
清单来自静态检查，逐项区分实际缺口与源码遗留 token。

## 实际修复

| 范围 | 结果 |
| --- | --- |
| ActionButton | 调用端明确指定 variant，去除旧 button/text-button/primary 映射及 product-button 覆盖；保留必要 Tooltip 组合 |
| Surface / Card | React 调用端清除 card token，Card 提供标准间距和主题；媒体、资源目录使用 Header/Title/Content；删除旧 React card/pill 基础皮肤 |
| Badge / 链接 / 图标 | 清除 pill 调用 token；链接保留原 href 和 asChild，独立链接用 Button；静态箭头/省略字符改显式 Lucide，不再运行 iconText 字符解释 |
| 错误与反馈 | inline-error 改 Alert；ActionFeedback 保留原提交折叠、只读读回和拒绝后核对动作；失败/处理中持久状态保留可访问反馈，实际成功沿既有 Sonner |
| 读取 / 空态 | 简单读取占位统一 LoadingState/Skeleton；空态使用官方 Empty；动态错误占位为空时不显示空边框 |
| 进度 | 目标和旧 Browse 音频进度换 Progress；没有伪造音频 seek 接口 |
| 头像 | 可达的评论/人物及遗留 Browse 分支改 CharacterAvatar；旧域 Avatar 接入头像端口，保留真实图片失败回退 |
| 确认框 | 衣柜、遗忘记忆、删除世界改 AlertDialog，取消、Escape、关闭均不写入；只在明确确认 handler 执行原动作 |
| 搜索 / 时间 | FilterChips 外壳改 InputGroup，保留同步 draft；时间输入同样使用 InputGroup 和时钟图标，维持原格式与焦点恢复 |
| 媒体 | 上传开关改 Collapsible；预览用 AspectRatio + object-contain，图像比例不变；真实 audio controls 保留，卡片动作从整卡 ghost Button 分离 |
| Cursor | CursorControls 和旧 CursorPager 使用 Pagination 容器，只提供真实可用的前后 cursor 动作；不合成总页数或随机页码 |
| 旧域展示壳 | 显式 card/alert/item/empty 标记使用真实 shadcn 样式和语义，原位保留同一宿主及子节点，不搬运 form 内容、不克隆 handler |
| 旧域切换 | 生活时间线筛选改 ToggleGroup，通过原按钮回写包括空字符串在内的域值；日程和 NPC 记录原本已用 Tabs，经核对保留 |
| 局部布局 | 世界编辑的表单/目录卡片补齐；静态内联布局转 utility；修复 SidebarInset flex 下 page-content 的垂直 auto margin |

## 对静态清单的核对

- 上轮 ActionButton、Surface、Badge、绝大多数 asChild 链接已经使用 shadcn。
  本轮清理的是旧 token/映射和展示结构，不能把它们描述为首次接入。
- 日程、NPC 记录使用共享 Tabs，不能因仍见 classList.toggle('on') 就替换其
  原有 panel dispose/read/query 业务逻辑。
- `.check-field` 中的控件本来是 Checkbox/Switch；不将普通勾选强改成设置开关。
- 追加式“加载更多”保留真实 append 行为；它不具备反向 cursor，不伪造上一页。
- 原生 dl 保留其键值语义；动态头像颜色保留现有实际值。
- HoverCard、ContextMenu、Field、Spinner、Vaul 是可选增强，本轮不添加无需求入口。

当前为 39 个基础组件文件。新增官方 Empty、Item、Pagination、AspectRatio，
registry 的错误 cn 导入已改回项目 cn，没有保留无关 cn 包。CLI 没有覆盖已有组件；Alert/Empty 为原位宿主增加 asChild。

## 原位展示端口契约

交互控件继续沿用 R11 的非可视原始载体。纯展示壳则使用 asChild 把生成的样式、
role 与 data-slot 应用到原宿主，保持其可见，不另建可视副本。
原 form、named controls、querySelector、contains、监听器、原节点重绘保持原位。
只接受显式标记，不解释页面 HTML，也不引入第二套业务命令。

## 验证

- typecheck、完整 Web 测试、生产构建及 fixture/React graph 守卫。
- 新回归：原位 Card/Alert/Item 的 form 祖先、精确 FormData、一次 submit、
  异步正文重绘；ToggleGroup 的空域值；真实 cursor 动作和 disabled；时间
  InputGroup 原字段值与控制器 focus；原确认测试改为断言真正 AlertDialog。
- 独立只读核验未发现阻断问题；补充的内存 fixture 验证一次可视保存只转发一次。
- CUA 实际 dist + 隔离接口：世界保存仅一次 PUT 并读回；三种媒体（4000×3000、
  600×4000、5000×400）预览框一致且 object-fit:contain；桌面及 390px 复验。
- 这些 UI fixture 不证明 NAS 已部署，也没有主动调用真实模型。
- 主 chunk 仍有体积告警，没有调高阈值隐藏。

最终验收：Web 385/385，typecheck、build、生产 fixture 排除及 React 图守卫通过。
上传与通用文件选择器的原生载体从可访问树隐藏，仅保留一个具名选择入口；
实际 file/change/拖放协议不变。临时截图：`/tmp/r2-media-desktop.png`、
`/tmp/r2-world-desktop.png`。本轮未部署 NAS。
