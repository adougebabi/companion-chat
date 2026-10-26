# R11 修复与组件替换验收记录

基线：d017b434（React/shadcn 迁移及 R10 合并）。本轮不创建 Trellis 任务。
隔离工作区：`/Users/vinson/.codex/worktrees/v62-retest-fixes/local-ai-companion`。
输入：摇光-UIUX走查-r11.md、r11-diag.tgz、摇光-组件替换清单.md。
附件为待核对的问题材料；字段、权限、版本与写入语义以现有 API 为准。
原 checkout 的其他 session 研究文件、Agent/Writer/Kev 提示词未修改。

## NAS 媒体工具故障

已单独提交并推送 f3dfba3b。只读核对用户指定 NAS 回合后确认：
`tool.prepare.failed` → `capability_prepare_failed` → `media_subject_not_authorized`。
故障发生于工具准备，未到图片生成。

`actor_self` / `self` 仅绑定当前调用的真实 task actor，其他人物仍须原授权。
明确的不授权返回可消费、不可重试的工具错误；真实依赖故障保留 retryable。
回归包括本人别名、冲突、外部人物、拒绝分类和用户相同结构的可空自拍参数。
没有在 NAS 部署、重试或主动调用模型；需用户拉取并重建后复测真实生成。
诊断原文不入仓库。

## 已修复的核心体验

| 问题 | 实际改动 |
| --- | --- |
| 80 字后输入失焦、异步输入被还原 | 固定字段元素类型；React 同步维护文本 draft，组字时不覆写；测试焦点、组合事件与跨 80 字连续输入 |
| 已有世界加载空态/迟到响应 | 角色目录变化后重读；独立已有世界、发布底稿、模板错误和 epoch；RadioGroup 选择来源 |
| 预览字段平铺和重复 | 基本、性格表达、生活、更多设定分组；精确去重字段别名/选项；隐藏技术空壳；保留 false/0 和实际字段 |
| 预览输入控件 | 性别 Select+自定义、时区/自由风格 Combobox、0–1 Slider；空值不自动写成中值 |
| 准备流程和任务详情 | 原地真实准备状态与恢复；任务中文名/失败原因/时间/最近三阶段；同页读取真实 workflow 的允许动作和检查点 |
| 诊断列表重叠 | 桌面七列 Table；窄屏 Card；共用记录与动作，详情/恢复为独立按钮；全文和时间 Tooltip |
| 工具调用详情压平 | 输入、工具请求/原回执、响应按原顺序组成 Accordion；失败组展开；原始 arguments 不重写 |
| 媒体参数/假输出 | 参数逐项 Table；只有实际 asset_id 才提供查看素材；实际图片使用保比例的原生媒体元素 |
| 上传旧表单 | MediaUpload + 原控制器；未知结果按原 key 查询，已接受后只读恢复；不重新发送字节 |
| 设置锚点打断路由 | 改章节按钮定位；管理/衣柜 Tabs、星期/筛选 ToggleGroup；删除聊天回复设置入口 |
| 基础展示不统一 | Card/Badge/Button 使用真实 variants；Sidebar/移动 Sheet、Breadcrumb、Command、Calendar/Popover、Skeleton、Progress、Sonner |
| 布局规则污染 | 通用旧 .grid 规则缩到 now-grid；字段标签纵向排列；弹层不继承工作区整屏最小高度 |

记忆的重要度、分类和标签以实际只读接口展示，不因清单建议而加入虚构写入。
头像动态颜色保留实际样式值；普通 UI 图标采用 Lucide。日期保留 civil date，
不先转 UTC；时间输入使用 HH:mm[:ss] 格式；数字输入有加减操作。

## 对象路由与业务完整性

原生 React 所有者：AvatarPage、TaskDetailPage/WorkflowRecovery、
StudioEditorPage/StudioPreviewPage/StudioMaterialPage、GovernancePage、SelfPage。
它们直接接真实 providers，不再装载对应旧路由 renderer。

保留的能力包括：

- 头像原选择/原因/历史/全 Owner 图片目录、选择版本 CAS、原 body/key 重试；
- 创作工作草稿、完整 JSON 修正、独立保存/发布/应用、逐项差异选择、实际库存
  搭配成员、Foundation 解析任务与提议、资源复制/归档/恢复、精确版本阅读；
- 创作任务原 key/草稿、隔离预览、明确采用建议、素材保留/恢复及来源；
- 治理当前 Foundation 版本、过期提议不可接受、真实能力实现版本；
- 认识的完整有效历史快照才可恢复，当前 claim CAS 与目标历史版本分别保存；
- Workflow 原命令/服务端未知命令核对、真实 allowed_actions、run_id、检查点；
- 成功写入后读取失败只读恢复；保存/读回期间的新输入保留并要求明确核对基线。

有幂等协议的治理/认识操作采用按 Owner/角色/对象/操作隔离的 session 草稿、
冻结 body 和单一实际 provider key；刷新恢复沿用原请求。Foundation 等无幂等
协议的 CAS 操作不伪造重放，未知结果须读取并确认当前对象后再继续。

## 剩余唯一领域控制器的共享控件接入

世界/地点、人物、日记/记忆详情、规划、媒体生成/授权、诊断/数据等已有领域
控制器继续拥有各自 API、CAS、FormData、原始事件和持久键。没有复制第二套
业务实现，也没有重新挂载旧应用或旧外壳。

`DomainPrimitives` 是这一边界的控件互操作层：只接受明确标记的 field/button/
Tabs/Badge/disclosure/calendar/progress/skeleton，使用同一 React 应用树的
portal 渲染 shadcn 控件。原节点是不可见、不可访问的表单/事件载体；提交只
序列化原字段，界面控件不重复贡献 name。值、disabled/hidden、原事件、焦点和
selection 同步。控制器重绘后释放旧 portal；嵌套折叠保留实际内容节点。
这是控件迁移，不把现有领域布局和控制器宣称为全部改写为声明式 React。
不解释任意页面 HTML，不执行模板，不为每个字段创建 root；新页面应直接写
React，不能扩张此互操作边界。

所有旧域确认/抽屉/弹窗由应用 OverlayHost 的 AlertDialog/Sheet/Dialog 管理，
保留原内容节点与 handler；全局通知统一 Sonner。共享日历改为真实 Calendar，
仍保留原 cursor 覆盖范围、按日期查找、只读跟随和已载入记录标记语义。

当前目录：35 个 shadcn 基础组件文件、35 个共享业务组合文件。文件数量并不
等于每个基础组件都有独立页面；input-group/toggle 等包含官方组合依赖。

## 验证与局限

- Web typecheck、完整 node 测试、生产 build 及 fixture/React import graph 守卫。
- 行为回归覆盖精确 Select 值、输入/IME 组合事件、上传未知结果、目录晚到、
  任务原命令、头像/治理/认识/创作 CAS、原提交跨刷新、慢读回新编辑、port 的
  FormData/Radio/Checkbox/disabled、折叠/重绘/焦点/selection 与清理。
- 独立只读核验后，补齐实际 provider key 持久恢复、读回后的再次草稿比较、
  Radio 回写和旧控制器焦点识别；由主线程修复并重跑。
- CUA 使用实际 dist + 临时隔离 API fixture 检查桌面 1280px 与 iframe 390×844：
  世界编辑长输入/折叠、创作世界布局、诊断 Table/Card、工具组展开、详情关闭、
  移动导航。临时 fixture 和截图不进入生产包。
- 浏览器直接输入中文及 CompositionEvent 回归不能等同于真人系统输入法验收。
- Fixture 不证明真实 PostgreSQL/Temporal/模型/ComfyUI/NAS 部署的生成成功。
- 构建仍报告主 chunk 超过 500kB；没有调高阈值掩盖告警。

最终测试数量、提交与推送证据记录于本次 journal。复测优先：NAS 原自拍调用、
初始化预览的真实中文组字和长文本、真实任务失败的原地恢复、窄屏诊断详情。

最终结果（2026-10-26）：Web 完整 380/380、控件专项 5/5、typecheck、build 与两项
生产守卫通过；Core 媒体专项 `go test -race` 通过。独立核验额外发现的
“提交中关闭、迟到回执”也已修复，并覆盖重开后的新输入与 Owner 离开清理。
桌面编辑器确认无横向溢出；390px 诊断卡片/详情、移动导航及零角色目录复验。
界面证据：`/tmp/r11-diagnostics-desktop.png`、`/tmp/r11-diagnostics-mobile.png`。
