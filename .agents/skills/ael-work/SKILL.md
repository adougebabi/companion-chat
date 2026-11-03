---
name: ael-work
description: 在已初始化 AEL 项目中领取并推进明确的工程任务，支持当前 Agent 连续执行或节点交接；普通讨论和资料查询不创建任务。
---

先运行 `ael doctor --project .` 和 `ael task list --project .`。无 CLI 则按工具 README 安装；shadow 模式只能预览，不能伪造执行。

选择用户要求的任务；实施但无任务时用 `ael task create --file definition.json` 建立最小定义。阅读验收和计划，然后：

1. `ael claim --task ID --executor SESSION`；记录返回的 attempt.id、token、workspace 与修订。
2. `ael context --task ID` 获取有来源的约束。只处理当前步骤；不启动嵌套 loop。
3. 完成当前步骤，用 `ael submit --file submission.json` 提交 StepResult。提交格式见 `ael help`。写入型执行至少附真实交付物相对路径；验证命令用 `ael verify --task ID --check CHECK`，主观 pass 不能替代它。
4. 按 next_action 继续 claim、交接、等待依赖/确认或结束。交接提交后无需自行调度下一个 Bot。长步骤用 heartbeat 保持租约。

未经用户明确授权，不发布、切换真实旧工作流或降低必需门禁。过期 Attempt 先核实旧进程已停止，再 resume --ack-stopped。定义变化须确认准确修订。完成后新反馈建立 feedback_of 关联任务，保留原历史。普通已授权范围持续执行，不逐步询问。
