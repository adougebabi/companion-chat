---
name: ael-work
description: 在已初始化的 AEL 工程执行已授权的项目迭代、修复或其他工程变更，或用户明确要求操作 AEL 任务时使用。普通讨论、查询、分析、审阅和仅供审核的材料不自动进入 AEL 流程。
---

先判断用户请求的目的：是否要修改项目实际内容以进行项目迭代、修复或其他工程变更。以用户意图和交付内容判断，不把 AEL 流程自身的文件或状态当作工程变更。

- 独立的一次性格式化、空白或明显笔误修正，若范围明确且不改变行为、契约或权限，直接修正并验证差异；无需启动会话、运行 doctor 或新建任务。已在任务内的沿用原任务，用户明确要求跟踪时按要求登记。
- 普通讨论、查询、调研、分析、审阅，或仅供审核的材料，即使保存为文件，也直接处理并与用户沟通；不运行 AEL 命令或创建任务，也不进入实施、验收和知识索引流程。可按需直接读取项目文件；仅供审核的材料放在用户指定的位置本身不构成工程变更。
- 用户明确要求执行某项 AEL 命令、操作或继续某个 AEL 任务时，按明确要求处理；单独的命令或任务查询不因此自动创建新任务。
- 如果请求后续转为已授权的工程变更，在首次实际修改项目之前重新按上述条件分流；不能因只读工作中发现问题就自行扩大授权。目的不明确时先继续可做的只读工作，仅在确有必要时澄清，不默认创建任务。

进入工程工作流程前先核对已有工作的修复归属。与当前工作或刚提交交付直接相关的机械性问题，例如 gofmt，按 [FEEDBACK](references/workflow.md#10-feedback修正与独立新工作) 核对根因、范围、授权和文件冲突，满足条件就直接修正、检查、提交并核对结果，沿用原任务和执行者。原范围内的实现缺陷在开放任务中更新候选并重验；已有完成记录时追加交付修正。独立新工作才按授权建立关联任务。

进入常规工程工作流程后，Local 从项目根使用 `./ael`。Remote 可用机器级 `ael --project PATH`，已安装轻入口时可用 `./ael`。运行 doctor 和 task list，核对真实项目、数据路径、工具版本及知识健康。复用相关的既有目标与已验证步骤，只为已授权的工程工作创建任务；不逐步请求继续，不自行发明后续需求。

唯一流程来源：[通用工程协议 v1.0](references/workflow.md)。开始时读取第 1—4 节和当前责任节点；W1 界定工作、W2 分析、W3 执行、W4 验证/回退、W5 交付及知识回写按需读取相应章节。项目原文是契约来源，索引和看板是投影；历史、决策、任务与证据保持身份区别。

实际命令：[命令与恢复](references/commands.md)，机器数据结构以当前 CLI 的 `ael help --json` 为准，项目入口可使用 `./ael help --json`。需要读取字段、调用脚本或保存结果时，命令明确追加 `--json`；默认输出供人阅读。

远程 authority 使用 [外部 Agent 通信协议](references/remote-agent.md)：已有 Agent 自行编辑、测试、截图和管理宿主；AEL 只接收并校验。先登记客户端，准备候选/context，再 runner claim；领取私有回执直接传给后续命令的 --lease-file；长步骤领取时用 --owner-pid 绑定本次宿主自动续租，已有租约可并行启动 runner keepalive。常规续租、按约定上传和可自行核实的恢复直接执行。真实决策点在聊天中一次说明，按远程协议同步用户答复；普通任务直接用 runner submit --lease-file CLAIM --repository REPO --status pass --summary TEXT 登记改动、做法、验证与缺口。实际检查在各自的责任中登记。明确需要文件时才选择附件，需共享的规范正文单独显式发布。每次 submit 消费 next_action。断网按 operation_file 调用 runner retry，未知工作区使用 runner recover 明确接收方与停工/独立工作区决定。远程不使用下述本地 claim/verify/loop 命令；不需要客户端 Docker，不启动其他 Agent。run/resume-result/recover-claim 已退出，不能用于恢复新模式。普通上下文发布只登记规范路径与版本，可复用已共享的准确正文。

导入任务不要整批机械要求用户网页恢复。先逐项读取当前 task/next：已是 continue/repair 的直接按原协议 claim；若恢复批准已经绑定当前定义/计划、confirmation已清除，且无后续人工暂停/阻塞/其他责任，技术性 resume 由当前 executor 执行，再刷新 context 并 claim，不再要求用户回复“已恢复”。新 Server 的 recovery_action=resume 是这条只读指引，不代替写入校验。来源不明且仍缺真实恢复授权时只列具体缺口与准确范围；不得用 Agent 自述用户同意生成审批。CLI/冻结 Skill 与 Server 的实际 build 必须分开核对，版本号相同不代表功能相同；不要修改正在使用的冻结会话文件。

含 .ael/runner-access.json 的 clone 直接使用 ./ael doctor、task list；首次访问会注册以主机名展示的 runner。无需 init 或手动注册，session start、candidate capture 和 runner claim 自动采用当前 checkout 身份。旧绑定沿用显式 ID。新的 Remote 工作会话先运行 `./ael session start --session HOST_SESSION_ID --runner RUNNER --json`，读取返回的冻结 Skill 路径。它在领取前查询兼容性，只按安装级明确选择的策略更新可信稳定版。后续 Remote 命令都带 `--session HOST_SESSION_ID`，或由用户显式提供 AEL_SESSION_ID。同一会话固定 CLI、Skill 和协议，已有会话不会因机器级升级切换。`session show` 可查看引用，责任完成或明确交接后可 `session end` 结束工具绑定。旧调用未给会话 ID 时按 runner ID 隔离兼容工具会话，恢复与新接入优先使用宿主的稳定会话 ID。

Remote 上下文压缩、会话恢复、任务切换或进度不确定后，先执行 `./ael recovery check --session HOST_SESSION_ID --json`。先核对 Server 项目、父/叶任务、步骤、版本、已通过检查和最近阻塞，再决定唯一下一动作。Server 当前记录是权威。保留已验收成果、原任务身份与范围，只继续当前未完成项。未在线核对成功前，只做只读定位与现场保存。检查点显示无当前叶子时，先核对已有明确授权的父任务或批次范围；范围内仍有未完成后继时，按现有队列与权限继续领取，不重复请求用户选择。只有缺少明确授权范围、存在真实未决决定或权限不足时，才列出具体缺口并等待。

首次将宿主会话与执行者绑定用 `./ael recovery save --session HOST_SESSION_ID --runner RUNNER [--parent EPIC]`。每个可验证单元完成、submit 消费回执后和任务切换时再次 save。必要的短交接可用 `--file PRIVATE_JSON`，只接受 next、tried、decisions，禁止凭据、租约、聊天、思考或全量日志。失权使用既有责任恢复，丢响应先查原回执。检查点位于仓库外私有配置目录，按身份、checkout 和 session 隔离。

Codex 和 Claude Code 可分别显式安装项目级恢复 Hook，使用 `./ael recovery install --host codex` 或 `--host claude`。保留现有宿主配置，重复安装幂等。安装结果为待宿主信任，需在宿主审阅项目与 Hook，并实际恢复后核实触发。缺少受支持宿主、未信任或 Hook 失败时，使用上述显式命令。PreCompact 只保存已有索引，SessionStart 仅注入固定恢复规则与命令引用。

以下命令流程描述稳定 Local/Attached 功能；远程责任与证据语义相同，具体调用以远程协议为准。

1. 按 W1/W2 明确目标、范围、权限与每项验收覆盖；领取当前单元，再 context 取得同源正文、版本、理由。修改前先读取；新增影响范围重新加载。未索引、加载失败、缺关键资料和真实冲突按具体原因处理，不能当成无规范。
2. 按 W3 做实际工作，在开始、重要结果、阻塞、交接时记录事实、产物与下一步。单次完成的代码任务在必要预检后直接提交本任务改动，再生成候选并验证提交后的版本；提交前核对本次拟提交的 Go 文件格式，遵守项目钩子。保留无关改动，不自动推送。submit 的普通进展不改任务定义。只负责当前领取单元，不启动嵌套 loop；Managed 子执行器只返回当前结果，由父运行器推进。
3. 按 W4 运行真实命令/场景，或记录标为 model 的语义审查及实际观察，附件按约定提供。缺能力是 blocked；不可把进程退出、总结或状态提交当作验收。只验收任务可直接交付观察说明；有约定文件时用 runtime 引用。只在任务授权范围内工作。
4. 消费 next_action：continue/repair 连续执行；失败按实现/方案/环境分类回路，保留失败。父任务范围内使用 work next/claim 继续同一叶子直至验证，再领取后继；交接时结束当前 attempt，下游自行领取同一未完成叶子；也可显式释放归属。等待项说明解除条件和恢复节点。无活跃执行者/调度器不声称后台运行。
5. 按 W5 核对最终候选、覆盖与知识变化，确有规范变化时写明实际处理。知识收尾若改受检输入必须重新验证。原交付要求远端 CI 成功时，查到最终版本的必需检查实际通过后才收口。任务已完成时的交付修正按 FEEDBACK 追加记录，保留历史结果。

Attached 长步骤可在 claim 绑定本次宿主的可靠常驻 owner-pid；未知就不猜，租约有效期内 heartbeat。过期写入者先核实停止再 resume --ack-stopped；活 owner 不可抢占。重装/update 不覆盖用户资产；缺离线工具包准确报错。未经授权不激活旧流程、改 hooks、迁移业务数据或发布。

不同任务可以在同一工作区并行，AEL 不对整个工作区加互斥锁；执行者负责避免对同一文件和共享外部资源的冲突。候选校验仍覆盖整个工作区，其他任务的改动可能使验证失效，需要在稳定候选上重验；需要文件隔离时使用已绑定的独立工作区。


## 工具发行

独立当前平台 CLI、Server 和 Skill 资源按发行 manifest 分发。外部 Agent 可用 `ael release check --source TRUSTED_SOURCE_JSON` 查询已核实来源，用 `ael release download --source TRUSTED_SOURCE_JSON --out NEW_FILE` 下载。默认没有线上源，结果明确无已配置来源。在线下载先验证固定发布身份，再核对清单与字节哈希。`ael release verify --manifest FILE` 只校验本地内容，publisher_identity 保持独立状态。CLI 使用无需 Docker，Server 镜像由服务端维护者管理。下载内容的安装/升级按用户授权和现行安装入口执行。


## 工具与会话版本

`ael version --check --project PATH --json` 分别读取 CLI、项目入口和 Server 版本/协议。当前支持 API 1、Skill 1。旧绑定缺省按 1 兼容，活动会话可用 AEL_API_PROTOCOL/AEL_SKILL_PROTOCOL 明确保留已取得的协议声明。遇到不支持的声明先核对匹配工具，CLI 不把它静默解释为新协议。

`ael upgrade --source TRUSTED_JSON --version EXACT --prefix DIR --check` 只读核对安装与签名发行，`--dry-run` 预览，正式执行先验证下载与内包。当前没有默认在线源。机器级升级为后续 CLI 调用生效，兼容 Remote 项目继续使用原入口和 Skill。显式 `ael update --project PATH` 更新轻入口时保留用户文件和旧离线 payload，并提示后续会话使用新资源。Local update 仍使用完整包及原 writer/维护规则。
