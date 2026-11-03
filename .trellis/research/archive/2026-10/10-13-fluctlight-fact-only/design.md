# Fact-only 接入技术设计

## 基线与变更边界

功能分支 `codex/fact-only-reply-vision` 基于 fetch 后 `origin/master=e36b3be8b2efb91e02fbed2771352f49557a60df`。本地 master 额外保留 `6e11490c381fbb61cad73dda686b35846eda2f06`，其 native tools 与 final JSON grammar 分离修复为前置依赖，FO-A 实施时 cherry-pick 到功能分支；本轮规划不应用该提交，不改 master。

产品运行时是 apps/core-go 的 Core API/Worker 与 apps/web，browser boundary 已在 Go API 内；发布集合仅 Core/Web 两镜像，Core 同时用于 migrate/cutover/worker。不恢复独立 BFF，不改 Dockerfile、镜像 registry 或 CPU 架构。代码 migration head 是 `0060_media_kev`，实际部署 DB 版本未知。

## 代码证据优先于旧描述

- 私聊现在已经在 ADK 内 `ExecuteTool` 立即提交，最终认知只消费 trace，不再次发送：`agent_result_adapter.go:14,287,543`、`adk_conversation_runtime.go:263,307`。
- `tool_contract.go:196-221` 的 reply v1 输入 text、只要求 CurrentLife、隐藏回填正文，是直接的行为缺口。
- `tool_execution.go:230` Prepare 在短事务前；`tool_execution_ledger.go:72-86` receipt 与消息同事务，是保留的可靠性边界。
- `diagnostics.go:179` 把全部非 embedding role 路由 generic_llm；现视觉角色并非独立模型。
- 图像输入调用点与媒体/visual lifecycle 详见 `research/vision-media.md`；发布证据详见 `research/release-baseline.md`。这些出处均以 apps/core-go/internal/core 为根（另注的文件除外）。
- README/旧 Capability 文档含 deferred、迭代次数、旧预算等历史表述；实现与验收按当前 caller 路径，最终同步受到本轮改变的文档条款。

## 1. 配置与路由

复用 PostgreSQL system_settings、model_roles、provider_endpoints 与既有 AEAD secrets，不新增凭据中心。新增严格解码的 `conversation.reply` 配置（mode=legacy 默认、model binding ref、sampling、context budget、timeout、max concurrency=1 默认、有限 retry、failure_policy=error 默认/显式 legacy_fallback）。新增独立 `vision.understanding`（enabled=false 默认、model binding ref）。最终字段命名采用现有 snake_case 设置格式，浏览器 DTO 同步 camelCase。

引入真实独立 reply/vision binding 身份（建议角色 `conversation_reply`、`vision_understanding`），扩充合法 role、binding 列表、保存 validator 与 UI。其余文字 Agent 继续 generic_llm，embedding 继续独立。model ref 绑定已经登记的 endpoint/model；不得由名称推断视觉能力。enabled 缺 binding 为 unconfigured，绑定完整与当前健康分开，管理员显式测试/首次真实请求验证，disabled 无 secret resolve/预热/请求。

每个用户 turn 或获准 WakeUp 开始读取一次 reply 配置 + model assignment revision/非敏感参数快照；整个 run 的 Tool schema、prompt version、sampling 固定。直接 Tool 调用由服务端建立相同合法快照，模型参数不能改变 mode 或收件人。视觉 gate 在每次任务执行前重读当前开关，既有队列任务关闭后 skip；已开始请求保留取消传播，重新开启不补跑旧 skip。

私聊范围以已授权 conversation 类型/participants 的权威数据判断，不能仅因 handleTurn 被调用就启用；HandleActorTurn 等内部/群聊共享入口继续 legacy 合同。回复或视觉失败只影响相应绑定/调用状态，不把所有 generic 后台 Agent 或全端点共享健康统一判不可用。现有 endpoint 编辑会清除相关 bindings，UI需回读并提示重新绑定，不能暗中改写其它角色。

Provider 路由身份与 queue class 分离，避免修改 providerBindingRole 后意外把 Redis global generated 桶分裂。Qwen 进入 Tool 后先释放其物理请求队列 lease，再申请 reply 生成容量；writer 并发控制复用现有队列，说明 Redis 掉线仅为进程内 fallback。timeout 沿用 queue wait + inference 总期限并分别记录阶段时延；不承诺权重自动卸载。

Web 沿用 SettingsView、KevSettings 与 control-center 保存/回填方式，增加独立回复与视觉区，显示配置完整性、模型引用、版本、近期调用与失败。视觉说明明确不控制生图/视频，测试按钮仅显式触发。设置无需整体后台健康依赖回复模型。已生效 run 不随保存中途换合同。

## 2. ReplyContext 与 Provider

新增 capability-owned ReplyContextBuilder/ReplyGenerationService，输入只含服务端认证 run/conversation/Actor/profile/source + 本轮有效 evidence/result 引用，依 `BuildContextProjectionFor` 刷新并核对 authority。读取当前已编译 Working Persona 的 revision/hash/budget，不临时让 Qwen 重写人格。

独立 reply prompt surface 复用 PromptComposer、WorkingMemoryResolver、PromptContextAssembler。人格、当前消息、Actor/关系、活动 Goal 及完成条件、真实拒绝、当前有效生活/物品状态为 required；按完整 fragment/turn 缩减历史和冗余摘要，required 超限在 Provider I/O 前明确失败并记录缩减。使用独立 assignment budget，不继承主 Agent tools、推理、最终 schema、response_plan、assistant tool 轨迹或任意 StableTaskContext。

真实 user/assistant 历史保留角色；assistant 自述、摘要、推断是数据/不确定来源，不能成为物品/地点已变证据。工具结果仅选经类型化 authority 确认的事实，保留时间/归属/不确定性。Cydonia 输入移除本 run 实际内部证据标识及数据库 ID，不用全局 e\\d+ 删除正常型号。Qwen 引用的“用户冷淡所以不应推进”等推断不作为限制；真实拒绝原话直接保留。

Writer 使用现有 Eino OpenAI-compatible adapter 的无工具调用，短职责协议由源模板/配置编译版本管理：模型独立接话与推进当前有效 Goal，无固定温柔/简短/每轮反问规则；动态状态变更须有真实工具，轻微神态采用既有全角括号。正常文本一次调用，无每轮 Qwen 校验 Agent。显式媒体意图使用最小 text/media_requests envelope；普通文本可保持纯正文，JSON/envelope 解析有确定规则，原始 JSON/占位符不发送，最多一次纠正。自然语言关键词不得自动授权媒体动作。

有限检查仅针对确定结构、当前内部标识、已知能力结果与可验证的状态冲突。修复把具体冲突与权威事实交回 Cydonia 至多一次；不因为语气/冷淡/大胆/长短否决，也不声称正则能证明任意文本无幻觉。截断/OOM/input-too-large/parse/connection/queue/timeout 分类，非可恢复错误不重复同样请求；默认真实失败，显式 fallback 用同一发布幂等入口并标 mode/provider。

## 3. 单一 reply Tool 与真实闭环

每个 run 只有一个 conversation.reply 定义。legacy v1 保留 text；fact_only 用封闭 refs/draft_ref/action_result_refs 合同，拒绝 text/direction/tone/social_strategy/persona/goal/relationship 等自由指导；WakeUp 所需 topic/purpose 只作为既有授权元数据，不能送 Writer 作社交指令。缺权限/上下文独立调用真实失败。

实现采用 capability-local prepare 服务：从请求快照选择该 run 的单一 registry/definition 与实现（不把能力名分支塞入 generic ADK）；NativeDefinition/BindNativeArguments 仅执行 schema/ref 绑定，不做 LLM I/O。Prepare 在事务外生成或读取已冻结 draft，PreparedPayload 保存实际正文与 generation provenance；ExecuteDirectTx 仅调用既有 PublicationService 并存 receipt。若现 registry 快照接口不足，增加 context-neutral per-run catalog/execution view，同时保留默认 legacy registry；其选择规则由 reply 业务 composition root 拥有。

逻辑 reply key 基于既有 inbound turn/conversation 或 WakeUp 沟通标识，不能只依赖 physical ToolCall。在生成前检查逻辑已发布结果与 receipt；同一逻辑未提交候选也持久复用，不能因新模型 CallID 重生成另一文本导致 digest conflict。存储草稿/生成 checkpoint 是内部记录，不进历史。直接 caller 与 ADK 使用同一服务与验证，不存在隐藏 Main 依赖。

生成完成后短事务再做 source/lifecycle/Actor/life revision/current-authority 校验，superseded/cancelled 禁止晚发。保留消息+receipt 同事务、单 turn assistant 唯一性、WakeUp awake/permission/topic 去重。发布成功的真实含义是 visible_output_committed；当前私聊没有外部 delivery ack，不捏造 delivered=true。Qwen result 在 fact_only 下包含真实发布正文与正确状态，不包含内部 storage IDs；最终 cognition schema/claims/Goal 流程原样继续，认知失败不重发已提交消息。

修改协议源与缓存失效路径，主 Agent 不代写正文、不删有效目标、不提供话术计划；等待/静默/睡眠/权限规则继续以原有业务条件为准。物理 request policy 区分已提交回复、真实等待动作与不应回复，不把 required 永久加到全部认知步骤，最终合同/普通最终 text 不发送。

## 4. 媒体回交与恢复

Cydonia 草稿产生明确 media_requests 后，reply 工具以 accepted + needs_action 保存 durable draft（fluctlight/conversation/Actor/original inbound/profile/config snapshot/logic key/正文/意图/authority revisions/回交次数/expiry），返回 server-issued opaque draft_ref，尚未发送。模型消费后调用现 media.image.generate，使用原有 MediaCapture/Kev/ComfyUI 提示词与权限；模型不能指定任意对象引用或用正文创造衣物/地点。

动作操作键、receipt/intent IDs 由服务端绑定到 draft。恢复调用只有 draft_ref + 本 run 可用 action_result_refs；校验所有归属、调用/资产真实状态与权限，同一草稿不能再次授权生图。回交最多两个步骤（首次请求 + 一次结果后修复/补救，配置可更严格），超过失败，不递归调用父 Main。

同步失败/不可用把真实结果交 Cydonia 一次局部调整，仍用同一 reply key；pending 只返回等待，不发“照片已发”。异步通过现有媒体终态 settlement/outbox 创建稳定的 continuation intent，复用现有 cognition/workflow dispatcher 启动带原 draft 身份的受限 Qwen 续接，不保存/重放整个原生 Runner 内存。该 continuation 属于同一逻辑回复，不增加用户聊天轮次；Tool 仍走原生 ADK。

新增 draft/continuation source 仅以可核验 draft authority 允许恢复：原输入未 superseded/cancelled，当前 continuation source pending/claimed，原逻辑 message key 保持唯一；不能把原来源已 processed 当作通用权限绕过。已有回复、已生成动作和成功组件复用，不重复外部副作用。

现媒体代码会自己发布 conversation/media_reference。新增 draft-bound completion target/回执标记，使 pending 草稿图片由统一回复恢复入口结算，同时普通 legacy media 发布保持原方式；有明确单一 publication owner，不能两边各发布。资产 bytes-ready 与消息组件提交分开，失败只重试缺组件；服务端从真实数据库状态判断完成/unknown。未视觉检查不能声称已观察成图。普通 Unicode 表情保留；不存在的 sticker 能力明确 unavailable，由 Cydonia调整，不假附件。

## 5. 视觉 gate 与初始化

一个能力解析入口返回 disabled/unconfigured/ready/unavailable + config version/安全原因；ready 表示配置/能力满足，不代表本次检验 passed。domain入口 + 执行前 + physical image-bearing request 兜底。图片仅发绑定的 vision Provider，绝不回退 generic Qwen/Cydonia。

实际覆盖：媒体 quality（含候选和设定卡）、VisualIdentityAgent commit_review 图片阶段、旧可独立调用 VisionTask、direct commit_review、后台 queued vision tasks、通用 image_url 转换边界。当前用户附图只有 refs、无实际图片理解调用，继续保留接收/存储并明确 unavailable/未理解，不为本轮新增上传与猜图流水线；头像实际为首字母，无虚构检查点。

普通生成 disabled/unconfigured 进入已有 quality=skipped 保存/发布路径；已配置但 timeout/无视觉/error 是 unavailable/failed，再按可选步骤已有规则降级，不伪装 disabled/pass。视频生成不修改；现代码无抽帧视觉任务，通用 gate防止未来旁路。

VisualIdentity生成和 finalize 都曾使用视觉角色的文字 Agent；将文字阶段保持现文字路由或 deterministic现有服务，只有真正输入图片的 review 阶段用 vision binding。可选视觉缺失时保存实际 candidate bytes-ready + inspection=unverified/skipped，进入现 awaiting_review 终止自动 checkpoint，不5秒永久循环、不拒绝文本初始化、不让 WakeUp反复申请安装视觉能力。旧有效 canonical/character sheet 保留；新图不会从 skip自动晋升。不重用 media_assets.status 当 verified（会破坏ready-only读）；检验 provenance 与资产可读状态分开保存。

direct commit_review 必须携带服务端核验的视觉检验来源才能称为视觉通过；既有显式 Owner 人工确认若可用标 manual provenance，不能写虚构“模型已查看”时间线。不改变 revision/CAS/accepted promotion/三次再生成的真正业务 fence。重新启用只影响未来/显式重评，旧跳过任务不集中补跑。

## 6. 诊断、兼容与发布

使用现物理 Eino ModelRun 与 Owner-only diagnostics，增加 role/mode/config/prompt/parent-run/draft/stage correlation、实际 Writer messages、原始响应、解析草稿、发布正文与 receipts、media handoff、vision skips、耗时/token/failure provenance。普通生命周期日志仅长度/hash/模型/状态，原请求仅授权诊断存储，密钥/Authorization/full assignment 永不进入。UI按时间倒序，把实际发送与内部认知/草稿分开。

默认缺设置走legacy，独立视觉默认disabled；新增字段/表/状态仅additive，不删除旧enum含义。新 draft continuation 使用单独 versioned intent / task queue 或明确旧 dispatcher不claim的协议 fence，回滚先暂停新模式、等待/取消/隔离待恢复draft；Temporal按现version机制保留历史。具体必须用旧源码/镜像消费实验验证，不能靠“多列旧版会忽略”保证回退。

CI 沿用 Docker Hub与actions版本，功能分支push首次构建；unique tag=`fact-only-<sha>-<runId>-<attempt>`，两镜像全成功写SHA/tag/digest release manifest后才晋升fact-only-latest，串行晋升时再核对远端分支head。两仓tag不是原子更新，部分晋升失败明示，不输出完整就绪。PR read-only不发布，不用pull_request_target。

master稳定latest保留，现master手动dispatch不发布镜像；功能分支中的workflow可修复未来master dispatch行为，但未合入master就不能宣称已提供稳定新入口。即时回滚以预先记录的stable digest；现可用master push rerun须核对run source SHA/产物，部署仍要切CORE_GO_IMAGE和WEB_IMAGE。若需让master获得最小CI引导改动，另列并等授权，不能合入业务。

部署预检明确两个实际镜像ref，pull→up --no-build并重建受影响服务→核对digest/health/私聊。独立测试Temporal namespace与数据，worker启动会切deployment current版本，同namespace不能只靠build ID隔离。数据库/配置私有备份、stable digest和生产namespace现均未知。所有远端CI、模型长聊、Compose/回滚未执行项如实列出。

## 工程取舍与可验证风险

- schema选择/prepare归能力业务，保持generic ADK与事务层不承载Cydonia话术。
- 草稿是本轮必要的新增持久状态，用于媒体与重试，不重做Goal/Memory/发送系统。
- 文本一次普通调用是默认路径；最小媒体envelope对自然表达的影响需真实模型验收，不能以mock证明八九成体验。
- 在小范围 deterministic fence 外，无法机械证明任意文字不虚构事实；仅测试与标明能力限制，不能放宽状态authority。
- 首次实施顺序A→B→C→D→E→F；E可独立验证但依赖B路由。六子任务都完成并通过联合检查才宣告本轮实现交付。


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
