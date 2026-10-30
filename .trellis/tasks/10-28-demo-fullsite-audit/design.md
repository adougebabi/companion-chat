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

## 任务详情截图修复边界（2026-10-30，历史R14，已由R16更正）

TaskRecordDialog负责wide尺寸、动态语义标题与单一弹窗滚动主体；TaskDetailPage去掉嵌套阅读卡层级，三页签与Demo一致，失败原因放执行阶段附近，完整来源在任务信息。WorkflowRecovery继续单独拥有allowed_actions/CAS/原请求/accepted围栏，当前保留原恢复区域，优先落实R14的调用数据主体。task-presentation只从同task/fid的正式衣柜result投影已确认阶段，生命周期payload.stage只作执行记录，不能当业务阶段。无阶段来源时如实说明，不编造百分比或时刻。只改该共享详情及必要测试/fixture/CSS，不新增后端恢复能力，不启用真实模型；截图仍仅在/tmp。

用户R14优先：TaskDetail默认执行记录视图，其主要内容为按真实task.correlation_id查询model-runs的Request/Response/错误；复用diagnosticsClient.page的正式查询及原游标，不发无范围全局查询，不凭标题/时间猜关联。已记录请求与响应直接可读，不能用工作流JSON当作响应；未记录/未读取区分。保留已有阶段修正和宽弹窗。工作流事件与完整任务JSON移入任务信息的辅助披露；allowed_actions/原身份恢复仍由WorkflowRecovery负责。

## r20实施边界

创建源选项只在成功确认全目录为空时隐藏，加载/失败/已选择来源保留真实读取和恢复；展示日常世界不等于选择。失败准备用实际blocker说明，原有角色/世界可达性保留。PreparationSteps可通过可选回调原地打开TaskRecordDialog的Sheet呈现，复用TaskDetail与Request/Response/GET恢复。TaskModelCalls仅使用真实sequence/round_count/时间/状态作轮次汇总，不用列表索引假造轮次。

Studio离开该创作访问范围时只清对应Owner/fixedKind的显示偏好，编辑/详情返回保留访问内状态；不清任何草稿/原命令。画像只解释明确已知scalar行，阅读上可合并同键同值并保留原文出现次数与完整原文，不改存储或跨记录合并业务事实。衣柜准备通过视图slot移到页头之后。Feed max-width先按原Demo证据判断，不任意铺满。KEV保留真实数值和开关；仅补总开关与媒体子选项的生效说明。数据页无可测本地健康不造“本地存储正常”。

后端改动仅限衣柜失败原因直接路径：Provider角色查询缺行提供精确typed cause；模型前读取按实际失败阶段包装并保留原error链；普通noRows不能视作generic_llm缺失。返回已知业务code/reason，不触碰执行、恢复identity、提示词、队列或默认配置。历史原始错误不靠UI猜测真实原因。

## R16：以Demo重新还原诊断三个模块

R16取代此前R14的任务详情Request/Response主体方案。以实际运行Demo和LifePages DiagnosticsPanel/DiagnosticHealth及共享详情/控件CSS为基准，逐项记录差异，不用旧实现当基线。任务中心负责业务阶段、执行事件、任务信息和真实恢复；模型调用记录负责模型/用途/耗时、请求/响应和调用错误；服务诊断恢复分项检查行，仅展示实际读取/探测或真实测试结果。正式额外筛选/辅助运行记录/导出保留在次级入口，不挤占Demo主要结构。当前阶段只改前端呈现和相关测试，不改Core、配置、prompt或任务状态；保留创建页原地Sheet和r20业务修复，在其中复用正确任务详情。
