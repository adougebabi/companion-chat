# 外部 Agent → AEL Client → Server

## 会话恢复

新会话先 `./ael session start --session HOST_SESSION_ID --runner RUNNER --json`，读取 session.skill_path，并在后续 Remote 命令中显式带相同 `--session HOST_SESSION_ID`。客户端会在新的工具会话/领取责任前核对 Server、兼容范围与已授权的更新策略。CLI 取既有不可变版本缓存，Skill 冻结在仓库外。活动会话继续使用其版本，`session show` 展示实际引用，`session end` 只结束工具绑定，仍有 Server 责任时须先完成或明确交接。

安装级策略用 `ael client policy --prefix INSTALL` 查看，用 `--mode notify|auto|pinned` 明确设置。固定版用 `--version EXACT`，显式可信发行源用 `--source TRUSTED_SOURCE_JSON`，清除来源用 `--clear-source`。来源与公钥内容指纹变化须通过此命令重新审阅。init 同样支持 `--release-source TRUSTED_SOURCE_JSON`。手动云端 `upgrade --source ... --check|--dry-run` 保留独立入口。精确回退使用 `upgrade --rollback --from REPORTED_BACKUP_DIR --prefix INSTALL`，校验匹配的旧可执行文件、安装标记与缓存后走原安装事务。

上下文压缩、会话恢复、任务切换或进度不确定后，先执行 `./ael recovery check --session HOST_SESSION_ID --json`。恢复查询读取 Server 当前责任、父/叶任务、候选、规范、检查和本地 HEAD/候选，只写仓库外恢复索引。`online_verified=false` 时只允许只读定位与现场保存。state/next_action 表示应核对的差异或唯一下一动作，领取和提交仍使用原有责任协议。

首次显式绑定使用 `./ael recovery save --session HOST_SESSION_ID --runner RUNNER [--parent EPIC --task TASK]`。每次完成一个可验证单元、消费 submit 回执或切换任务后再次 save。可用 `--file PRIVATE_JSON` 补 next、tried、decisions，文件必须私有且不超过 4 KiB，只写必要交接说明，禁止凭据、租约、聊天、内部思考和全量日志。多个 checkout/session 的索引独立，已完成项会清除当前任务指向，恢复命令不会自动领取后继。

`./ael recovery install --host codex|claude [--dry-run]` 合并项目级 PreCompact、SessionStart，保留用户 Hook 与文件权限，修改冲突需审阅。Codex 安装到 `.codex/hooks.json`，在宿主 `/hooks` 审阅并信任。Claude Code 安装到 `.claude/settings.json`，遵循宿主项目信任及 Hook 审阅。安装结果保持 pending_host_trust，实际触发由宿主确认。缺少宿主支持、未信任或运行失败时，显式恢复命令与短 Skill 入口可用。

PreCompact 仅持久化已有索引并标为待在线核对。SessionStart 的 startup/resume/clear/compact/fork 仅通过 hookSpecificOutput.additionalContext 提供固定规则和恢复命令。Hook 不访问聊天、不调用模型、不领取任务、不运行工程命令、不升级，配置超时 5 秒且按需退出。

## 任务通信

现有 Agent 负责宿主执行，项目通信命令带 `--project PATH --json`。首次接入用统一 `ael init --server ORIGIN --create NAME` 或 `--remote-project ID`，从实际客户端包启动，可通过 `--username USER --password-stdin` 登录，或使用已有 profile/项目 token。交互初始化会选择项目并设置更新策略。项目目录仍使用 `--project PATH`，稳定远程 ID 使用 `--remote-project ID`。旧 profile add/project connect 保留为低层兼容入口，统一初始化复用其绑定和受管安装。

init 默认共用机器级 CLI，支持 `--prefix DIR --from CLIENT_PACKAGE`。项目入口通过仓库外机器映射找到本机安装与配置，项目只保存非敏感绑定、轻入口和 Skill。Hook 安装可选 `--host codex|claude|both|none`，默认探测可用宿主并保留待信任状态。更新策略 `--update-policy notify|auto|pinned` 记录用户选择，固定版可另传 `--pin-version EXACT`。初始化不会自动领取任务，资料和当前任务上下文仍须核对。

接入后再 repository register、knowledge index 与当前责任流程。机器级 upgrade 使用安装 prefix 参数。中间文件放项目外私有目录，不进 Git。每个写请求使用稳定 operation_id/idempotency_key，修改请求必须换键；网络结果未知先查原回执，不回退本地。

### 常规领取与提交

`runner claim --runner RUNNER --file PRIVATE_CLAIM_JSON --out PRIVATE_CLAIM_RECEIPT` 返回的私有文件直接用于后续命令的 `--lease-file`。CLI 从该文件读取 task、attempt、token、fence 和冻结领取版本。`task show.revision` 与 `recovery.source_revision` 用于当前任务状态查询；attempt 写入由 CLI 使用 claim_revision。heartbeat 使用同一文件并更新其中租约期限。

```sh
./ael runner heartbeat --runner RUNNER --lease-file PRIVATE_CLAIM_RECEIPT --file PRIVATE_HEARTBEAT_JSON --session HOST_SESSION_ID --json
./ael runner submit --runner RUNNER --lease-file PRIVATE_CLAIM_RECEIPT --repository REPO --file PRIVATE_RESULT_JSON --session HOST_SESSION_ID --json
```

heartbeat 文件只需 operation_id、idempotency_key。submit 文件提供 operation_id、idempotency_key、result，以及当前检查实际需要的 method、exit_code、inputs。result.artifacts 填选定的原仓库相对文件路径，如 `apps/web/src/views/Page.tsx`。CLI 固定当前候选和选定字节，完成 observe、上传、finalize、绑定后提交。源码扩展名由 CLI 处理，Agent 使用实际交付文件路径。result.knowledge.artifacts 同样接受路径。原始字节、候选、租约与子操作保存在仓库外私有操作记录中。

execute change/pass 示例请求如下，summary、路径和知识结论按实际工作填写。

```json
{
  "operation_id": "execute-result-unique",
  "idempotency_key": "execute-result-unique",
  "result": {
    "status": "pass",
    "summary": "已完成本次修复并提供实际源码",
    "artifacts": ["apps/web/src/views/Page.tsx"],
    "knowledge": {"status": "unchanged", "reason": "本次修复沿用现行规范", "artifacts": []}
  }
}
```

选定的报告、日志和截图放在项目外目录，引用如 `runtime:report.json`、`runtime:desktop.png`，命令增加 `--artifacts-dir ABSOLUTE_DIRECTORY`。verify 的 report.json 由执行 Agent 根据真实检查产生，candidate/check/definition_revision/plan_revision/method/source/observations/covers 按下方协议填写。先用 `runner observe --lease-file PRIVATE_CLAIM_RECEIPT --repository REPO --file PRIVATE_OPERATION_JSON` 获取候选，再据此执行检查和形成报告，submit 会核对当前 checkout。`--dry-run` 只校验本地候选和选定字节并展示文件路径，不执行网络写入。

`runner submit --repository` 模式中省略 bindings 与服务器附件 ID。完整手写请求继续使用下方低层接口。通信失败沿返回 operation_file 执行 `runner retry --pending FILE`，重试使用已固定字节，已接受结果可在租约结束后查询原回执。请求内容变化使用新的操作键。提交后消费 next.next_action，下一单元重新领取并使用新的私有回执文件。到期或撤权继续使用显式恢复流程。

### 低层接口与证据约束

1. `runner register --file PRIVATE_JSON --operation-id OP --idempotency-key KEY`：文件含 id、workspace_id、checkout_id、repository、version、capabilities（例如 development、model、scenario）、expected_revision:"absent"。可选 alias 和 machine 提供客户端别名与设备声明，未提供时使用 runner ID 并显示设备未声明。这是身份/能力声明，不是 OS 隔离证明。
2. `task show --task ID` / `next --task ID` 取得 revision、next_action、current_step；父范围先 `runner next --runner ID --parent EPIC` 查询合法后继，再 claim，不凭查询直接工作。人工责任保持 wait_human；父级叶子与集成条件继续生效。
3. `candidate capture --task ID --repository REPO --checkout CHECKOUT --workspace WORKSPACE --runner ID --expected-revision REV --operation-id OP --idempotency-key KEY`，然后 context publish/show，核对实际原文、版本、workspace_matches、validity。
4. `runner claim --runner ID --file PRIVATE_JSON --out NEW_PRIVATE_FILE`：请求含 task_id、可选 parent_id、context_id、expected_revision、operation_id、idempotency_key。请求文件必须私有（0600），返回的 attempt/token 仅保存到新私有文件，可读及 JSON 输出均不打印 token。保存领取请求用于响应丢失时原键重试到新文件；这只读取原领取结果，不重新授权。
5. 由 Agent 从私有 claim 文件取 attempt.id/token/fence，以私有 JSON 请求调用 `runner heartbeat` / `runner progress` / `runner interrupt`。共有 task_id、attempt_id、token、fence、operation_id、idempotency_key；progress 另含 summary/next/stage，interrupt 另含 reason。续租按当前租约期限进行，无常驻助手也可工作。撤权/过期停止提交有效结果；停工是 Agent 的责任，AEL 不杀宿主进程。失联展示最后上报/未知。
6. 自行使用宿主工具完成本责任。只验收任务不得修改源码。结果产生后 `runner observe --runner ID --repository REPO --file PRIVATE_JSON` 读取本项目候选并提交已有 Attempt observe；文件含共有租约字段及领取时 expected_revision。也可在文件中提供 candidate，此时不传 repository。observe 不启动检查或复制工作区。
7. `artifact upload --task ID --attempt AT --candidate HASH --file LOCAL --lease-file PRIVATE_JSON --expected-revision CLAIM_REV --operation-id OP --idempotency-key KEY`。lease-file 含 runner_id、task_id、attempt_id、token、fence、expected_revision、lease_until（RFC3339；续租后更新）。上传后原件固定。只选任务所需依据、报告、日志与截图；不默认上传所有源码、HOME、聊天或密钥。
8. `runner submit --runner ID --file PRIVATE_JSON`：共有租约字段及 expected_revision（领取时版本）、candidate_id、result、bindings、method、exit_code、inputs。result 含 status(pass|fail|blocked)、summary、artifacts（已 finalized 的 ID）；execute pass 必须有候选中的真实交付文件及 knowledge:{status:unchanged|updated,reason,artifacts:[]}。bindings 为 [{path:"app.txt"或"runtime:report.json",artifact_id:"ID"}]。非 verify 阶段 method 留空，不传 exit_code/inputs；verify 使用声明的 method 和实际 inputs。command 的 exit_code 必须符合 pass/fail。
9. verify 非 blocked 必须上传并绑定 `runtime:report.json`，内容为 {candidate,check,definition_revision,plan_revision,method,source,observations,covers}。source 为 `external-command:RUNNER` / `external-model:RUNNER` / `external-scenario:RUNNER`，observations 是实际观察，covers 完整覆盖本检查的验收项。UI 仍需真实图片。哈希只证明字节一致，不能证明命令真实运行；Agent 不得冒充独立人工审批。必需报告/产物、旧候选、旧验收或权限不符继续被拒绝。
10. 提交后消费 `next.next_action`：repair 留在同一叶子修复；continue 领取下一个当前责任；completed 后读取父范围后继，能力授权满足则继续。交接结束当前角色，由已有外部 Bot 自行领取。等待环境/人工时记录恢复条件，不循环伪进展。`runner receipt --runner ID --idempotency-key KEY` 查原提交回执；重复同一 submit 返回相同结果，不重复推进。旧回执不是新写授权。达到重复失败预算按既有复核规则停止，不改类别规避。

execute 的 change 责任提交 pass 时，至少一个 result.artifacts 附件须绑定候选中的实际交付文件路径，附件 SHA-256 与候选哈希相同。上述 app.txt 是示例候选路径。常规提交入口自动生成这些绑定。单独 artifact upload 使用 --lease-file 时也自动处理源码扩展名，保留原始字节。每次重新领取后，附件需属于新的 attempt、候选与租约。

`runner run`、`runner resume-result`、`runner recover-claim` 明确不支持；不要用它们恢复新模式。稳定本地 Managed/Attached 仍是显式本地功能，不作为远程前置。合并/发布交给外部 Git/CI/部署工具与用户授权，AEL 保存目标与回执，不能将 verified 偷换为 merged/released。没有活跃外部 Agent 就等待接手，不声称后台自动编码。

## 双客户端、断网与恢复

两个客户端使用各自 profile、令牌、runner 和 workspace 标识，连接同一 Server。独立任务可以同时持有责任，同一单元的并发 claim 由服务端事务去重。父计划继续执行既有顺序与 WIP 规则。

runner 写命令和显式 artifact upload 在网络写入前，将完整请求及选定上传字节存入配置目录的 agent-operations。记录绑定服务地址、project、profile、原 checkout 和凭据摘要，权限为 0600，所在目录为 0700。CLI 返回 operation_file。记录含原责任租约，必须保存在项目外私有配置目录。保留的上传字节和候选在重试时固定，原请求文件或工程文件随后改变时，使用 runner retry 读取已保存记录。

通信每轮最多尝试三次，每次 HTTP 请求设置五秒超时，尝试间隔为 100/200 毫秒。每次写入还包含能力查询或恢复查询，单次命令可能包含多个请求。预算耗尽返回 AGENT_OPERATION_PENDING 并保留操作文件。重复失败后由 Agent 读取具体条件并处理，停止无证据重复调用。

```sh
./ael runner retry --pending /private/config/agent-operations/OP.json --project PATH --json
# claim 的恢复仍需指定新的私有租约输出文件
./ael runner retry --pending /private/config/agent-operations/CLAIM.json --out /private/claim-receipt.json --project PATH --json
```

恢复先向服务端查原操作。已接受的结果核对服务端保存的原始请求摘要，再读取原回执。早期回执没有摘要时，通过原接口原键重放完成服务端幂等内容校验。未受理的结果先用原 attempt/token/fence 续租核对，再提交固定的原始请求。撤权、到期、取消或修订变化导致拒绝时，保留本地操作与诊断材料，读取当前 task/next 并按权限显式恢复。旧结果继续绑定原 Attempt。令牌或服务配置变化时，自动恢复会拒绝凭据摘要或作用域不匹配，需核对原身份与服务端回执后处理。

上传恢复先查 upload-status，使用原 start 请求核验完整元数据与幂等键，再续传或读取 finalized 原件。相同字节、不同任务/候选/来源也会产生幂等冲突。记录的校验摘要可检测意外改写。完成的操作记录可在核对服务端回执后由操作者从私有配置目录清理，待确认记录继续保留。

## 显式工作区交接

接收者先注册自己的 runner/workspace，再从自己的 checkout 为当前任务版本准备 candidate/context。源代码同步由宿主工程工具处理。调用下列命令提交私有 JSON 请求。

```sh
./ael runner recover --runner RECEIVER --file /private/recovery.json --project PATH --json
```

请求包含 task_id、attempt_id（原责任）、context_id（接收方当前上下文）、expected_revision、mode、reason、operation_id、idempotency_key。mode 为 confirmed-stopped 或 independent-workspace。前者表示操作者明确核实旧执行者停工，后者表示接收者明确选择独立工作区。独立工作区必须使用不同 workspace_id，并由宿主提供实际隔离。

未知工作区的旧责任不能直接重新领取。独立交接会撤销旧接口责任，记录旧工作区未知状态，并把下一次领取限定到接收方。接收方随后重新获取当前 revision、candidate/context 并 claim。接收方在领取前失联时，可基于当前 revision 针对同一原 attempt 显式更换接收方，每次决定持久记录。存在活跃责任时，恢复必须针对该责任。确认旧执行者已停止后，可针对保留的旧 attempt 提交 confirmed-stopped 释放旧工作区限制，此操作保持其他正在执行的责任。

实施完成后可以把相同候选交给另一身份验收。接收方提供相同候选，服务端保留实施 PASS 历史并要求接收方重新报告必需检查。存在活跃检查时，交接必须明确针对当前检查责任。

活动远程任务的 task cancel 立即撤销有效提交权，并保留宿主状态未知的旧责任。runner show 的 connection_state 表达最后上报或失联，host_execution_state 为 unknown。操作者确认停工后，可通过 confirmed-stopped 清理该责任。发布结果未知时仍按既有外部运行标识核查，交付责任不会通过恢复入口自动重做。

## 统一远程工作台

浏览器登录 Server 后选择项目，进入共用的父子工作台、检查证据、规范/决策和演进页面。中央摘要按项目、Server 实例和事件游标更新，断线时保留最后成功数据并显示过期提示。项目和会话切换会使旧请求失效。

任务详情将协作责任、最后联系、上报阶段和未知宿主状态分别呈现。别名和 machine 是客户端声明，实际会话对应 Attempt。暂停/取消使用当前责任版本和真实权限，结果是撤销有效协作提交并请求外部执行者停工。执行依据与附件入口继续提供原始快照和上传对象的读取。

规范正文来自任务已接收的固定快照。任务提供版本的正文及其相对链接绑定到同一 context 和目标 hash。多个任务引用不同版本时，资料库显示冲突并提供准确快照选择，任务依据仍使用本任务实际提供的版本。

## 外部 UI/UX 发现

先为当前任务修订采集候选并 publish/show context。扫描工具由外部 Agent 在其宿主选择。诊断附件可用 `artifact upload --context CONTEXT --candidate HASH` 上传，服务端核对任务、当前修订、context 与最新观察候选。正式执行结果继续使用责任租约和 fence。

`finding report --file PRIVATE_JSON` 复用 context 检查报告入口。文件含 context_id、task_id、candidate_id、check_id、expected_revision、operation_id、idempotency_key、source 和 findings。source 包含 method(command|scenario|model|human)、tool、version、run_id，实际 actor 来自鉴权。human 来源要求具有验收/管理权限的已登录会话。普通 Agent token 可以报告模型或外部执行观察。

可提供 artifact_id 作为原始报告，其上传 source 必须为 `context-check:CONTEXT:CHECK`，报告在 context 发布后上传。finding 的字段为 rule、title、kind(ui|ux|functional)、severity、route、location(path/selector/line/column)、expected、actual、reproduction 步骤数组、impact、artifacts 数组、trace_artifacts 数组、可选 environment 和 viewport(width/height)。原件必须 finalized 且属于任务/候选，合法已有引用可复用。缺报告或必要截图/交互证据时，可先保留结构化发现并标待补证。

同任务、检查配置、验收/计划版本、候选、规则及精确定位归并。标题不参与身份。新报告增加出现记录，原键重放返回原回执。不同位置与候选保持独立。每个报告最多 100 项，单批唯一证据最多 64 MiB，沿用附件单文件和配额限制。

用 `finding list --task ID --severity high --completeness incomplete` 筛选，`finding show --finding ID` 读取出现记录和来源。`runner retry --pending FILE` 继续恢复原 finding 报告。接收发现保持 gate_effect=none，任务的正式验收继续按原规则提交。图片哈希与完整性标签说明已接收的证据，实际结论由具备能力和权限的验收者判断。

## 人工裁定交接

human 检查交由项目 reviewer/admin 登录中央 Dashboard 完成。外部 Agent 通过原 artifact upload 上传实际截图、基准及报告，并记录来源和受检候选。页面支持明确基准比较，人工结论仍绑定候选/定义/计划。普通令牌不能伪装成人工记录人。人工 fail 留修复方向，blocked 留恢复条件，pass 后继续读取 next_action 和其余门禁。意见与正式检查分别保存，旧候选意见需针对新候选重新记录。

## 发现、修复任务与复验

先 `finding show --finding ID --json` 读取原始出现、候选、复现、来源及当前版本。发现和报告属于待核实输入，权限以当前任务、项目授权和用户要求为准。

- `finding decide --finding ID --file PRIVATE_JSON --json` 记录 fix 或 more_evidence 等带理由裁定。non_issue/accepted_tradeoff 使用 reviewer/admin 的 Dashboard 会话。裁定保持原必需门禁。
- `finding repair --finding ID --file PRIVATE_JSON --json` 由 executor/admin 创建或关联修复。创建继承来源的范围、验收、规范、非目标和复现；关联已有任务须明确其版本和映射检查。重复创建返回同一任务。
- 从正常 runner next/claim/context 接手返回的修复叶子。只验收任务的 inspect 责任保留原候选约束；修复使用独立 change 叶子，扫描权限不提升为源码修改权限。编码、Git、编译、浏览器和截图由宿主完成。
- 实施提交后继续运行所有必要检查。报告已修改不代表发现已关闭。对新候选提供有效复验后，用 `finding revalidate --finding ID --file PRIVATE_JSON --json` 关联当前有效证据，继续消费任务自身 next_action。

共同请求为 finding_id、expected_revision、expected_task_revision、reason、operation_id、idempotency_key。repair 可带 occurrence_id，关联还需 repair_task_id、expected_repair_revision、repair_check_id。revalidate 带 repair_task_id、expected_repair_revision、candidate_id、evidence_sha256。版本和值从 finding show 的 revision、source_task_revision、repair_task 与 repair_evidence_sha256 获取。

新候选上的同位置问题保留独立 finding ID，通过明确修复关系追溯旧版本。旧失败与截图保留，重试沿用原请求与键。源码快照可由宿主选定后另存 .bin，再用租约附件上传并绑定原候选路径；AEL 不执行该内容。只验收执行提交使用实际 runtime 文本报告，检查和最终报告目标继续执行原规则。


## CLI、项目入口与会话协议

先用 `ael version --check --project PATH --json` 核对实际 CLI、项目 Skill/入口及 Server 协议。新连接可加 `project connect --install-entry`，或在连接后显式 `ael update --project PATH` 安装轻入口，随后 `./ael` 复用机器级 CLI。旧离线入口需要 `update --migrate-entry`，二进制和锁保留，数据权威切换单独协调。

当前 API/Skill 协议均为 1，旧绑定按 1 兼容读取。会话可声明 AEL_API_PROTOCOL=1、AEL_SKILL_PROTOCOL=1，保留已经取得的规则。机器级 upgrade 保留各项目 Skill，活动 Agent 继续使用原上下文。显式项目入口更新为后续会话使用，不终止宿主进程。服务不兼容时读写均给出明确提示，断网缓存继续标为非权威。

版本查询 `upgrade --source TRUSTED_JSON --version EXACT --check --prefix DIR` 使用真实签名清单和目标安装，dry-run 不下载资产。正式升级才下载并验证包、暂存和原子切换。updater 自身中断可用原包 `upgrade --from ORIGINAL --prefix DIR --ack-stopped` 恢复，仍核对其停止身份。该参数的安装恢复与任务责任恢复各自使用对应原始记录。


## 服务维护

读取 server_maintenance 的 mode、epoch、deadline。draining 时保存现场并按授权释放当前责任，截止后接口拒绝有效写入。quiescing/quiesced 时等待维护完成，保留原请求与私有材料。维护后先核对协议和当前责任，查原回执，再根据当前版本处理新领取或 runner recover。unknown workspace 仍需停工确认或独立工作区声明。恢复备份会要求管理员重新签发项目令牌，旧回执可作为历史查询。宿主工作由现有 Agent 自行管理。

## 已有 Local 项目切换

已有 Local 状态通过 transfer scan/plan/export/import/verify/commit/cutover 显式迁移。先审阅来源映射并协调外部责任，未知旧执行状态阻止冻结。切换后读取 Server 当前任务、上下文和回执，再领取新的有效责任。历史 PASS 保留来源版本，新候选继续经过现行验收。异常恢复使用原导出与操作键，回滚先通过 rollback-freeze 核对中央新增事实。

## 远程父级集成与有限失败复核

所有必需叶子达标后，由宿主同步所需源码，在最终父候选上 capture/publish 当前上下文，再调用 `ael task integrate --task EPIC --context CONTEXT --expected-revision REV --operation-id OP --idempotency-key KEY --json`。声明完整影响文件的任务另提供 `--impact-artifact ARTIFACT_ID`。Server 校验同一仓库、不可变清单、输出附件、规范依据和工作区责任。Remote 采用 `parent_rechecked`，所有叶子的必需检查须映射到含义及配置一致的父检查，并在最终候选上重新执行。`impact_unknown` 保留输入影响未知事实。此路径的版本证明来自候选清单和重新检查，Git 祖先关系仍由宿主核查。

消费 `runner next --parent EPIC` 并通过 `runner claim` 领取准备后的父级检查。宿主实际运行命令、浏览器和截图，再按原租约上传并提交。人工检查由 reviewer/admin 登录看板审阅准确候选。新增父候选观察后，重新准备集成与检查。

父候选中的源码输出须与已验收叶子的内容哈希及执行位匹配。同一输出被后续工作改写时，先更新并验收受影响叶子，取得可核对的当前输出，再准备父集成。

达到重复失败阈值时，保存真实调查材料并上传为同任务 finalized 附件。调用 `ael task review-failure --task ID --file PRIVATE_REVIEW_JSON --expected-revision REV --operation-id OP --idempotency-key KEY --json`。文件包含 `tried,excluded,hypothesis,next,recovery,evidence`，evidence 为中央附件 ID，actor 由认证身份确定。每次至多 32 个引用和 64 MiB 总量。成功后按返回版本显式 resume，失败次数保留。复核后再次失败进入 blocked，再次复核需要此前未审阅过的证据内容。宿主自行停止旧工作并保留现场。
