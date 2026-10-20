# 全栈交付设计边界

当前 `apps/web/src/v3/proto` 的页面/公共展示组件与 CSS 是被确认的基线，快照在 `evidence/full-stack/ui-before-t00*`。仅先落实 UI-01/UI-02，然后保存新基线。之前的 Core/BrowserClient WIP 保留，逐项复验后复用，不假定已经完成。

同一套页面经明确、类型化 provider 访问真实客户端；Mock 只在显式测试构建/入口使用。认证、错误、空/未配置、版本、任务状态、分页与范围投影集中处理。页面只拥有草稿和 UI 状态。生产不导入种子，不发失败后自动返回空/默认值，不通过隐藏徽标假接入。

后端仍是现有 Go Core 和 in-process browser boundary；复用已有领域服务、工具、事务/outbox、Temporal/Worker、媒体与 Provider 基础。补缺失模型与生产者/消费者，保证版本、幂等、权限、恢复。确定性 CRUD 不强制过模型；Qwen 认知与 Cydonia 独立表达不改变。

实施按纵向工作包推进；共用 provider/API/schema/迁移编号由主代理协调。独立领域可以并行，但明确文件所有权；不重写工作流平台。全部验证使用有标识的隔离对象/数据库，维护单一功能台账与决策/执行记录。

UI-01 移除聊天推荐话术和发送图片入口优先于历史附件入口。任务书 WP-03/E05 的附件要求若与此有语义冲突，先查最新确认记录，再集中提出具体裁定，不阻塞非附件工作。UI-02 人格切换必须使用受控 Core 命令并保留有效生活状态。

## 当前共用契约与分工
- 真实资料组件已提供 ProfileProvider(read/switchPersona/editBio)；集中 profileSnapshotFromDetail 读取 persona_compilation 与 persona_switch_history。
- Owner不同人格切换必须形成请求事实/既有评估任务/受控执行/持久终态回读；不能把Owner确认等同任意自由文本触发条件，也不能以安全拒绝替代该功能建设。API不承担后台模型Activity。
- 前端共享数据层保留现有展示组件，生产默认real、显式mock构建才导入seed；替换store为明确provider facade，真实模式禁止通用mutator假写。各页面写操作逐动作接真实命令，不能按审计文案猜dispatch。
- 当前runtime证据的API是既有旧二进制，后续必须重编；旧任务库有待处理自治/媒体任务，因此新全栈验证使用独立app DB/Temporal队列与明确测试Owner，保留已有库/卷。
- 迁移0064已有WIP且已在旧隔离库应用；后续新迁移编号由主线程分配，当前world差额切片预留0065，其余不得抢号。

## 下一波独立服务注册
`proto/services.js` 提供 configureDomainService(domain, implementation)，真实 facade 按明确方法转发，不接mutator/审计文案猜业务。world/NPC与life前端适配分别放独立provider模块并导出install/register函数；主线程在 providers/real.ts 的 registerProviders 统一注册，子代理不并发重写共享入口。世界/NPC前端代理只负责其页和 providers/world.ts；life前端代理负责此刻/日程/目标/衣柜/行动相关页和 providers/life.ts。其它领域仍逐包实现，暂时明确未实现不能算完成。

当前真实创建首先被初始化输入预算拒绝（估算13584 > 12288），不是缺API或合法空数据。修复须核实际加载模型上下文/预算和必要提示负担，不提高虚假窗口、不裁剪用户描述、不回退默认角色。

## 2026-10-16 真实聊天预算核验
首个真实v3聊天在Provider前被required wire estimate22561>16384拒绝，无Qwen/Cydonia调用。定位到Fact-only native工具阶段物理请求早已省略response_format，但前置AssemblePromptContext仍计入6094估算的完整final schema。局部修复只使Fact-only+实际Tools的装配按实际wire format预算；结果仍保留完整logical schema，tool-free/final repair和真实physical budget检查保持。共享projection refresh复用该assembler，因此同样修复。固定协议覆盖权限、物品来源可用性和真实工具结果约束后去重完全重复的factOnlyExecutionBoundary，不裁剪用户/事实/Goal、不改加载24576窗口、不改Qwen/Cydonia分工或总执行框架。

## 0075约定模型补充
Owner原请求是约定身份，链接表只存窗口/版本/真实执行关联；完成真相仍由原GoalCommitment评估持有。Commitment区分step与promise：原单活跃step规则保留，多个独立约定可挂同一Goal，约定不附着短期Stage以防阶段换代误取消参与。沿现有评估单Goal最多8个活跃承诺的输入能力做事务准入，容量满明确冲突，不先接受后卡死评价；读取历史不截为8。约定ID是可信服务上下文，不能让模型或Owner GoalSet直传升级权限。通用Goal plans不能直接改/弃约定；改期/取消经原约定入口，完成仍走证据。响应未知持久保留原请求身份与输入，确认前不创建第二条。

## 当前体验修复边界（2026-10-20）

主线程复用当前组件与服务，补明确设置工作区容器/响应式字段组；provider-editor作为单一连接上下文编排地址、凭据、目录与角色，保留原配置slot和原命令恢复。工作区mode是导航分组而非授权身份，所有管理URL由统一注册元数据同步mode，服务端鉴权不变。

启动先会话+角色目录首屏，可选域后台读取；JSON读取期限覆盖headers和body，模型触发写入保留服务端生命周期，不套短读取期限。Session SQL校验有局部deadline、best-effort last_seen短时限并节流。匿名访问和已登录链路分开记录；没有运行证据不能声称已复现NAS行锁。

世界先作为Owner定义底稿保存/发布，选择精确版本后带入人物向导。人物激活的回执先checkpoint；world apply/active select各自保留原命令与版本并读回，按fid隔离恢复。任何后续失败不重新激活人物；原输入及待补步骤保留，可继续进入人物。初始化恢复只修表示层可选结构，将未解析原文及warnings留在extensions；空模型响应、鉴权或真实引用不能假装成功。错误页直接消费既有correlation/path/code，并给明确空白创建与诊断入口。


## 资料展示修复边界（2026-10-20）
最小差距在展示组合而非业务存储：生产意向页仍用RecordDetails，Profile组件仍遍历原字段、只给对象JSON；导航写错people/manage；settings返回分属shell与内容。主线程在现有v3 cards/profile及公共语义字段组件落实可读行、中文状态和设定分组；原完整数据保留折叠技术详情，不生成未知人格或补假事实。意向同组件用于目标、此刻及约定；日记只调整父容器gap。shell声明式返回策略统一settings子页，人物管理复用既有NPC页并兼容原链接。
初始化进度复用现有visual identity/avatar/timeline/task只读API，独立局部加载及有限轮询，不能因可选进度失败阻断人物资料或重置切换草稿。canonical、character_sheet、avatar分别按真实产物/读取状态显示；候选不等于接受，跳过视觉检验不等于完成，queued不等于Worker已执行。保留当前人格runtime权威、CAS/来源/受控命令、初始设定与当前有效状态边界。预计仅Web页面/公共组件/投影/provider及测试/文档，无迁移和后端执行器改写。通过组件用例、type/build、真实浏览器受控生产组件的桌面/390px核对验证，真实NAS/LLM仍由用户验收。


## NAS报告修复最小边界
真实差距：页面父容器缺gap、诊断formatter绕过本地时区、profile通用递归丢失衣物布尔语义、runtime catalog读取Go原生大写字段却使用小写、初始化审阅直接展开目标JSON。修复位于现有Web展示组件/provider投影，不修改数据库、已存日程时间或模型/Worker。新增共享时间/领域状态展示与衣物/当前状态组件；保留原始记录供折叠核对，false/zero和unknown/cleared语义不可丢失。日程采用实际schedule.timezone优先，context/设备仅作明确显示后备，建议命令继续原带offset字符串+双CAS。能力在configuration provider集中归一化大小写兼容，不改全局Go JSON契约。自治仅分组排版，继承禁用和保存预算/版本行为不变；删不受后端支持的personal目标选项。所有修复以NAS报告/既有真实schema形状为夹具并保留实测未覆盖项；本地验证不记作NAS修复后或功能回归通过。
