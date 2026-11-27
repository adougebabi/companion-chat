# 真实命令与恢复

完整命令由 `./ael help` 列出；机器数据结构以同一二进制 `./ael help --json` 为准，不猜子命令。下列命令作为 Agent 或脚本使用并需读取字段时，一律追加 `--json`，例如 `./ael claim --task ID --executor SESSION --json`、`./ael next --task ID --json`。JSON 输出协议为 1；成功返回码 0 只表示调用成功，必须读取 next_action 和证据判断完成、等待或失败。

先按工作流程的 W0 判断请求目的。普通讨论、查询、审阅或仅供审核的材料直接处理，不因落盘而运行下列 AEL 流程；用户明确要求某项 AEL 操作时按该请求执行。

- 工具维护使用 `ael upgrade` 更新机器 CLI，支持 `--from CLIENT_ARCHIVE_OR_FULL_BUNDLE`、显式可信 `--source JSON`、精确 `--version`、`--prefix`、`--check` 和 `--dry-run`。`ael version --check --project PATH` 报告 CLI、入口和 Server 版本。Remote 兼容项目共用机器工具，显式 `update --project PATH` 安装/更新轻入口，遗留受管离线入口用 `--migrate-entry` 并保留旧 payload。Local update 保留完整包与运行/中断责任、维护锁保护。不在 Managed 子执行器中替换其工具，外部 Agent 使用当前会话已取得的协议。
- 独立 Server：管理员使用 `ael server bootstrap --data-dir /absolute/local/DIR --username NAME --password-stdin` 初始化专用空目录，再用 `ael server start --data-dir /absolute/local/DIR` 启动 loopback 服务；密码从标准输入读取，不放在命令行。浏览器登录后管理项目、成员与可撤销令牌；服务端不接受任意客户端路径或执行任务命令。协议、TLS 代理与本机卷限制见 `docs/standalone-server.md`。
- 远程任务：使用 `ael server profile add --name NAME --url ORIGIN --token-stdin` 在仓库外保存令牌，再用 `ael project connect --profile NAME --remote-project ID --project PATH` 显式绑定。远程写操作提供 `--operation-id`、`--idempotency-key` 和 Server 返回的 `--expected-revision`；新建默认为 `absent`。超时先 `ael operation show --idempotency-key KEY --json`，不要换键重做。离线缓存会明确标记非权威及时间，写入不会回退本地；计划应用的 operation ID/幂等键须与 proposal ID 相同。完整命令见 `docs/remote-tasks.md`。
- 中心附件：`ael artifact upload --task ID --file LOCAL --expected-revision REV --operation-id OP --idempotency-key KEY --json` 上传并 finalize，任务已有候选时须加 `--candidate HASH`。以 `artifact list --task ID`、`artifact show --artifact ID`、`artifact download --artifact ID --out NEW_FILE` 读取；关联可用 `artifact reference --artifact ID --task ID --purpose reference --expected-revision REV --operation-id OP --idempotency-key KEY`。上传中断使用相同参数和键重试；源文件不可用或响应丢失时用 `artifact upload-status --idempotency-key KEY` 查询自己的 upload ID，确需放弃时用 `artifact cancel --upload ID --operation-id OP --idempotency-key KEY`。附件关联不是检查通过结论；预览、配额和清理说明见 `docs/remote-tasks.md`。
- 远程候选和依据：显式 `repository register --repository ID --operation-id OP --idempotency-key KEY`，本机 `knowledge index` 后用 `candidate capture --task ID --repository REPO --checkout CHECKOUT --workspace WORKSPACE --runner RUNNER --expected-revision REV --operation-id OP --idempotency-key KEY`；再 `context publish --task ID --observation OBS --expected-revision REV --operation-id OP --idempotency-key KEY` 登记依据版本。仅在获授权共享规范正文时添加 `--share-source PATH`。`context show --task ID --json` 检查当前任务和本机候选；历史记录用 `--context ID`，读取 `validity`、`workspace_matches` 和 `input_equivalence_proven`，未知输入不能声称检查等价。`candidate compare --before ID --after ID` 显示变化路径。正式结果用当前检查责任的 `runner submit` 登记。需要额外的诊断报告时，以 `artifact upload --source "context-check:CONTEXT_ID:CHECK_ID"` 上传该上下文建立后的报告，再 `context record-check --context ID --task ID --check CHECK --artifact REPORT --expected-revision REV --operation-id OP --idempotency-key KEY`；客户端报告不自动通过门禁，也不能单凭上传时间证明检查何时执行。快照来源、范围和恢复方式见 `docs/remote-tasks.md`。
远程实际执行接入见 [外部 Agent 通信协议](remote-agent.md)。下方 claim/verify/assess/work/loop 为稳定本地模式入口，远程 authority 不执行它们；远程使用 runner 通信命令。

- 项目与知识：`./ael doctor`、`./ael knowledge status`、`./ael knowledge search --query TEXT`、`./ael knowledge read --id ID`。原文变更后先审阅差异，再 `./ael knowledge index` 重建来源账本。索引只读取当前工程，绝不执行旧脚本。
- 任务：进入工程流程后，先 `./ael task list`、`./ael task show --task ID` 复用相关工作；新增已授权工程工作用 `./ael task create --file /tmp/definition.json`。原始请求填 original_request，scope/non_goals/acceptance/goal/checks 明确。明确要求使用 AEL 的只验收任务：intent=inspect，goal=report 或 analysis，观察直接写入正文，约定的文件可写入运行目录。源码保持只读。
- 任务树：大任务定义使用 `type: "epic"`；创建叶子时 `parent` 可指向本项目已存在的 Epic（默认必需）。已有叶子使用 `./ael task link --parent EPIC --task LEAF --milestone 阶段 --json` 关联，可选叶子加 `--required=false`；用 `./ael task tree --task EPIC --json` 查看真实子任务状态，`./ael task progress --task EPIC --json` 使用相同汇总。必要叶子达标后，`./ael task integrate --task EPIC --json` 核对内容并记录父候选，再用 `verify --task EPIC --check CHECK --json` 在准备候选的工作区逐项运行父级必需检查；缺少内容证明或候选变化时先核实，不能把叶子通过当作父级集成通过。后续叶子改写了先前叶子的文件时，只要后一叶子的受验输入正是前一叶子的结果，集成即视为已纳入并要求复验：已完成叶子保持完成，不要 resume 或 edit 叶子，也不要另建复验叶子；报 `EPIC_RECHECK_REQUIRED` 时编辑父定义，添加与该叶子必需检查同类型、方法、命令、超时和产物的必需检查并在 `affected_tasks` 列出该叶子，再 integrate 并在父候选上运行。最后验收叶子之后又合入父任务之外的改动（合并、独立任务、知识索引刷新）时，不要补建事后叶子或退回旧提交，用 `./ael task integrate --task EPIC --external-file /tmp/external.json --json` 声明 `{"base":"最后验收的必需叶子","sources":["提交/合并/任务"],"reason":"说明"}`；起点叶子之后的差异记入集成记录并复验，起点结果中不存在的叶子内容仍拒绝。与其他工作或同一父级其他检查共用可写工作区的父检查只保留历史结果；核实旧工作停止后重新准备父候选并逐项复验。独立运行库和外部编辑器不受这一协调保护，应使用稳定隔离工作区。各类叶子计数可以重叠，发布目标仍需明确确认和真实回执。
- 拆分：先用 `./ael plan prepare --task EPIC --file input.json --json` 读取用户原文、已有分项/任务及 schema；由 Agent 按实际语义拟定 proposal，原样携带 input_hash、请求和用户分项，保留用户 ID 并复用已有任务。用 `./ael plan preview --file proposal.json --json` 检查字段差异、父验收覆盖与摘要，再 `./ael plan apply --file proposal.json --digest HASH --json`；相同计划重试须使用原文件和摘要。范围变化导致的确认须处理，但不为每个已授权子项重复申请。失败先修复并重试，不删除部分成果；完整备份恢复后先核对并解除相关任务的恢复确认/暂停，再用 `./ael plan recover --task EPIC --plan-id ID --reason 理由 --json` 恢复原计划。确需放弃时用 `./ael plan abandon --task EPIC --plan-id ID --reason 理由 --json`，已产生任务保留。仅验收项声明 `inspection_only: true`，不能修改业务文件。
- 执行：`./ael claim --task ID --executor SESSION`，保存 attempt.id/token。`./ael context --task ID` 在领取后加载当前执行的依据。必须在修改前加载；新增范围先补读。执行后提交 `./ael submit --file /tmp/submission.json`，其中 task/attempt/token/key/result 为 help 的 Submission 结构。result 用 status、summary 登记实际工作；普通任务可写 `artifacts: []`。改动路径记录在范围和候选清单中。不要把改过的源码自动列为附件或交付文件。显式要求的交付文件列入定义 deliverables，检查文件列入相应 checks[].artifacts。可按需补充 knowledge。失败类别仅在确有不同问题和实际失败证据时填写稳定 `failure_class`，同一问题不得改写类别规避计数；未分类失败保守累计。更新规范后重新 index/context 并重验最终候选。
- 父范围顺序推进：`./ael work next --parent EPIC --runner RUNNER --capabilities 能力列表 --json` 后用 `work claim` 原子领取；消费 `checkpoint.task/unit` 与 `next.attempt.id/token`。每次提交或检查后继续同一叶子，失败先修复并复验；叶子完成再查询下一项。命令检查用 `verify --task LEAF --check CHECK --executor RUNNER --attempt AT --token TOKEN --json` 执行已领取的检查。交接时先结束 attempt；后继可自行领取同一未完成叶子，也可用 `work release --parent EPIC --runner RUNNER --reason 理由 --json` 显式记录交接，不安排其他 Bot。Managed 父运行器用 `loop run --parent EPIC --runner RUNNER --config executor.json --capabilities 能力列表 --json`，读取 stop_reason 与 checkpoint，不递归启动 loop。
- 单次代码任务：完成实现和提交前检查后，直接 `git commit` 本任务改动，再提交实施结果并验证提交后的候选。不要将其他任务或用户的未授权改动加入提交；commit 不等于 push、merge 或 release。
- 中途进展：`./ael progress --task ID --attempt AT --token TOKEN --file /tmp/progress.json`；文件含 {summary,artifacts,next}，只写已发生事实，不修改定义版本。真实交付物需显式加 `evidence_kind: "deliverable"` 才更新有证据进展；未分类或 `diagnostic` 仅记录，日志不计成果。长阶段可附 `stage` 和未来的 RFC 3339 `expected_until`；它们不代表新的证据或自动延长检查超时。
- 命令检查：`./ael verify --task ID --check CHECK --executor NAME`。自动化和场景命令保存真实日志；缺工具/环境是 blocked。
- 模型/场景/人工审查：`./ael assess --file /tmp/assessment.json`；若已通过 `work claim` 领取，须追加 `--attempt AT --token TOKEN`，assessment 的 executor 与领取 Runner 一致，不能再次领取。提供准确的 task/check/candidate/method/executor/result 和实际观察 reason。inputs 可关联具体受检路径，artifacts 按检查约定提供。场景登记实际步骤与结果。具名人工确认使用人工验收入口。无能力提交 blocked，不能改成人工 pass。候选和配置必须匹配。
- 循环：读每次返回的 next_action；continue/repair 继续当前已授权任务，verify 检查失败回 execute；需求/方案问题用 return_to=define|analysis；wait_environment 保存具体原因、恢复条件和恢复节点。交接后当前角色可以结束，下游 task list 可领取；没有运行器时只能称待接手。
- 恢复：`./ael next --task ID`；旧 writer 核实停止后 `./ael resume --task ID --ack-stopped`，正常失败修复无需 ack。重复失败达到阈值时先用 `./ael task review-failure --task ID --file /tmp/review.json --json` 记录 `actor,tried,excluded,hypothesis,next,recovery,evidence`，引用真实可读取的证据；再 resume。复核后仍失败会停止，再次复核须有新的证据内容，不清零失败次数。候选变动需 `--return-to execute` 后重新提交与验证；远程验收领取因候选无法还原而持续 `CANDIDATE_CHANGED` 时，executor 可按错误 detail 自行 resume 回到执行。不可对存活旧 writer 强行复活。
- 并发：不同任务可以在同一工作区领取和执行；同任务重复领取仍受租约约束。共享文件或外部资源由执行者协调；整个工作区指纹变化可能使验证结果失效。需要文件隔离时绑定独立 worktree。
- 交付：analysis/report/verified 无发布。merged/released 只在原任务有目标及授权时交付，准确候选审批与真实回执缺一不可。独立新工作按授权去重后用 `task create` 的 `feedback_of` 关联原任务。同范围的收尾纠正见[已完成任务的交付修正](#已完成任务的交付修正)。

双客户端恢复使用 `runner retry --pending PRIVATE_OPERATION_FILE`，领取回执追加 `--out NEW_PRIVATE_FILE`。显式工作区交接使用 `runner recover --runner RECEIVER --file PRIVATE_JSON`。请求字段、权限、原责任和有界重试规则见 [远程 Agent 协议](remote-agent.md)。

外部发现通过 `finding report --file PRIVATE_JSON` 接入。`finding list` 支持任务、候选、严重性、完整性和文本筛选，`finding show --finding ID` 给出所有出现及原件。完整请求见 remote-agent.md。

远程 human 检查使用中央 Dashboard 的会话人工验收入口。截图、基准和报告仍由外部客户端调用 artifact upload 上传，页面明确选择比较来源。reviewer/admin 的结论绑定准确版本与中央证据，普通 Agent 令牌无法调用人工裁定。详见 remote-agent.md 的人工裁定交接。

发现闭环新增 `finding decide`、`finding repair`、`finding revalidate`，均接收 `--finding ID --file PRIVATE_JSON`。发现版本和来源任务版本、理由及操作键必填；修复创建需要 executor/admin，非问题/权衡需要授权人工会话。复验从关联任务当前有效证据读取 pass，并核对新候选、仓库和字节/hash。完整字段和宿主执行顺序见 remote-agent.md。

## 已完成任务的交付修正

按 [FEEDBACK](workflow.md#10-feedback修正与独立新工作) 核对分类和权限后，复用已有交付记录。Remote 可直接在原任务下追加普通附件。先用 `ael task show --task ID --json` 读取当前修订和历史候选，用 `ael artifact list --task ID --json` 去重。使用当前 Remote 会话上传。

```sh
ael artifact upload --task ID --file CORRECTION.md --source delivery-correction \
  --expected-revision REV --operation-id OP --idempotency-key KEY --json
```

如果原任务有候选，加 `--candidate ORIGINAL_CANDIDATE` 以通过附件的元数据校验。新提交的完整 SHA 和实际检查写在正文。使用普通附件上传，无须复用已结束的 attempt 或租约。记录是原任务的参考资料，正式验收和交付门禁仍使用既有候选与检查规则。网络重试沿用原操作键。

Local 优先在已有项目回执或证据位置追加一条带任务 ID 和提交 SHA 的记录。没有现成位置时，Git 仓库可在原交付提交上复用 Git note。先用 `git notes --ref=ael-delivery show ORIGINAL_SHA` 检查对应运行和提交是否已记录，再用 `git notes --ref=ael-delivery append -F CORRECTION.md ORIGINAL_SHA` 追加。该记录存在本地 Git 引用中，普通提交不会传播它。需要团队可见时使用已有且获授权的共享交付记录。保持 AEL 管理的检查点、终态文件和原回执完整。
