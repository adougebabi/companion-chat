---
name: ael-work
description: 在已初始化 AEL 项目中领取并推进明确的工程任务，支持当前 Agent 连续执行或节点交接；普通讨论和资料查询不创建任务。
---

从项目根目录使用仓库自带的 `./ael`，无需全局安装。先运行 `./ael doctor --project .` 和 `./ael task list --project .`。若入口/锁文件/对应平台二进制缺失，报告工具资产不完整；不要用其他全局版本代替，也不要重做项目初始化。shadow 模式只能预览，不能伪造执行。

选择用户要求的任务；实施但无任务时用 `./ael task create --file definition.json` 建立最小定义。阅读验收和计划，然后：

1. `./ael claim --task ID --executor SESSION --owner-pid PID`；PID 必须是本次 Agent 的常驻进程，不是短命 shell、AEL 自身或任意无关进程。独立续约器在模型排队/工具等待时每 20 秒续约，提交或 owner 退出时停止。无法可靠识别常驻 PID 时省略此参数，使用下述租约规则。记录返回的 attempt.id、token、workspace 与修订。
2. `./ael context --task ID` 获取有来源的约束。只处理当前步骤；不启动嵌套 loop。
3. 完成当前步骤，用 `./ael submit --file submission.json` 提交 StepResult。提交格式见 `./ael help`。写入型执行至少附真实交付物相对路径；验证命令用 `./ael verify --task ID --check CHECK`，主观 pass 不能替代它。
4. 按 next_action 继续 claim、交接、等待依赖/确认或结束。交接提交后无需自行调度下一个 Bot。默认租约 30 分钟，policy.lease_seconds 可配置；未绑定 owner 的长步骤在有效期内用 heartbeat 续约，不假装有后台续约。Managed 已独立续约，续约失败会停止子执行器。

默认无任务时间上限；显式正数预算仍生效。已有任务的旧预算需要调整时用 `./ael task budget --task ID --max-seconds 0 --reason 理由`，0 表示不限时；只改变调度预算，不改变验收条件。误建任务用 `./ael task delete --task ID --reason 理由` 真正删除；有活跃执行或被其他任务引用时须先处理对应阻塞。test、qa、verify 是合法类型。

任务检查点和脱敏历史自动写入 `.ael/records/`，与定义一起提交 Git。新机器读取后须确认恢复并重新验证，不能沿用旧租约或授权。Git 不同步运行日志/截图等原始产物；需要它们时另做 `storage backup/restore`。

未经用户明确授权，不发布、切换真实旧工作流或降低必需门禁。过期 Attempt 先核实旧进程已停止，再 resume --ack-stopped。定义变化须确认准确修订。完成后新反馈建立 feedback_of 关联任务，保留原历史。普通已授权范围持续执行，不逐步询问。
