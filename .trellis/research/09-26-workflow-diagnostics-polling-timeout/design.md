# 技术设计

## 前端读取边界

把工作流列表从 `loadDiagnostics()` 的五路并发请求中拆出为独立 `loadWorkflows()`。该方法只由工作流区进入事件和显式刷新按钮调用，以 store 内的进行中标记合并重复触发。现有 2 秒定时器仍只刷新另外四类诊断。清空 PostgreSQL 诊断记录不重置仍在运行的 Temporal 列表请求或其进行中标记，避免再次进入该区时重叠发请求。工作流失败只更新工作流警告和列表，不占用通用诊断错误；工作流控制区显示警告与加载状态。

## Core 超时边界

`WorkflowList` 保留 Owner 授权与现有 `query`、200 条上限，在调用 `WorkflowRuntime.List` 时派生 5 秒 deadline。Temporal 适配器沿用传入 context，超时由现有 `workflow_runtime_unavailable` 错误路径返回。审计仍使用原请求 context，不新增缓存或替代数据源。

## 验证

先建立能检出“诊断定时器调用工作流列表”和“工作流列表调用没有 deadline”的快速回归信号，再改代码。分别覆盖非工作流区、进入工作流区、手动刷新、进行中重复触发和失败提示。Go 测试用阻塞的工作流运行时验证 deadline，不依赖真实 Temporal 服务。最后运行项目现有前端与 Go 门禁。
