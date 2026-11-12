# M2-13 动态 NPC 评论与停用恢复

本项只核验 T05 已有动态互动，不新增 NPC 或插件能力。隔离 PostgreSQL 中通过
正式世界入口建立已确认结识，通过正式 `moment.publish` 发布动态，再由正式
`ProcessCognitionInbox` 处理 NPC 动态互动。受控 Provider 返回 NPC 评论。

三种场景分别核对：正常评论及终态重放只有一条评论和一条 outbox；停用 Moment
且尚无持久决定时，不调用 NPC 模型或发布评论；决策已保存后，在测试库的评论
插入点注入明确数据库错误，停用插件再恢复时复用原决定、只提交一次评论且不
再次调用 NPC 模型。相关角色、世界、动态和数据库均为测试夹具。

第一次编译因磁盘空间不足失败，见 `m2-13-first.log`；清理本会话中可从已提交
源码和清单重建的临时导出后，三种场景通过，见 `m2-13-after-space.log`。
随后增加对注入错误内容的断言，常规编译再次因空间不足失败，见
`m2-13-final-build-failure.log`；利用缓存以 `-vet=off` 运行更严格的最终测试，
三种场景通过、无 SKIP，见 `m2-13-final-no-vet.log`。此前默认 vet 的直接相关
回归 5 个顶层测试通过，见 `m2-13-regression.log`；最终测试的单行断言差异和
两版受检哈希分别保存在 `m2-13-test-assertion.patch` 和两个 manifest。

受控 Provider 与隔离数据库不代表真实模型、真实 Temporal 或完整 NPC/生活系统
验收；M2 和父 T05 的其他阻塞仍然保留。
