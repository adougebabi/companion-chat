# 执行计划

- [x] 将临时 overlay 失败样本变为真实 Tool/ADK 边界回归测试，先观察对象/字符串及多调用进度失败，再修至绿色。
- [x] 在 `commit_review` 输入边界实现有界观察形态规范化；Provider Tool schema 和 Agent 指令均明确首选数组。
- [x] 依赖 `09-27-capability-contract` 的非 retryable 参数错误分类；模型第二次物理调用收到字段/类型反馈。
- [x] 失败后可在既有请求期限内修正调用；进度只统计一次非 replay 的有效提交。
- [x] 隔离 PostgreSQL/MinIO 实图 Agent 回归、对象/字符串/数组、坏类型、业务矛盾与重复调用测试通过；完整纯 Go 测试及 vet 通过。
- [x] Trellis check 已复核输入上限、真实图片、ADK 修正回合、幂等/CAS、诊断脱敏与规范同步；无额外代码问题。
