# 后端缺失接口清单

2026-10-19 最终非LLM交付状态：源码及隔离实际schema均为0095，统一验收入口为 http://127.0.0.1:18080/#/library 。完整Core注册集1534项均已对账：1500通过、34条件跳过、0失败（2337个pass事件）；其余16个Go测试包通过，完整迁移112/112。Web213、BrowserClient36、CoreClient3、脚本自测24全部通过；类型、构建、Go vet和正式无Mock产物检查通过。真实PG、MinIO上传/授权/字节、整卷备份与新隔离恢复、独立Temporal控制及浏览器关键路径均有实际证据。总证据为 `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/concentrated-nonllm/progress.json`；唯一动作台账仍为 `docs/v3-functional-ledger.json`。

真实Qwen/Cydonia/Vision/KEV/Comfy按用户要求尚未运行，不计通过；当前隔离模型绑定0、模型调用0，业务Worker保持停止。当前代码建设和非LLM回归已收敛，后续为用户配置真实依赖并统一验收；原用户部署和数据卷没有升级/重置。下方各日期切片为历史过程，其“未部署/39失败/仍待建设”描述由本条最新状态覆盖，不能据历史记录推定当前结果。


2026-10-19：WP18公开定义包及真实对象修订源码已接。完整导出活动保存定义与套装精确依赖/分组名称/已保存通知偏好；版本、完整SHA256、结构、敏感字段与引用校验；导入原子创建独立draft及映射/来源，包内引用重映射到导入依赖v1，不发布/不应用。运行地点/NPC引用显式待核对；分组明确采用（不移角色），通知偏好经现有设置组件采用为草稿再保存。/data、包详情、真实memory/关系/定义/self修订目录与回档说明走当前v3真实分支。只读核验后修了分组route漏接、合法64000字节定义出口、文件晚响应覆草稿及配置原提交重试自动换基线；同时修工作流去重后Memo读取暂时失败误终结原命令。业务备份仍待下一切片；实际库最后已知0066，source0093未应用，不宣称运行就绪。

2026-10-19：0092/0093诊断与任务恢复源码接线：当前v3诊断各分区/审计改为真实持久源、独立筛选游标/稳定详情、同快照派生去重；一致快照JSONL全量导出有complete终端回执；清理明确表/范围/截止/快照/理由且保留领域/Agent/审计权威。永久伤害统计来自实际Owner浏览器域拒绝，不推算内部Tool历史。工作流详情分别呈现PG任务/Runtime/命令，控制绑定具体Run+intent revision、持久原身份/审计，unknown不报成功；新Run可重复pause/resume，旧Run按实际剩余接收能力限制。通用restart/reset仅适用runtime control，其他走真实领域恢复；重启创建原命令确定的后续intent/拒重复Workflow，prepared-beforeRPC和响应丢失可原身份恢复。媒体恢复复用checkpoint/ready资产或显式重新生成，保留原尝试/源且新执行ID围栏。旧无范围清理/无版本媒体重试明确拒绝，不能静默执行。尚未部署/应用迁移/连接PG运行SQL、浏览器或模型验收；实际库最后已知0066，source0093。

2026-10-19：0086_owner_media_requests源码已补通用/物品图片请求→持久Owner fact→原NativeCognition/DirectTool适配→唯一请求目标media intent→原MediaWorkflow→真实资产与物品引用→当前v3结果观察/历史。accepted只表示任务排队；实际media/asset独立查询。物品和指定画面地点核对并冻结版本；生成不取得/穿用/移动，也不伪造聊天发布。当前穿搭入口已改为整套穿搭画面请求，场景入口已接共享真实抽屉，正式路径不再进入旧c1的Mock拍照。重置只改当前草稿，未知原提交身份保留。已修独立核验发现的Preflight接口签名、失败地点读取假重基线与晚来源响应覆盖新选择；强类型photo方法原已由v3-contract扩展提供，新增实际transport纯测试确认，无复制第二个SDK方法。L09扩general-request/item-photo/current-outfit，完整17入口保持；真实模型/媒体用例均留用户完工后逐个执行。实际库仍最后已知0066，新PG/升级/浏览器未运行，总任务未完成。

2026-10-19：0085_media_reference_grants源码已补限定Owner/精确图片版本摘要/目标Studio资源/用途的显式模型参考授权、撤销、历史与实际job使用记录。上传、旧生成图和人物画面可明确授权；不是Actor身份认证或无人物/视觉通过证明。原Studio任务冻结grant ID/revision，准入、排队、读取原图后及上传参考后提交生成前复查，真实字节核size/SHA256；撤销不会抹除已发生的外部效果。当前v3素材详情可管理授权，Studio分页读取选择实际授权；已修独立核验发现的undefined游标和上传期间撤销后继续启动生成两处缺口。L16扩上传参考/用户指定人物文件/撤销拒绝链，仍17独立入口。开发检查通过，PG/迁移/真实S3/浏览器/模型未运行；实际库仍最后已知0066，总任务继续。

2026-10-19：0084后续补显示头像闭环：复用原visual_identity/current展示记录与visual:角色修订，新增显式图片选择/跟随规范图、Owner+资产版本检查、同事务当前引用/选择历史、角色detail真实消费。旧未验证选择明确标记；选头像不改canonical/身体/穿着，不授予模型参考权。当前v3有共享图片选择组件与角色资料/视觉身份入口，离页/旧提交重试保留新草稿。L09追加avatar保存/重查/detail/历史与有效状态不变断言；真实执行仍留统一验收。2项受控组件和1项Browser纯校验通过，类型/构建/no-Mock通过；PG/S3/浏览器未运行。参考授权、通用生成、item photo及其他工作包仍继续建设。

2026-10-19：0084_owner_media已补Owner原始上传、持久提交恢复、真实媒体库/详情/来源、服务器筛选分页和库信息双CAS。使用版本化对象/SHA256/同key恢复；上传与角色/Studio生成来源严格分开。当前v3复用共享预览/上传/元数据组件，角色列表仅含本角色实际关联素材，创作中心可进入我的素材库。已修独立核验发现的metadata并发快照、Owner切换后晚404继续上传及重试覆盖新草稿三处边界，并增加回归。L09补固定PNG真实上传/原key恢复/授权字节摘要/隐藏恢复和双CAS步骤，17个入口保持。这里只完成媒体基础切片：显示头像、参考授权、删除/任务生命周期、物品照片及通用生成仍待后续。开发检查通过不代表PG/MinIO/浏览器/模型验收；本轮没有部署或live执行，实际库仍最后已知0066，总任务未完成。

2026-10-19：0083_studio_outfits源码已补独立全局搭配定义：精确服饰发布引用→真实库存候选/显式逐成员映射→原profile-scoped套装writer/修订→已有套装与整套换装消费者。保存不取得/不穿着；同槽自选替代保留局部差异，原库存取得来源不改写。Owner/Tool后续编辑保留来源并重算差异，旧应用重放不覆盖新修订。助手/图片任务冻结精确服饰语义并在规划/规范化消费；重复槽位与新引用已归档/非服饰成员会拒绝，历史来源继续可读。L08/L15/L16/L05已同步搭配应用/助手/真实图/独立换装脚本。仅静态/纯测试有通过证据，PG/迁移/浏览器/LLM均未执行；完整交付仍未完成。

2026-10-19：0082_world_items源码已补物品定义→运行世界目录→真实virtual_shopping来源消费者。严格item-definition.v1可发布/差异应用，应用只注册目录且核对角色/条目/活动世界双版本；独立配置可取得状态和地点。原wardrobe.inspect按需查目录，购物开始核验并冻结来源，实际结果仍由原模型/confirmed Event处理，单次库存INSERT保留取得事件与精确定义/目录版本两类来源。当前v3有物品定义组件、世界目录/详情/配置/待发聊天建议及库存历史快照。L05新增purchase等待实际至少15分钟，需30分钟预算，拒绝/延期/未取得不算通过。静态/纯检查通过，PG/迁移/浏览器/模型未运行；全局可复用outfit仍待下一切片，不能宣称全包完成。

2026-10-19：0081_studio_foundation源码已接：persona-fragment.v1可发布；精确已发布人格/结构化角色模板→JSON Pointer字段差异/逐项选择→原Foundation提案（明确applied=false）→既有治理接受/Working Persona编译→真实当前版本/来源引用。保留已有profile身份、有效身体/衣柜/习惯和当前人格；新增profile只初始化其缺少的习惯。文本角色模板有明确隔离foundation_prepare任务，复用Initializer产出待审阅结构化定义，不覆盖Owner初始化凭证；明确采用/保存发布后才进入应用。L08/L15已同步新增模型步骤和错误/结果断言。双向只读核验无确定P1/P2；纯边界/组件测试与静态检查有结果，PG业务/迁移/浏览器/真实LLM均未运行，实际库仍最后已知0066。总任务未完成。

2026-10-19：0080_studio_visual_style源码已补：严格visual-style.v1→精确发布→双版本差异/逐字段选择→真实Visual Brief及来源/局部覆盖→新视觉会话冻结消费；不修改有效身体/穿着/canonical或全局工作流。历史发布按自身kind消费，不受后来草稿类型影响，真实引用独立于草稿类型读取。已修高级JSON错误恢复和保存后来源展示，2项受控组件测试通过；API/Worker构建、Core/migrations/browser compile-only、1项纯校验测试、全workspace类型/正式Web构建及19项harness自测通过。PG业务/迁移/浏览器/模型/媒体未运行；实际库最后已知0066，本轮未复查或部署。总任务仍未完成。

2026-10-19：新增L01—L17独立回归入口、串行runner、原提交恢复/任务等待/逐步断言/失败诊断/脱敏报告及受控测试调用；后续操作册新增模型流程须有对应脚本，自测检查编号和选中测试存在性。仅由用户在全部功能完工后逐个--live执行，agent没有运行真实模型。脚本自身19项受控自测通过，17入口--help和--list通过。新增只读模型parentRunId契约，沿真实父子关联核对独立Qwen/Cydonia；生成/typecheck/正式build和Go源码编译通过，纯Browser DTO测试通过，PG父关联测试未运行。脚本仍不替代未完成的功能或整站验收。

2026-10-19：接续创建切片后，已补Studio定义目录生命周期源码：复制精确已保存草稿/发布版本形成独立未发布资源、分页真实发布历史与精确定义读取、带引用约束的归档删除/恢复；通用资源和NPC页共用resource-lifecycle组件。归档保留版本/任务/素材，实际实例/其他定义引用会拒绝归档。初始化事务锁原模板并复查归档状态，先前未激活预览不能绕过；父页重绘或离页后的命令成功先保存结果标识，再由当前组件查询Core实态，跨Owner不恢复旧回执。无新增迁移，源码Head仍0079，运行库最后已知0066，未部署/未执行业务或浏览器/模型验收。

2026-10-19：创建/角色模板源码闭环已补：当前v3向导支持描述、空白、原始JSON、精确已发布模板及同Owner分析预览；统一分组/时区、初始Goal/意向审阅、预览编辑/导出。模板发布与不可变版本读取、Owner目录搜索/类型/发布筛选先于分页，模板来源写回初始化证据。原激活输入/requestId持久恢复，离页成功先存角色检查点再释放提交；分组失败只恢复同一角色，分组删除后允许明确重选；角色库可真实移出分组。源码迁移Head保持0079（复用现表，无新增空迁移），实际库最后已知0066，未部署/未执行本切片业务、浏览器或LLM测试。既有人格/视觉/item/outfit资源应用、通用媒体、动态、设置/诊断/备份等仍未完成，不能宣告全量交付。

2026-10-19：0079_studio_media源码切片已接当前v3共享面板：Owner隔离图片任务→原MediaWorkflow/捕获解析/KEV/规范化/Comfy/MinIO链→真实Studio资产与授权回读→显式资源图片素材/受验证的非人物参考。无人物不读角色身份，预览不进入角色媒体库或生活。已修旧工作流迟到失败的身份隔离、ready资产终态恢复及Opaque定义传输。只有构建/生成/类型/测试源码编译证据；没有运行PG业务/浏览器/模型/渲染/迁移。最后已知独立库0066，本轮未升级/替换API或Worker。通用上传、人物/旧图参考、其余资源应用和原台账剩余WP继续实施，不能全量交付。

2026-10-19：源码Head已到0078_studio_jobs。Studio助手/隔离文本预览已接真实Owner任务、attempt、lifecycle/Temporal消费者、Qwen结构化建议与Cydonia独立预览、阶段缓存/显式重试/丢弃/文本素材关系及当前v3共享面板。原草稿可在等待时继续编辑，建议经明确采用才更新工作草稿；缺少必需信息返回needs_input，保持原定义。实际库仍0066，未迁移/部署，新业务/浏览器/LLM测试均未执行。图片预览的Studio资产范围、其他创作资源应用和剩余WP继续建设。

2026-10-19：0077世界事件源码已接当前v3：结构化定义/发布/差异应用→禁用初态的运行模板→启用与受控/自动排队→Worker短事务判条件/频率/概率/冷却→真实ambient Life Event/Native/outbox、临时效果与恢复→分页执行/直接引用读回。环境不覆盖角色位置，NPC到访仅形成在场/机会；新建、保存、发布和应用均不等于触发。实际库仍0066_life_place，新API/Worker未部署；没有新增业务/浏览器/真实LLM通过。完整Studio助手/试生成、其他未完工作包仍连续建设。

NPC切片0076源码已接当前v3：Worker实际在场→持久机会→角色判断→独立NPC回应→真实Tool事件/关系/记忆→结果与来源分页。预测、创建、应用均不自动结识；Owner人工确认与自动虚拟来源分开，NPC拒绝/pass/暂缓和失败独立。新增独立NPC回应只消费本NPC定义，不泄露为角色记忆。源码Head0076_world_encounter_runtime，实际库仍0066，新API/Worker未部署；新业务与真实LLM测试均未执行，详见同一台账及LLM操作册L13。


约定切片0075源码已接当前v3：稳定原请求关联Goal/Commitment/Intention，明确时间窗，规划待定不伪装接受，Planner显式绑定与原请求回接在同一事务；改期走角色请求，取消参与保留共用Goal和历史，期限repair只产生时钟事实。履约来自既有Commitment证据评估，允许真实期限内证据迟到入库后修正。共享约定列表/详情/表单/历史已接。新测试已写未执行；Go编译、生成及类型检查通过，不算业务验收。源码Head0075_promise_lifecycle，实际库仍0066，未迁移/重启。


> 当前阶段为完整全栈交付（2026-10-16）。旧B/C标签仅用于寻找差异，不能据此延期已确认范围。当前v3组件和交互为唯一基线，必要模型、迁移、服务、Tool/Agent与后台结果消费者均在实施范围。功能和证据统一回填 [原功能台账](v3-page-acceptance.md)。

本次新增源码包括：真实事件通知及快照已读、逐来源待处理、Owner request→实际intent/workflow关联、角色策略的继承/覆盖与执行器消费、日记日级/尝试/注记模型、日终触发、Qwen素材选择和独立Cydonia正文、日记回应Native认知及远程共同交流读面。Browser/Core OpenAPI与生成客户端已同步；这些项目尚未迁移部署或完成统一运行验收，不能仅因接口存在标完成。

仍需继续的范围以同一机器台账为准，包括完整创建/人格治理、自动NPC相遇、约定/物品完整生命周期、共同在场事件、Developing Self、完整创作/媒体/动态、设置/诊断/审计及备份恢复等。修复bug不替代这些功能建设。新增LLM用例由用户按 [统一操作册](v3-llm-acceptance.md) 集中执行。

自我认知、Foundation和能力治理新增源码已接当前v3：逐claim复核/遗忘/历史恢复及来源依赖、完整治理读面/分页/历史、提案幂等与提交CAS、能力审核CAS与严格状态迁移。新0070/0071迁移尚未部署，运行库仍0066；新测试已编写未执行。Foundation接受/恢复涉及画像编译，按LLM操作册由用户统一验证。

视觉身份编辑的正式路径已切换真实provider；0072新增参考定义/修订，既有视觉Agent消费新会话的冻结版本。生成、候选、完整阶段、Owner人工评审和后续任务已接当前v3。旧Mock同名页面仍仅在显式Mock模式使用；当前实际环境未升级，尚未执行本切片测试。

物品管理切片0073正在实施：现有CRUD加入同一Owner事务/退休校验与锁序；全库筛选分页、独立详情、集合+单物品双CAS、幂等/理由/审计、归档保留FK历史，以及库存不可用时清理当前使用。当前v3已接新读写与共享物品组件；新增PG用例未执行。角色借还/脱下/套装和通用媒体生产仍需继续，不能标整个WP完成。

库存动作与套装切片新增源码：借入（含ordinary object）/归还/脱下/整套换装复用现有OwnerLifeRequest、Native Agent和真实Tool收据，精确目标/版本/人格范围重查。套装确定性保存/编辑与原Tool共用领域writer，0074增加定义修订和独立完整穿搭变化历史。请求结果显示实际effects，no_change不算mutation，deferred不再冒充rejected。全部新业务测试未执行，物品照片/通用媒体和其他工作包仍需继续。

## 历史候选接口与差异线索（以下名称和分类不作为现行契约）

汇总阶段 1–2（外壳与聊天）、阶段 3（生活、关系、记忆、媒体）和阶段 4–5（创作、世界、动态、自治、数据工具、诊断剩余项）。只列浏览器所有者接口仍缺、因而页面必须显示「尚未接入」或「未提供」的能力。已有并被页面调用的接口不在此重复。

分类：

- **B 类补入口**：领域里已经有读模型、相邻路由或内部能力，缺的是所有者浏览器路由、响应字段，或页面还不能安全调用的那一步。
- **C 类新领域能力**：`browserOperations` 里没有这一领域的资源。新路由不能靠现有列表接口假装完成。

每项都要求所有者会话与 CSRF。写操作默认带 `Idempotency-Key`（或请求体 `idempotencyKey`）和期望版本；冲突返回 409，并带回当前 `revision`，前端保留草稿。原因和证据沿用现有治理字段：`reason`、`evidenceRefs`。浏览类 GET 不得移动角色或改写事实。

优先级：P1 挡住页面主操作；P2 补展示或次要操作；P3 本期明确不做，只保留说明页。

## B 类补入口

### B-01 此刻场景图

- 用途：此刻页英雄区显示当前场景，缺图时继续显示「场景图尚未接入」。
- 建议：扩展 `GET /api/fluctlights/{fluctlightId}/detail` 的 `current_state`。
- 响应字段：`scene_image` 或 `scene_url`（资产 id 或地址）、`captured_at`。没有键表示未接入；空字符串表示记录为空。
- 权限与版本：所有者只读。不要求幂等。不得把插画当成生活事实。
- 页面与 BP：`LifeNowView`，`life.now` / BP-06。
- 优先级：P1。

### B-02 心情与精力证据

- 用途：有心情标签时展示证据；没有 `evidence` 键时显示「证据字段未接入」，不标推测。
- 建议：同一 detail 的 `current_state.mood`、`current_state.momentum`。
- 响应字段：`label`、`evidence[]`（可为空数组）、`updated_at`。
- 权限与版本：所有者只读。
- 页面与 BP：`LifeNowView`，BP-06。
- 优先级：P1。

### B-03 逐条日程建议

- 用途：在已接受的整日日程上增删改一条，而不是每次提交完整 JSON。
- 建议：`POST /api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items`，`PATCH` 与 `DELETE` 同一资源。现有整日 `PUT /schedules` 保留。
- 请求字段：`activity`、`scene`、`location`、`startAt`、`endAt`、`evidenceRefs[]`、`expectedRevision`、`expectedLifeContextRevision`、`idempotencyKey`、`reason`。
- 响应字段（按契约 v0.1，Q3/Q4 待裁决）：`202 {request:OwnerRequest(kind:"schedule_item_suggestion"), schedule_revision, audit_id}`。日程要等 Agent `accepted` 后才变。409 的 `details.current_revision` 用于保留草稿。不在请求事务里改事实。
- 权限与版本：所有者写。幂等键与 revision 必填。
- 页面与 BP：`LifeScheduleView`，BP-06 / GOV-04。
- 优先级：P1。

### B-04 换装

- 用途：把已拥有或借用的物品标成正在穿着。穿着不改变所有权和来源。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/wear`。内部已有 `wardrobe.wear` 能力，没有所有者浏览器路由。
- 请求字段：`itemId`、`slot`、`expectedRevision`、`idempotencyKey`、`reason`。管理视图二次确认后另带 `confirm: true`。
- 响应字段：交流视图 `202 {request:OwnerRequest(kind:"wardrobe_wear"), audit_id}`，接受前不改穿着。管理视图成功为 `200`，`status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`、`audit_id`，并返回新的衣橱 `revision`。`expectedRevision` 传衣橱 GET 的 `revision`，不传 `owner_revision`。`owner_revision` 只展示所有者修改计数。409 为 `revision_conflict`，`details.resource` 为 `"wardrobe"`，`details.current_revision` 是她刚写成的衣橱版本；草稿保留，并提示她刚更新过。`worn_items[].recorded_by` 为 `owner` 时显示「所有者修改」，为 `fluctlight` 时显示她自己。
- 权限与版本：所有者写。幂等键必填。管理视图的期望版本是衣橱 `revision`。
- 页面与 BP：`LifeWardrobeView`，`items.wardrobe.view` / BP-08。
- 优先级：P1。

### B-05 开始或结束使用

- 用途：记录物品正在使用。使用、穿着、拥有、来源继续分列。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/use`，结束使用用同一路由加 `active: false`。
- 请求字段：`active`、`expectedRevision`、`idempotencyKey`、`reason`、`evidenceRefs[]`。管理视图二次确认后另带 `confirm: true`，`expectedRevision` 同样是衣橱 `revision`。
- 响应字段：交流视图 `202 {request:OwnerRequest(kind:"item_use"), audit_id}`。管理视图成功与换装相同：`status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`、`audit_id`。409 同样是 `revision_conflict` 且 `details.resource` 为 `"wardrobe"`。`used_items[].recorded_by` 为 `owner` 或 `fluctlight`，显示「所有者修改」或她自己。
- 权限与版本：所有者写。幂等键与 revision 必填。
- 页面与 BP：衣柜与 `items.item.detail#usage`，GAP-10 / BP-08。
- 优先级：P1。

### B-06 入库中状态

- 用途：衣柜筛选「入库中」。现在没有该字段，筛选固定显示尚未接入。
- 建议：衣柜物品增加 `inbound_status`。
- 响应字段：`pending | failed | ready`、`failure_reason`、`source_action_id`。缺键表示未接入，不要返回空数组冒充。
- 权限与版本：所有者只读；状态由服务端行动写入。
- 页面与 BP：`LifeWardrobeView`，BP-08。
- 优先级：P2。

### B-07 物品拍照且不改事实

- 用途：给物品留一张图，不改所有权、穿着或生活事实。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/photos`。
- 请求字段：`prompt`、`outfit_snapshot_id`、`idempotencyKey`。
- 响应字段：`asset_id`、`factual: false`。页面继续标「生成图·非事实」。
- 权限与版本：所有者写。幂等键必填。不接受 revision 改写物品事实。
- 页面与 BP：BP-08、BP-14。
- 优先级：P2。

### B-08 角色媒体库列表

- 用途：媒体页列出该角色的生成图。现在只有视觉身份时间线和诊断提示词。
- 建议：`GET /api/fluctlights/{fluctlightId}/media?cursor=`。
- 响应字段：`items[]`（`asset_id`、`kind`、`created_at`、`outfit_snapshot_id`、`item_id`、`factual: false`）、`next_cursor`。
- 权限与版本：所有者只读。创作预览资产不得出现在此列表。
- 页面与 BP：`MediaView`，BP-14 / DET-02。
- 优先级：P1。

### B-09 聊天侧栏摘要

- 用途：对话侧栏显示位置、下一项日程、穿着和目标，缺字段保持空白。
- 建议：`GET /api/fluctlights/{fluctlightId}/sidebar`，或在 detail 上固定这些键。
- 响应字段：`location`、`next_schedule`、`worn_item_ids[]`、`goal_ids[]`、`memory_ids[]`。每个键缺失表示未接入，空值表示没有记录。
- 权限与版本：所有者只读。不要用初始人格填空。
- 页面与 BP：`chat.thread.view`，BP-02、BP-06。
- 优先级：P2。

### B-10 角色头像

- 用途：顶栏和角色库显示可选头像。没有地址时继续用首字。
- 建议：实例列表与 detail 增加 `avatar_asset_id`。
- 响应字段：`avatar_asset_id`、`factual: false`。
- 权限与版本：所有者只读。生成图标「生成图·非事实」。
- 页面与 BP：外壳与角色库，BP-01。
- 优先级：P2。

### B-11 用户发布动态

- 用途：动态页已有列表、评论、反应、隐藏。缺的是用户自己发布。所有者不能以角色身份代发。
- 建议：`POST /api/moments`。作者固定为当前所有者 Actor。
- 请求字段：`text`、`media_ids[]`、`idempotencyKey`。不接受 `author_fluctlight_id` 或代发标志。
- 响应字段：`id`、`author_actor_id`、`created_at`。媒体继续标非事实。
- 权限与版本：所有者写。幂等键必填。重复键返回同一条。
- 页面与 BP：`MomentsView`，`feed.post.compose` / GAP-07 / MOM-02。
- 优先级：P1。

### B-12 附件上传

- 用途：聊天发送已能携带 `attachmentRefs`，没有上传入口。
- 建议：`POST /api/media` 返回 `asset_id`，再把 id 放进现有 turn。
- 请求字段：文件字节、`mime`、`idempotencyKey`。
- 响应字段：`asset_id`、`kind`。
- 权限与版本：所有者写。幂等键必填。上传失败不产生消息。
- 页面与 BP：`chat.media`，CHAT-04 / GAP-06。
- 优先级：P2。

### B-13 新会话分段

- 用途：`POST /api/conversations` 已存在，界面没有「新分段」。新分段不能重置长期记忆、关系或生活事实。
- 建议：沿用现有路由，响应里标明 `segment_id`，并保证 fluctlight 长期状态 revision 不变。
- 请求字段：`title`、`participantActorIds`、`idempotencyKey`。
- 响应字段：`conversation.id`。不得返回已清空的记忆或关系。
- 权限与版本：所有者写。幂等键必填。
- 页面与 BP：`chat.thread.new`，GAP-09。
- 优先级：P2。

### B-14 移出分组

- 用途：`DELETE /api/actor-groups/{groupId}/members/{actorId}` 已存在。角色库能归入分组，没有移出入口。移出只改归属，回到未分组。
- 建议：页面调用现有 DELETE。若删除分组，组成员全部回到未分组，需要 `DELETE /api/actor-groups/{groupId}`。
- 请求字段：无正文，或 `Idempotency-Key`。
- 响应字段：204，或更新后的分组成员。
- 权限与版本：所有者写。退休角色不能拖入普通分组。
- 页面与 BP：`InstancesView`，INS-01 / GAP-08。
- 优先级：P2。

### B-15 永久伤害拦截计数

- 用途：诊断首页只展示被拦截次数和说明。不提供放行。
- 建议：`GET /api/diagnostics/harm-intercepts`。不要新增放行 POST。
- 响应字段：`count`、`window`、`latest_reason`。接口不可用时前端继续显示尚未接入，不显示 0。
- 权限与版本：所有者只读。
- 页面与 BP：`DiagnosticsView` 概览，`diag.home` / BP-17。
- 优先级：P1。

### B-16 诊断任务稳定深链

- 用途：模型运行和工作流已能按 `correlation_id` 跳转。缺稳定任务 id 页 `/diag/tasks/:taskId`。
- 建议：`GET /api/diagnostics/tasks/{taskId}`。
- 响应字段：`task_id`、`type`、`initiator`、`steps[]`（`status`、`idempotency_key`、`error_code`）、`correlation_id`、`fluctlight_id`。
- 权限与版本：所有者只读。重试继续走现有媒体提示词重试或工作流命令，并带确认。
- 页面与 BP：DIA-02 / DIA-03，BP-17。
- 优先级：P2。

### B-17 修改角色视觉定义

- 用途：视觉身份时间线可读。设置里的 `visual_identity_workflow` 是全局渲染配置，不是这个角色的定义。
- 建议：`PUT /api/fluctlights/{fluctlightId}/visual-identity`。
- 请求字段：`identity_snapshot`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`current_revision`、`status`。
- 权限与版本：所有者写。revision 与幂等键必填。不得顺便改全局 ComfyUI 工作流。
- 页面与 BP：`MediaView`，BP-14、BP-05。
- 优先级：P2。

## C 类新领域能力

### C-01 通知

- 用途：通知中心列出「发生了什么」，可已读。铃铛不显示虚构未读数。
- 建议：`GET /api/notifications?cursor=`，`POST /api/notifications/{id}/read`。
- 请求字段：已读只需 `idempotencyKey`。
- 响应字段：`id`、`title`、`body`、`read_at`、`created_at`、`link`。没有接口时不要返回空列表冒充已读完。
- 权限与版本：所有者读写自己的通知。
- 页面与 BP：`UnwiredView` 通知中心，`inbox.notice.list` / BP-03。
- 优先级：P1。

### C-02 此地附近

- 用途：此刻页显示同一地点的在场 Actor。关系名单不是在场名单。`PUT /presence` 只写自己的在场，不能当附近查询。
- 建议：`GET /api/fluctlights/{fluctlightId}/nearby`。
- 响应字段：`place_id`、`actors[]`（`actor_id`、`npc`、`since`）。缺接口时页面保持「此地附近尚未接入」。
- 权限与版本：所有者只读。GET 不得移动角色。
- 页面与 BP：`LifeNowView`，BP-06、BP-12。
- 优先级：P2。

### C-03 日记与共同经历

- 用途：日记页读写条目。试生成和创作预览不能写入。
- 建议：`GET/POST /api/fluctlights/{fluctlightId}/diary`，`PATCH /api/fluctlights/{fluctlightId}/diary/{entryId}`。
- 请求字段：`body`、`occurred_at`、`event_id`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`id`、`revision`、`source`。
- 权限与版本：所有者写。revision 与幂等键必填。
- 页面与 BP：生活日记，BP-09。
- 优先级：P1。

### C-04 反思详情

- 用途：记忆页打开一条反思。Developing Self 的回滚已经存在，不能代替反思正文，也不能改写 Core Persona。
- 建议：`GET /api/fluctlights/{fluctlightId}/reflections/{reflectionId}`。
- 响应字段：`id`、`body`、`responded_at`、`claim_ids[]`。
- 权限与版本：所有者只读。若以后允许回应，再单列带 `reason` 的 POST。
- 页面与 BP：`GrowthView`，BP-13.4 / GAP-01。
- 优先级：P2。

### C-05 世界地点、场景与地图

- 用途：世界页除本实例已记录事件外，还要地点、场景、地图和 NPC 图。浏览不得调用会改位置的接口。
- 建议：`GET /api/fluctlights/{fluctlightId}/world`。
- 响应字段：`places[]`、`scenes[]`、`map`、`npc_ids[]`。事件继续读 detail.`events`。
- 权限与版本：所有者只读。不提供「前往」写路由，除非另有带 revision 的行动接口。
- 页面与 BP：`WorldView`，BP-10、BP-11 / GAP-03。
- 优先级：P1。

### C-06 实例事件模板开关

- 用途：已发布的全局事件模板，由该实例开关。世界页不能创建模板。
- 建议：`PUT /api/fluctlights/{fluctlightId}/world/templates/{templateId}`。
- 请求字段：`enabled`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`template_id`、`enabled`、`revision`。
- 权限与版本：所有者写。含永久伤害的模板不能被开关绕过，服务端拒绝。
- 页面与 BP：`world.instance.settings`，BP-11。
- 优先级：P2。

### C-07 创作资源

- 用途：创作中心的角色模板、人格片段、地点设定、物品、事件模板、视觉风格。编辑定义不自动改变运行中的实例。
- 建议：`GET/POST /api/studio/resources`，`PUT /api/studio/resources/{resourceId}`。
- 请求字段：`kind`、`body`、`expectedVersion`、`idempotencyKey`、`reason`。
- 响应字段：`id`、`version`、`referenced_by[]`。版本冲突 409，并给出双方版本。含永久伤害事件的模板直接 422，导入和复制同样拒绝。
- 权限与版本：所有者写。版本号与幂等键必填。
- 页面与 BP：`StudioView`，`studio.resource.edit` / BP-15 / GAP-04。
- 优先级：P1。

### C-08 设定助手试生成

- 用途：把现在只留在浏览器里的预览会话存成创作资源。结果不得写入角色媒体库、日记、记忆或生活事实。
- 建议：`POST /api/studio/previews`。
- 请求字段：`resource_id`、`text`、`idempotencyKey`。
- 响应字段：`preview_id`、`text`、`label: "创作预览·不进入生活"`、`life_write: false`。
- 权限与版本：所有者写。幂等键必填。服务端拒绝任何附带的生活写入。
- 页面与 BP：`StudioView` 设定助手，`studio.preview` / BP-15、BP-14 / J-07。
- 优先级：P1。

### C-09 应用到实例

- 用途：把某个资源版本应用到一个角色，先看差异和冲突。
- 建议：`POST /api/studio/resources/{resourceId}/apply`。深链 `/studio/apply?resource=:rid&char=:charId` 已能打开说明，没有提交。
- 请求字段：`fluctlightId`、`resourceVersion`、`expectedInstanceRevision`、`choices[]`（保留或覆盖）、`idempotencyKey`、`reason`。
- 响应字段：`status: "recorded_not_applied"`、`applied: false`、`resource_version`、`instance_revision`（可为 null）、`choices`、`conflicts: null`。界面只标「未完成·已记录选择，尚未写入实例」。不读取已删除的应用版本字段，也不把 `conflicts: null` 显示成冲突已解决。
- 权限与版本：所有者写，记审计。版本、实例 revision、幂等键、原因都必填。未完成响应不写实例。
- 页面与 BP：`studio.apply.drawer`，BP-15、BP-05 / GOV-10。
- 优先级：P1。

### C-10 角色级自治与触达

- 用途：全局 `product.autonomy` 已能保存。角色级免打扰、花费上限、主动触达和需确认的行动类型还没有。
- 建议：`GET/PUT /api/fluctlights/{fluctlightId}/autonomy-policy`。
- 请求字段：`level`、`quiet_hours`、`spend_cap`、`outreach_limit`、`confirm_action_types[]`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`revision`、`effective_at`。
- 权限与版本：所有者写，记审计。不得用全局预算冒充角色上限。
- 页面与 BP：`AutonomyView`，`settings.char.autonomy#outreach` / BP-16 / GOV-03。
- 优先级：P1。

### C-11 待处理聚合

- 用途：铃铛打开的待处理页。能力审核列表已有，其余来源没有聚合。本接口只返回条目，不批准。
- 建议：`GET /api/pending?fluctlightId=`。
- 响应字段：`groups[]`，每组 `kind`（`capability`、`persona_proposal`、`studio_apply`、`failed_task`）、`status: "ok" | "unwired"`、`items[]`（`id`、`title`、`href`）。`unwired` 不能写成 `items: []`。
- 权限与版本：所有者只读。批准仍走原页面：能力审核、人格提案、创作应用。
- 页面与 BP：`PendingView`，`diag.pending.list` / BP-17.7、BP-16.6 / GOV-07。
- 优先级：P1。

### C-12 配置包导入

- 用途：导入全局定义，成为创作中心新版本。不是读档，不含运行事实。JSON 创建角色走现有创建分析，不属于恢复。
- 建议：`GET /api/studio/packs/export`，`POST /api/studio/packs/validate`，`POST /api/studio/packs`。
- 请求字段：包内容、`idempotencyKey`。
- 响应字段：导出 `{pack, available}`；校验 `{valid, status:"validated"}`；导入 `{pack_id, resource_versions, status:"draft"}`。永久伤害模板返回 422。导入只生成草稿，不发布、不应用到实例。
- 权限与版本：所有者写。幂等键必填。之后必须走 C-09。
- 页面与 BP：`DataToolsView` 配置包栏，BP-18 / INS-02。
- 优先级：P2。

### C-13 业务数据备份与恢复

- 用途：灾备和换机。不提供历史时间点选择，不能当剧情试错。
- 建议：`POST /api/ops/backups`，`POST /api/ops/backups/{backupId}/restore`。
- 请求字段：恢复时 `confirm: true`、`idempotencyKey`、`reason`。
- 响应字段：未接入时 `{available:false, status:"not_implemented"}`。恢复按钮在 `available:false` 时禁用，不显示成 0 份备份。
- 权限与版本：所有者写，记审计。恢复必须二次确认。
- 页面与 BP：`DataToolsView` 备份栏，BP-18。
- 优先级：P2。

### C-14 时间线回档

- 用途：检查点恢复。本期不实现。说明页标记「未提供」，并写明对日记、记忆、关系的影响。
- 建议：将来才考虑 `POST /api/ops/timeline-checkpoints/{checkpointId}/restore`。现在不要提供该路由。
- 请求字段（将来）：`checkpointId`、`confirm`、`idempotencyKey`、`reason`，以及受影响的日记、记忆、关系 revision。
- 响应字段（将来）：`restored_at`、`audit_id`。没有路由时前端不显示 0 个检查点。
- 权限与版本：若实现，所有者写，二次确认，记审计。
- 页面与 BP：`RollbackView`，`data.rollback.view` / BP-18 / J-09。
- 优先级：P3。本期保持未提供。

### C-15 人格切换

- 用途：在角色资料里切换人格，和基础修订不是同一步。
- 建议：`POST /api/fluctlights/{fluctlightId}/persona-switch`。
- 请求字段：`targetPersonaId`、`expectedRevision`、`idempotencyKey`、`reason`、`confirm: true`。
- 响应字段：`revision`、`audit_id`、`status: "pending"`。没有可选人格列表 GET，缺列表时不编造人格。
- 权限与版本：所有者写。二次确认。不得把切换直接写成运行状态。
- 页面与 BP：`profile.persona.switch`，GOV-12。
- 优先级：P2。

### C-16 Agent 与 Tool 定义只读

- 用途：设置里查看 Agent / Tool 定义。P3 只读，页面不能保存。
- 建议：`GET /api/settings/agents`。
- 响应字段：`agents[]`、`tools[]`、`version`。无 PUT。
- 权限与版本：所有者只读。
- 页面与 BP：`settings.models#agents`，GAP-02。
- 优先级：P3。

### C-17 独立 Desire 列表

- 用途：与 Goal 分开的愿望列表。目标规划接口不能冒充。
- 建议：`GET /api/fluctlights/{fluctlightId}/desires`。
- 响应字段：`items[]`（`id`、`text`、`status`）。
- 权限与版本：先只读。写操作若增加，必须带 revision 和原因。
- 页面与 BP：`goals.desire.list`，GAP-05。
- 优先级：P3。

## X 类：契约补充（v0.1，清单外）

前端 `BrowserClient` 已按契约实现这些读接口。响应 snake_case。写请求体使用 `idempotencyKey` 与 `expectedRevision`（Q1 待裁决：CORS 尚未放行 `Idempotency-Key` 头；multipart B-12 例外，键在请求头）。409 读取 `details.current_revision` 并保留草稿。缺键显示尚未接入，`null` 是无记录，`[]` 是空。

| 编号 | 路由 | 页面用途 |
|---|---|---|
| X-01 | `GET /api/notifications/unread-count` | 顶栏红点。404 时不显示假计数 |
| X-02 | `GET /api/pending/count` | 待处理计数。`partial` 表示有未接入分组 |
| X-03 | 列表上的 `now_line`、`status`、`status_note`、`group_id` | 角色库此刻一句话。另需 `bio` 才能还原卡片副标题（契约未单列，缺键则不编造简介） |
| X-04 | 消息 `identity` / `kind` / `recall` / `memory_ref` / `goal_ref` / `action_id` / `link` | 交流/管理只影响视图，不作为服务器授权 |
| X-05 | `POST /api/memories/{id}/suggestions` → 202 `memory_suggestion` | “告诉她”。不直接改记忆 |
| X-06 | `GET /api/owner-requests/{id}` 与按角色列表 | 处理中 / 失败。状态只由 Agent 推进 |
| X-07 | `POST /api/fluctlights/{fid}/promises` → 202 | 约定提议 |
| X-08 | `POST .../moment-suggestions` → 202 | 角色发帖建议。没有代发 |
| X-09 | `GET .../world/places/{placeId}` | 地点详情。纯 GET |
| X-10 | `GET .../world/templates` | 实例模板与永久伤害锁定 |
| X-11 | `POST .../apply-preview` | 只读差异，不需要幂等键 |
| X-12 | detail.events[].impacts | 事件影响链 |
| X-13 | 记忆 / 关系 `revisions[]` | 修订历史 |
| X-14 | 动态 `author_type`、`author_name`、`author_avatar_url`、`source_action_id` | 作者渲染 |

诊断首页还要从 detail 读到当前 `actions`（契约没有单独的行动列表，待裁决）。反思时间线需要 `GET .../reflections` 列表；契约 C-04 只有按 id 读取，客户端方法已标注待裁决，404 时不编造反思。

### 契约 §6 存疑点（先按当前写法）

- Q1：JSON 写操作使用请求体 `idempotencyKey`。日程条目更新与日记更新按已合入的 OpenAPI 使用 PUT。上传同时带请求头 `Idempotency-Key` 和表单字段 `idempotencyKey`。
- Q2：新响应使用 snake_case。
- Q3 / Q4：交流视图的换装、使用、逐条日程是 202 请求，不把本地结果写成事实。管理视图换装和使用在二次确认后带 `confirm: true`，成功回执是 `status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`。衣橱的 `expectedRevision` 是衣橱 `revision`。409 的 `details.resource` 是 `wardrobe`。
- Q5：人工插画头像 `source: "owner_upload"`、`kind: "visual_identity"`，不打「生成图·非事实」。其余生成图仍打该标。
- Q6：反思回应是 `POST .../reflections/{id}/feedback`，只提交候选反馈，不改反思正文。
- Q7：配置包有导出、校验、导入。导入只生成草稿。
- Q8：没有人格列表 GET。切换前的列表缺键则显示尚未接入，不编造人格。
- 通知未读数和标记已读未接入时是 404 `not_provided`。红点不显示，不把缺计数当成 0。
- 分组删除的 `expectedRevision` 固定为 0。
- 创作应用的未完成状态只显示「未完成·已记录选择，尚未写入实例」。
- Q9–Q12：备份在 `available:false` 时禁用恢复；C-10 响应必须带 `runtime_effect: "preference_only"`。花费上限和 identity 中文对照仍按当前契约。

## 本期明确不接成可用功能

- 桌面宠物与 Live2D。
- 时间线回档的恢复按钮（C-14）。
- 永久伤害放行（B-15 只有计数）。
- 创作预览写入生活（C-08 必须拒绝）。
- 所有者代角色发动态（B-11 必须拒绝）。
- 浏览世界时移动角色（C-05 的 GET 不得写位置）。


## 2026-10-19 功能优先接续：0087—0090

按用户最新指示暂停新增/执行17项E2E，先完成剩余功能；只进行必要构建/类型/语法检查。0087角色范围参考、0088媒体删除及素材保留生命周期、0089真实动态与建议消费、0090有真实结识来源的NPC动态互动已补源码及当前v3接线。唯一功能台账仍为 `docs/v3-functional-ledger.json`，执行详情在原总任务 `implement.md`。

动态复用原moments/comments/reactions；建议沿原Native判断与独立Writer，单次真实发布后读回。NPC响应使用自己的Actor和既有独立NPC Agent，来源来自当前世界的实际结识事件，结果先持久化再提交评论/回应或跳过；不制造新关系或Owner同场。媒体删除保留精确版本与引用保护，恢复素材保留不重新生成。公开API/生成客户端已同步；新页使用共享组件及同一真实provider入口。

源码迁移Head0090，实际独立库仍最后已知0066；这些新迁移没有在本轮执行，API/Web/Worker没有部署重启。编译和源码核对不是数据库、S3、浏览器或真实模型验收，均不记为E用例通过。设置/实际计量、完整诊断与恢复/审计、资源包及业务备份/隔离恢复等仍需继续，总任务未完成。


## 2026-10-19 0091 当前v3系统设置接续

已补 `0091_configuration_commands` 源码：当前v3模型端点/四角色绑定、Reply/Vision、KEV、媒体工作流、运行/队列/摘要/意向重试、通知分类、Owner会话和只读能力目录改接真实配置服务。通用配置编辑器复用当前组件；配置默认值来自原运行消费者，不把未配置或失败当默认成功。凭据替换/明确清除、删除后版本、原提交回执查询/恢复、配置与关联绑定CAS已接。媒体处理等待上限与通知分类开关有实际消费；浏览器权限由真实浏览器状态读取，授权后仅投递新到达且未静音的真实通知。

配置值/凭据状态/端点/绑定及其版本在同一只读重复读快照返回，避免旧值配上新版本。模型发现仍在短事务外，最终提交复验角色、端点与凭据版本。凭据明文不写入浏览器恢复记录；未知提交保存原身份和盐化指纹，先查询原回执，未提交时须重填原凭据。来源及细节见原总任务 `implement.md` 和唯一 `v3-functional-ledger.json`。

源码已接线与必要构建检查不等于业务验收。本轮没有执行新迁移、数据库/S3业务、浏览器、LLM或Comfy真实测试；实际独立库最后验证仍0066。仍须继续完整诊断/任务恢复/审计、资源包/业务备份/隔离恢复及其他台账未完成项，最后统一非LLM回归并补17项与新增模型用例脚本，由用户统一执行模型验证。总任务保持in_progress。


## 2026-10-19 集中回归与实际运行接续

源码和新隔离实际schema均0095；原用户部署仍最后已知0066，未改原卷。新隔离正式入口 http://localhost:18080 ，Worker未启动、模型运行数实际0。Web213/213、BrowserClient36/36通过；新数据工具关键真实PG闭环、关于你浏览器保存/刷新/跨角色API隔离、真实MinIO字节/摘要/授权隔离、整卷备份及新隔离目标恢复/比较/0095升级已执行并读回结果。证据见 `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/concentrated-nonllm/progress.json` 和实际公开回执。

仍有39个顶层Core回归场景未通过（后续定向修复中），全站浏览器/实际Temporal控制及最终模型验收尚未完成，不声明全量交付。维护执行入口现为 `infra/backup/maintenance.py`；固定MinIO镜像不可取得时，使用 `infra/minio/build-source-images.py` 构建已校验的原固定官方源码。唯一功能台账仍为 `docs/v3-functional-ledger.json`，本节覆盖旧阶段状态，不将编译或受控Provider当真实模型通过。
