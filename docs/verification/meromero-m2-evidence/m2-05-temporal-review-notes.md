# M2-05-T 真实 Temporal 中的图片提交不确定状态

一次性 Temporal `temporalio/auto-setup:1.29.7` 使用独立 PostgreSQL 容器；应用
测试使用本机另一个一次性 PostgreSQL 数据库。服务仅绑定本机回环地址，健康
检查为 `SERVING`，见 `m2-05-temporal-environment.log`。首次内部网络未发布
端口；诊断日志确认服务实际启动后改用独立网络和回环端口，见启动日志。首次
健康检查误用容器内回环地址，改用容器服务名后通过。测试日志的本机 Worker
名称已脱敏。

受检源码基于 `9d25d208c0d8dab42ea7b8ff8fde1a69f04e7d37` 加测试文件，
哈希见 `m2-05-temporal-final-manifest.json`。真实 Temporal Worker 执行正式
`MediaWorkflow` 和 `ProcessMediaActivity`：测试库预置已持久提交围栏、没有
Job ID 的状态后，媒体记录为 `submission_unknown`；工作流历史中活动 attempt
为 1，受控渲染端收到 0 个请求，未核对的恢复与重新生成均被拒绝且没有恢复
记录。干净候选及含四处原有草稿的工作区均实际执行并通过、无 SKIP，见
`m2-05-temporal-final.log` 和 `m2-05-temporal-working-tree.log`。另有 4 项
相关受控工作流回归通过，见 `m2-05-temporal-regression.log`。

首次测试自身因未使用的导入而编译失败，见 `m2-05-temporal-compile-failure.log`；
移除该导入后通过，再补充活动 attempt 和未核对恢复断言。此测试从已持久的
围栏开始，未连接真实 ComfyUI，也不证明真实服务接单后丢响应的完整链路。
包装上下文取消/超时的特殊重试边界仍在关联 AEL 工单中，Studio 和视觉身份的
人工处理缺口仍未完成。
