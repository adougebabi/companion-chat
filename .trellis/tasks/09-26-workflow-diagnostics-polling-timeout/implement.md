# 实施清单

1. 建立快速失败测试：诊断自动刷新不触发工作流列表；工作流进入/手动刷新与进行中去重；Core 列表查询有 deadline。
2. 拆分工作流列表加载与通用诊断轮询，在工作流区显示加载和失败状态。
3. 为 Core 工作流列表调用增加 5 秒超时，保留授权、查询与错误映射。
4. 运行针对性红绿测试、前端 typecheck/test/build、Go test/vet/build 和 diff 检查。
5. 核对本任务是否形成需要同步到 `.trellis/spec/` 的稳定约定；保留用户工作区其他改动，不默认提交或推送。

## 执行结果

- 红色反馈：Web 回归测试在原实现中捕获 `loadDiagnostics()` 的无条件工作流列表调用；隔离 PostgreSQL 测试在原实现中捕获缺少短 deadline。
- 修复后：Web 50/50、browser client 12/12、workspace typecheck/build、相关 Go Core/HTTP 测试、Go vet/build、隔离 PostgreSQL 阻塞运行时测试通过；后者验证约 5 秒返回。
- 独立复核发现清空诊断时可能重置进行中标记，已改为不触碰独立的 Temporal 列表状态；新增测试运行真实 store 加载方法的并发与失败分支。
- 本机没有完整运行中的 API、Worker 和 Temporal，无法用用户的实际服务测量超时前后延迟；未提交、未推送。
