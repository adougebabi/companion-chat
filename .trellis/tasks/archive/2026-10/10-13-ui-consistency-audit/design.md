# 全站UI与交互统一化设计

## 目标与边界

在Vue 3/Pinia现有客户端内完成巡检第一、第二期全部统一化。保留亮色、安静、可信的产品气质，沿用现有导航、基础UI组件和BrowserClient。改造不改变自治、目标、身份、关系或记忆的服务端业务语义，不迁移数据。单一任务分六个顺序批次，因为共享组件、表单语义和各页面整合存在强依赖；每批有独立回归点，最终以全站验收闭环。

原始问题与file:line证据见report.md F01–F17；补充契约见research/implementation-evidence.md。

## 1. 共享视觉与交互层（R01、R07）

- 明确产品accent与UI语义primary/foreground/ring的映射，避免同名token覆盖。默认Button直接满足产品蓝色主按钮、可读前景、至少40px触控区；次级、危险、ghost各有明确状态。底层控件依靠语义样式，不再以页面补丁修颜色。
- 将按钮、选择器、复选项、标签、提示和状态徽标对齐现有glass表面、间距、圆角、字体层级。所有布局/响应式规则归styles目录。
- Lucide统一X/Chevron/排序/状态图标；普通菜单和按钮不用字符图标。
- 导航与展开动效使用短促opacity/transform及必要的height展开机制；加载和错误反馈不使用装饰弹跳；prefers-reduced-motion关闭非必要运动。保留聊天唯一滚动面与稳定输入栏。

## 2. 表单与状态契约（R02）

新增产品层FormField、FeedbackNotice、AsyncState与ConfirmDialog，复用现有Input/Select/Dialog底座；只抽取重复且有明确语义的模块。

- FormField统一label、hint、required、error、aria-invalid与描述关联。采用明确客户端校验取代浏览器气泡，提交失败聚焦首个错误，校验空值/trim空白/范围/IANA时区/日期/JSON。
- 状态区分idle/loading/ready-empty/ready/error/saving；配置ready前锁定保存，失败保留草稿；按钮反馈说明动作进度，不把传输失败等同于模型失败。
- 错误属于具体资源/动作与request epoch。创建、设置、治理、诊断不共用无范围错误横幅；中文错误+恢复动作+高级错误码/关联ID。取消/关闭有明确清理规则，后台轮询不清草稿。
- ConfirmDialog显式确认后才调用已有命令，取消无请求；保留原clearDiagnostics语义。在线不可做清空验证，fixture验证确认与取消路径。

## 3. 背景资料单一主入口（R05）

治理只保留一个“背景资料”主编辑区。按对象选择展示完整10字段，将基本资料、所在地与交流关系渐进分组，meeting_confirmed为未确认/是/否三态。修订方式明确为纠正/情况变化，说明必填，来源/版本/历史放可展开详情。

同一呈现组件使用不同adapter，保持原来的并发契约：Owner使用current-facts revision适配器，其他可编辑Actor使用context-version适配器。Owner身份来自已有session.actorId，并在认证生命周期存储/清理；不再由列表位置推断。

草稿由对象身份+instance绑定，响应先检查epoch再赋值。409保留草稿，提供读取新版本保留草稿，以及明确放弃草稿重载两种恢复；payload改变必须改变幂等身份，保存中的冻结请求不能与新草稿混用。历史facts/relationship数据保留。

## 4. 设置表单化与工作流导入（R06）

- 自治状态、允许行为、预算与已支持的普通数值用标准表单；复杂quiet_hours等保留高级编辑。开放对象从已加载配置merge已编辑字段，保留自定义action与未知字段。封闭queue/wakeup schema严格按服务器字段序列化。
- 并发数两个整数1–8；定期唤醒仅开关，周期显示固定10分钟。诊断保留规则显示当前有效30天/10000条，不提供伪生效的修改表单；已有存储配置在高级详情保留可读性。
- ComfyUI显示连接地址、已保存工作流、文件/文本导入、结构校验结果、节点摘要和高级JSON。保留裸占位符兼容。校验只代表JSON结构，不代替实际运行验证。
- media保存基于原开放对象更新当前字段，保留media_workflows、visual_identity_workflows、stage分支和未展示配置。不新增模板目录API，不提供未经验证的“可运行模板”。

## 5. 名称、详情与诊断（R03、R04）

- 抽取名称解析、中文状态/错误词典、显示时间helper；Owner评论显示你、实例显示名称，未知对象使用短标识与完整ID详情。聊天保持sender frozen时区，诊断保持viewer时区，日程输入保持对应实例zone。
- 人格详情先显示有值的常用资料，未知项/原始扩展字段收起；衣柜/外貌结构化，保留高级系统层、初始化来源、视觉身份时间轴。只读详情不出现治理变更控件。
- 各诊断共享摘要行、统一过滤应用/清除、选定时区日期输入、活动过滤摘要与高级条件折叠；既有server paging/cursors/filter epochs不变。
- JsonViewer默认折叠、可复制、单一内部滚动边界，安全text渲染；完整Owner非图片原始数据不丢失。系统事件与Prompt/Tool详情使用相同查看器。
- Workflow按workflow+run构造唯一行identity，展示名称、Run短标识、状态与时间。旧条目字段未知时明确标未知，不伪造。读取/控制仍走原BrowserClient。

## 兼容与验证

配置、Actor、Goal、对话、诊断的payload及授权边界保持；generated client不手改。CSS全局调整影响面广，每批跑对应现有行为测试，并在整合批完成完整web test/typecheck/build。

新增测试聚焦：表单真校验、错误局部归属、加载不显示临时默认、异步切对象、409草稿与幂等、设置无损往返、日期DST、Workflow唯一行、确认取消路径。单纯可逆颜色调整不写镜像实现测试。

构建产物使用任务自有同源fixture服务进行提交/错误/删除确认验证，通过CUA检查320/390/768/1440、200%缩放、键盘、reduced-motion、详情滚动与聊天草稿锚点。线上实例用于只读回归；fixture与真实Provider验收分开记录。生产部署在代码、构建和截图可审阅后独立处理。

## 回滚与风险

按批保留清晰diff/提交边界；不进行数据库迁移。先修共享根因后迁移消费组件，移除旧入口时保留底层API以便回归定位。全局CSS、Actor并发适配和开放配置保存为重点风险；任何保存字段不明确时先检查契约，不能猜默认值。
