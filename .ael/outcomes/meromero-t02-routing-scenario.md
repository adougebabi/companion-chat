# T02 模型路由场景记录

受检提交：`604a28aa7553a05419552fc02e5c80c1498ecd5b`。数据库均为一次性隔离 PostgreSQL，受控 Provider 不代表真实模型质量。

1. `TestAgentModelConfigurationSavesReadsRunsAndClears` 在隔离数据库中保存默认 MeroMero 与单个 Qwen 任务覆盖；检查其他任务仍用 MeroMero、覆盖任务实际请求携带 Qwen ID，并检查恢复继承。日志：`/private/tmp/fluctlight-t02-postcommit-routing.log`。
2. 同一日志覆盖变体/Agent/默认优先级、拒绝不调用备用模型，以及真实图片结构经独立视觉绑定发送至受控 Provider。React 覆盖和恢复继承测试通过。
3. 最新提交的隔离恢复测试覆盖 Goal 部分提交、每日记忆、片段摘要、embedding、累计摘要和回复续接模型绑定；日志：`/private/tmp/fluctlight-t02-postcommit-recovery.log`。累计摘要和 embedding 的新迁移及前序链通过：`/private/tmp/fluctlight-t02-postcommit-migration.log`。
4. 本机指定 MeroMero 的普通会话真实调用成功，1 次物理请求，返回原生 reply 并持久化 1 条回复：`/private/tmp/fluctlight-t02-meromero-live-final.log`、`/private/tmp/fluctlight-t02-meromero-live-final.json`。这是普通会话，不是 T04 私聊单主链验收。
5. 本机 `Huihui-Qwen3.8-27B-3.8bpw` 的真实反思 Agent 测试通过：`/private/tmp/fluctlight-t02-qwen-reflection-live.log`。该真实夹具使用 Qwen 作为其配置模型；单个任务覆盖不影响其他任务的断言来自第 1 条受控测试。
6. 真实 Temporal 的跨版本运行及真实视觉模型图片识别未在本项运行；没有用文本测试冒充视觉验收。升级前无可证明绑定的旧任务及诊断尽力写入限制见复核记录。
