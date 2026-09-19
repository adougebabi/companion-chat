# 摇光项目第五阶段：前四阶段联合验收与最终审查证据

## Goal

在当前最终代码快照上，对第一至第四阶段进行要求—实现—测试—证据核验，执行可安全运行的跨阶段回归，必要时做最小修复，并交付可供外部审查者复核的脱敏报告与证据包。报告不得把阶段总结、编译通过、Fake 或 curl 结果扩大解释成真实模型、数据库、Temporal、部署或浏览器验收。

## Requirements

- 冻结仓库/提交/工作区/工具和依赖版本；记录隔离环境与缺失条件。
- 建立带稳定 ID 的 acceptance matrix，覆盖第一阶段 Eino/Prompt/Task、第二阶段 ADK/人格能力、第三阶段 WakeUp/后台权限、第四阶段无独立 BFF API 接入，以及跨阶段幂等/取消/事务/部署连接点。
- 以当前源码和真实调用路径核查实现，区分 PASS、FAIL、BLOCKED、NOT_RUN、NOT_APPLICABLE。
- 运行现有 Go、Web、browser-client、OpenAPI、部署配置、静态扫描、race/vet 和阶段专项测试；保留原始失败与修复后结果。
- 使用 Fake ChatModel/Tool 证明 ADK 工具闭环、人格边界、WakeUp 权限与取消；可安全时使用真实隔离 PostgreSQL/Redis/Temporal/浏览器/Provider，否则明确阻塞原因。
- 检查并修复直接违反前四阶段已确认契约的最小缺陷，不扩张架构、不改产品规则、不关闭安全校验。
- 生成 `docs/verification/phase-5/<run-id>/`：主报告、CSV 矩阵、manifest、脱敏证据片段/日志和 `review-bundle.zip`。

## Constraints

- 不清空用户数据库、Redis、Temporal、MinIO 或生产容器；不向真实用户发送主动消息或媒体。
- 不提交密钥、Cookie、Session token、完整数据库、完整 Prompt/人格/记忆、Reasoning、node_modules 或模型文件。
- 不因缺少真实外部环境把测试改成 Skip 后宣称通过；不恢复任何已删除 BFF、旧 Provider、旧循环或双轨 fallback。
- 第五阶段只做验收、证据和最小修复，不新增 Agent 平台、调度系统、人格/记忆规则、协议或无关性能优化。

## Acceptance Criteria

- [x] 有可独立阅读的要求—实现—测试—证据矩阵和分阶段结论。
- [x] 四条关键证据链 A（对话 ADK）、B（人格接管/切换）、C（WakeUp）、D（无 BFF 浏览器 API）均给出代码符号、测试断言和证据文件。
- [x] 已执行的命令、退出码、测试数量/Skip、源码快照和环境均可追溯；修复前失败与修复后结果不混淆。
- [x] 关键跨阶段回归覆盖工具回填、权限、事务副作用、取消、幂等、Outbox/Temporal/Worker 接线和部署清理；无法运行的部分标记 BLOCKED。
- [x] 外部依赖缺失项被准确标为 BLOCKED/NOT_RUN，不能用 L0/L1 证据替代 L2/L3/L4 结论。
- [x] 报告、CSV、manifest、evidence 和 zip 均已生成并脱敏；源码快照保持干净，任务归档与 journal 待收尾步骤完成。
