# v6.2 r13 修复与回归记录

依据《摇光-UIUX走查-r13.md》§3 与 §7.1 的确认项处理；保持单一 React/shadcn 生产入口，不创建任务。诊断包及外部报告仅作为问题证据，不据其操作建议恢复 API 或修改环境。

## 修复对照

| 问题 | 实现与结果 |
| --- | --- |
| P0 手机日历裁切 | 将旧业务 table/th/td/tbody 样式限定到 .table-wrap，包括 850px 最小宽度、padding 和 hover。共享 Calendar month grid 明确 min-w-0。375 下月表 285px，容器 299px，35 个日期按钮均在边界内；1440 也验证完整日期。没有通过取消裁切掩盖问题。 |
| P0 日程规划无法挂载 | renderer 使用当前 store 模块读取 session，移除失效 ctx.store 引用；按 Owner/角色保留分区，登出清理视图缓存。375/1440 均挂载实际日程查询和内容。 |
| 生活子项热区收缩 | SidebarMenuSubButton 使用 w-full；375 下子项铺满父行。≤800 的导航高度以明确 44px 约束，根字号保持 14px。 |
| 媒体/创作加载更多 | CursorControls 增加明确 append 模式并使用 Pagination 原语，不渲染不存在的上一页或页码。两页保留原 applied 筛选、rows、cursor、Owner/epoch；读取失败保留原 options/cursor/mode，重试原读取。Studio 读取错误与归档命令错误分离。浏览器实际 2→3 项，Node 覆盖失败后修改未提交筛选再重试。 |
| 手机聊天预览遮挡 | 共用一份预览内容，≤800 使用受控 Sheet，桌面保留 aside。关闭按钮与 Escape 均关闭，焦点还给展开按钮；跳日程也关闭。375 下预览宽 375px，800/801 两侧行为验证。 |
| 指定移动操作热区 | 用显式 touch-target 类为导航开关、关闭、发送、更多菜单、步进器及业务 ActionButton 设置 44px 最小宽高；该 opt-in 类同时覆盖 Portal，不改全局所有 button 或日历日期格。 |
| 衣柜标签重叠 | 标签条采用有自然高度的 flex 容器，Badge 只包文字，改名/删除为同级操作；4 个标签窄屏换行后按钮均在各自容器内。 |
| 聊天安全区 | 窄屏 composer 保留原 padding 并增加 env(safe-area-inset-bottom)；未声称验证真实 iOS 设备或软键盘。 |
| 重复标题 | DomainToolHost 保留返回操作和可访问 section label，去除额外壳 h2；内部业务标题和子章节保留。关于你/世界物品目录实测单份业务标题。衣服 DrawerFrame 显式 title 由唯一 SheetTitle 输出，移除表单重复 h2。 |
| 目标/约定空状态 | 两个网格分别使用 EmptyState，提供真实“去聊天”导航；不提供未实现的创建写入。Node 覆盖两个数组空/非空的四种组合。 |
| 初开添加衣服就报错 | 空名显示中性帮助，保留 required、空名禁用与原保存校验；不在初开时渲染错误 Alert。 |
| 归档文案与死样式 | 衣服入口为归档、确认按钮为确认归档；删除标签仍为确认删除。原归档 body/revision/key 和关闭时机保留；仅删除已无消费者的 .wardrobe .button block。 |

世界单卡留白、未复现的旧域空白/控件叠字/成长认识无参错误按报告既有结论保留。没有为未确认的反思详情问题编造测试结果。

## 验证与证据

- Web 402/402 测试，typecheck、build 及 production fixture 排除检查通过；React 图 314 个本地模块，未恢复旧应用。主 chunk 体积警告仍保留。
- 新增 r13-regressions.test.mjs 的 7 项集成/行为回归：两个实际列表追加失败重试、真实规划 renderer/当前 context、域标题重绘、衣柜真实确认与 Drawer、目标空状态组合、手机预览关闭与草稿。
- 最终只读核验未发现本轮可确证的实质回归。未修改提示词、模型入口、Token 策略、并发或 Kev 超时。
- [26 份最终测量](./evidence/v62-r13/measurements.jsonl)及[汇总](./evidence/v62-r13/summary.json)使用实际 production dist 与隔离合成数据；375/1440 CSS 宽度 iframe 及 800/801 预览断点。中途热区缺陷、载入未完成快照和不完整关于你 fixture 已由修复后样本替代，不计入最终证据。
- [日记桌面截图](./evidence/v62-r13/diary-desktop.png)为当前 IAB 实际窗口截图，不作为原生 375/1440 截图；几何尺寸来自被测 iframe。
- 测试请求无业务写入或真实模型/媒体调用；未恢复 API 切断设置、未改数据库、未部署 NAS。隔离证据不代替真实环境验收。

## 根因与防复发

日历属于 C（旧 CSS 变更传播遗漏）与 D（几何检查缺口）：前轮只检查 date 控件由 Calendar 接管和 document 无溢出，没有检查被 Card 裁掉的表格及最后一列边界。规划页属于 B（迁移 context 契约不一致），旧测试 fixture 自带 store，掩盖生产 context 不再提供 store。热区属于 E（以 16px rem 推算尺寸、忽略 Portal 祖先边界）。

已将表格隔离、真实 renderer context、日历子元素边界、物理触控尺寸及 Portal、追加读取重试契约同步到 frontend spec，并以实际行为和生产 DOM 测量覆盖；此仓库无模板源目录，不添加无关模板或任务。
