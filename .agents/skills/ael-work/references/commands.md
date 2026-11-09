# 真实命令与恢复

完整参数与数据结构由同一二进制 `./ael help` 输出；以它为准，不猜子命令。

先按工作流程的 W0 判断请求目的。普通讨论、查询、审阅或仅供审核的材料直接处理，不因落盘而运行下列 AEL 流程；用户明确要求某项 AEL 操作时按该请求执行。

- 工具维护：`ael upgrade` 更新本机安装；`ael update` 或 `./ael update` 检查并更新当前已接入工程，`--project PATH` 指定其他工程。`--from DIR` 指定完整本地分发，`--dry-run` 预览。安装维护不领取业务写单元；有运行/中断单元先安全结束或交接，不在 Managed 子执行器中替换工具。旧刷新命令已移除。
- 项目与知识：`./ael doctor`、`./ael knowledge status`、`./ael knowledge search --query TEXT`、`./ael knowledge read --id ID`。原文变更后先审阅差异，再 `./ael knowledge index` 重建来源账本。索引只读取当前工程，绝不执行旧脚本。
- 任务：进入工程流程后，先 `./ael task list`、`./ael task show --task ID` 复用相关工作；新增已授权工程工作用 `./ael task create --file /tmp/definition.json`。原始请求填 original_request，scope/non_goals/acceptance/goal/checks 明确。明确要求使用 AEL 的只验收任务：intent=inspect，goal=report 或 analysis，报告写运行目录；不得改业务源码。
- 执行：`./ael claim --task ID --executor SESSION`，保存 attempt.id/token。`./ael context --task ID` 在领取后加载当前执行的依据。必须在修改前加载；新增范围先补读。执行后提交 `./ael submit --file /tmp/submission.json`，其中 task/attempt/token/key/result 为 help 的 Submission 结构。result 包含 status、事实 summary、真实 artifacts，及 knowledge:{status:unchanged|updated,reason,artifacts:[]}。更新规范后重新 index/context 并重验最终候选。
- 单次代码任务：完成实现和提交前检查后，直接 `git commit` 本任务改动，再提交实施结果并验证提交后的候选。不要将其他任务或用户的未授权改动加入提交；commit 不等于 push、merge 或 release。
- 中途进展：`./ael progress --task ID --attempt AT --token TOKEN --file /tmp/progress.json`；文件是 {summary,artifacts,next}，只写已发生事实，不修改定义版本。
- 命令检查：`./ael verify --task ID --check CHECK --executor NAME`。自动化和场景命令保存真实日志；缺工具/环境是 blocked。
- 模型/场景/人工审查：`./ael assess --file /tmp/assessment.json`。必须提供 task/check/candidate/method(model|scenario|human)/executor/result(pass|fail|blocked)/reason/inputs(受检文件)/artifacts(实际报告或trace)/return_to。模型审查明确标记模型判断；UI 必须真实截图，场景必须真实步骤与结果。无能力提交 blocked，不能改成人工 pass。候选和配置必须匹配。
- 循环：读每次返回的 next_action；continue/repair 继续当前已授权任务，verify 检查失败回 execute；需求/方案问题用 return_to=define|analysis；wait_environment 保存具体原因、恢复条件和恢复节点。交接后当前角色可以结束，下游 task list 可领取；没有运行器时只能称待接手。
- 恢复：`./ael next --task ID`；旧 writer 核实停止后 `./ael resume --task ID --ack-stopped`，正常失败修复无需 ack。候选变动需 `--return-to execute` 后重新提交与验证。不可对存活旧 writer 强行复活。
- 并发：不同任务可以在同一工作区领取和执行；同任务重复领取仍受租约约束。共享文件或外部资源由执行者协调；整个工作区指纹变化可能使验证结果失效。需要文件隔离时绑定独立 worktree。
- 交付：analysis/report/verified 无发布。merged/released 只在原任务有目标及授权时交付，准确候选审批与真实回执缺一不可。`./ael task create` 的 feedback_of 关联新反馈；先去重，不篡改旧结果。
