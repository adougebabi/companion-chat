# 技术设计

## 变化边界
本次以当前React源码为基线，不重做应用。先建177个子视图/操作族的覆盖记录，并继续根据实际浏览补足。共享样式/组件由主线程统一修改，子代理只定位和独立核验。Demo源码、离线文件和参考图只读，hash验证防止参考漂移。

## 三条证据链
1. 视觉：提取Demo真实固定数据/程序SVG资源，映射为仅test目录下正式HTTP DTO；正式组件通过原workspace/provider/BrowserClient读取。相同浏览器1348x925/390x844、DPR/缩放/字体、文案数量/图像/选中状态/滚动位置，记录DOM有效样式、截图与差异。
2. 交互：在真实正式构建上逐项执行原输入、打开、编辑、取消、确认、异步与恢复，证据具体绑定矩阵ID和状态。已有测试可复用，但本轮实际运行和断言范围分别记录。
3. 业务：当前源码构建独立Compose project，独立Postgres/Redis/MinIO/Temporal和新Owner，仅本任务数据。实际HTTP→Core→DB/任务→HTTP读回与刷新/跨页验证。可用本地模型/Comfy仅通过正式配置接入；缺真实素材、工作流、凭据等逐项明确受阻，不使用UI fixture代替。

## 当前发现
当前真实整栈全部停止，旧镜像过时且原私有部署env丢失。新建隔离环境可避免重启旧数据；本地模型11234与Comfy8188可读。177行源码映射仅待检查，不等于通过。

## 契约
保持Owner/fid/world/epoch、CAS、版本与原幂等body/key、已接受回执GET-only、nested扩展保留。显示头像/规范图/设定图、方案/实际穿着、资源草稿/发布/应用、日程建议/实际安排、已读/业务处理独立。Demo专属模拟与正式多出的能力均显式登记。后端修改只能针对定位到的直接缺陷；不得清库或更改提示词/token政策。

## 产物
coverage.json为完整数据源，coverage.md为可读索引；每行视觉/交互/业务独立状态与证据；evidence按模块/场景归档；fixes.md记录现象/根因/文件/验证；business-verification.md记录真实持久化/产物与受控测试边界；progress.md持久保存当前批次和未完成项。


## 追加反馈治理设计边界

先生成全调用点inventory（React、保留renderer、通用桥与toast），再定反馈组件显式语义与迁移表；不能全局换默认色。角色库读取使用现有creation draft/recovery/command/accepted状态，不建立第二份业务权威；恢复入口只指向真实支持的核对/阶段/配置/读取能力。共享组件负责视觉/可访问语义，页面负责业务类型、动作与生命周期。具体迁移按审计发现收敛到原拥有者，不宽泛按error字符串猜状态。新增feedback UX维度不改既有视觉/交互/业务证据状态。

## r18 修复边界

先修已定位的拥有者：宽屏内容盒模型在workspace-replica；配置草稿误报在configuration-draft和Management controller；JSON输入/配置反馈在CharacterWizard视图与creation-feedback；任务事件呈现、领域恢复入口与管理上下文在TaskDetail/DiagnosticsWorkspace/WorkflowRecovery。随后将设置通用折叠表单换回Demo分卡、滑块和开关，保留全部正式字段/数值精度/CAS与原提交。只在现有Go恢复元数据里补衣柜业务入口，不扩展通用restart/reset能力，不改变模型配置或提示词。新增真实缺配置/密集诊断/宽屏和390场景，保留既有矩阵和修复。

## r19 修复边界

基线ea3d8e33，沿用既有业务控制器和已接受命令恢复。导航问题在App/approved routing及Management/Studio目录的实际入口收敛；新建资源提示在StudioEditor显示层区分，保留dirty/save业务不变量；设置展示在SettingsSections/SettingsField/ModelDirectory及现有字段拥有者收敛；数据页保留原导入/导出/维护能力但统一标题、次导航几何和低干扰说明；共用CommandDialog保留真实同步/冲突能力，正常表单只保留必要主要操作；媒体与全局主题根据生效CSS定位黑框/宽度/保存栏覆盖；预览使用正式语义组件处理已声明枚举和关系量表，未知扩展完整保留。涉及业务状态选择由主线程亲自实施，不扩展媒体收藏等缺失API，不改模型/提示词协议。参考70图留在/tmp，提交只含文字报告和检查数据。
