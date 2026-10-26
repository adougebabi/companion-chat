# 正式控件盘点（迁移前）

日期：2026-10-26。分支：codex/fluctlight-functional-recovery，基线 HEAD 23e04bc7。
三个只读探子分别检查控件、状态接缝、验证设施；主代理抽查入口、routing、primitives、workspace、provider 与样式。
本文是迁移前证据，不能用作迁移后完成清单。

## 实际链路与偏离点

- `apps/web/src/main.ts:2-7` 用 createApp(App)，只引入 approved-ui.css。
- `apps/web/src/App.vue:4-6` 默认加载 approved-ui/ApprovedApp.mjs，仅 VITE_PRODUCT_MODE=mock 加载 reference-fixture。
- `approved-ui/ApprovedApp.mjs:15-26` 导入 9 个页面控制器（6 个 mjs + 3 个 Vue SFC），`:201` 仍可能挂载 LegacySurface。
- `approved-ui/routing.mjs:36-84` 定义 17 个页面标签、hash 映射及 legacy=1；未匹配深链回退 legacy。
- `approved-ui/LegacySurface.vue:15-16` 调用 v3/proto/app 的 mountProduct；该目录包含正式 real-provider 页面，不能按目录名认定是 mock。
- `components/ui` 虽已生成 shadcn-vue 组件，但正式 approved-ui 没有引用这一层；旧 views 有引用，不代表主界面采用。
- `styles/globals.css:2-4` 才加载 Tailwind、tw-animate-css、shadcn-vue/tailwind.css；正式 main.ts 不加载它。
- `test/layout.test.mjs:64` 仍断言 shadcn-vue；规范 component-guidelines 与 approved-demo-contract 仍指向保留 Vue/native roots。

以上证据说明当前实现如何偏离目标，不足以认定历史上哪个会话或提交做错了决定。

## 库存

统计口径：仅 approved-ui/{renderers,pages,shared} 中 mjs/vue 的静态源码创建点；不按循环倍增，不含隔离 reference-fixture，不含整个 legacy 目录或根控制器全部 fallback。input 包含 checkbox，数量不能相加视作唯一控件数。

| 控件 | 迁移前库存 | 证据 / 目标 |
|---|---|---|
| Button | 138 处共享调用：renderer 106 + pages 28 + shared Vue 4；另至少 4 处业务原生按钮 | shared/primitives.mjs:3-9 的 ActionButton 只透传 h('button')；目标 React shadcn Button |
| Input | 42：renderer 29 + pages 6 + shared 7 | shared/SettingsField.vue:20-22、ProfileField.vue:23；目标 Input，按 type 保留数字/日期/文件语义 |
| Textarea | 10：renderer 3 + pages 5 + shared 2 | shared/SettingsField.vue:19、ProfileField.vue:13、聊天 composer；目标 Textarea |
| Select | 14：renderer 8 + pages 3 + shared 3 | SettingsField.vue:18、ProfileField.vue:10-13；目标 Radix Select，明确空值/数字/null 转换 |
| Checkbox | 10，已包含于 input | LifePage renderer:260、WardrobePage renderer:292、SettingsField.vue:15；多选仍是 Checkbox |
| Switch | checkbox + .switch，至少两类来源 | SettingsField.vue:20、ManagementPage renderer:290；目标 Switch |
| Dialog | 媒体、人物、衣柜及多个确认 overlay | BrowsePage renderer:63,169；WardrobePage renderer:277,306；目标 Dialog / AlertDialog |
| Drawer | 一个共享 DetailDrawer 及页面使用点 | shared/components.mjs:125-162 自管 Escape/Tab/focus；目标 Radix-backed Sheet |
| Tabs | .tabs + 状态按钮 | WardrobePage renderer:6、ManagementPage renderer:6,113；目标受控 Tabs |
| Disclosure | 25 个 details：renderer 8 + pages 11 + shared 6 | 按互斥/独立开合语义选择 Accordion/Collapsible，保留确有文档语义的原生 details 并记录 |
| Badge / Card | .pill/.current-badge；Surface 空壳 + .card | BrowsePage renderer:84、shared/primitives.mjs:12-19；按真实语义使用 Badge/Card |
| Avatar | CharacterAvatar 与页面手写 avatar 并存 | shared/CharacterAvatar.vue:19-25、ChatPage renderer:74,98；目标 Avatar + 真实姓名/图片失败 fallback |
| Icons | 20 个复制 SVG 图标导出 | shared/icons.mjs:1-4（保留 Lucide ISC 几何）；目标 lucide-react 语义命名 import |

## 剩余清单候选（实施后重新核对）

| 现有拥有者 | 类型 | 下一步 |
|---|---|---|
| shared/FilterChips | 搜索/筛选业务组合 | 内部统一 Input/Button/Badge，保留筛选业务协议 |
| shared/ActionFeedback | 原请求反馈与恢复 | 提炼 RequestFeedback，原始 receipt/身份不进入通用 ui 组件 |
| shared/CurrentOutfit、GarmentArt | 穿搭语义与展示 | 保留领域组合，不把业务插画强行当 Lucide 图标 |
| ProfileForm/ProfileField/SettingsField | 字段及 schema 编辑 | React Field/Label + 标准控件；保持只读/nullable/数字语义 |
| ModelDirectory | 模型选择与保存目录 | 可封装 searchable selector；保存与选中分离 |
| PreparationSteps/WardrobePreparation | 多阶段准备/恢复 | 独立业务步骤组件；不能从单个 task 状态推断全部成功 |
| SemanticTree | 结构化数据树 | 业务/通用树候选；没有一比一基础组件时不伪装为已替换 |
| NotificationGroup/NotificationIssues | 分组通知/待处理 | 业务组合 + Accordion/Badge/Alert |
| 聊天消息、生活摘要、日程/日历、媒体、世界树、治理比较 | 复杂业务展示 | JSX 重建保留业务，继续列出可复用结构，避免与基础控件混为一类 |
| v3/proto/components/forms.js:12-57 | legacy Button/Field/Tabs/Composer/Toggle/CheckboxGroup | 按可达 legacy 页面排后续迁移，不能声明已清零 |
| v3/proto/components/{controls,overlay,toast,states,sources,avatar}.js | legacy HTML 通用层 | 保持路由可达，逐项替换；Avatar 与正式页有重复 |

## 交付报告口径

迁移后以组件家族 + 实际路由/拥有者标记“已迁移 / 保留原生语义 / 待迁移 / 业务组合”，附残留扫描。HTML 中仍存在 button/input 并不说明手写：shadcn 本身也输出原生元素；应检查组件来源、行为所有权和正式引用。
