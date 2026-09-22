# Eino API 核验

- 当前锁定 `github.com/cloudwego/eino v0.7.37`，不升级依赖。
- 已执行 Context7 library Eino 查询及 docs /cloudwego/eino 查询，均退出 0。查询针对 native tool-result loop、stream、MaxIterations。当前文档索引没有给出 v0.7.37 专属版本，示例只作辅助，具体 API 以本机模块源码为准。
- 源码 `/Users/vinson/go/pkg/mod/github.com/cloudwego/eino@v0.7.37/adk/chatmodel.go:210-247`：ChatModelAgentConfig 有 Model、ToolsConfig、GenModelInput、Exit、MaxIterations、Middlewares；MaxIterations 注释定义默认 20，超过为 error，非正常完成。
- `adk/runner.go:40-83`：RunnerConfig.EnableStreaming、Runner.Run 输入消息与每次 run 状态。
- `adk/interface.go:111-126`：MessageVariant.GetMessage 对流式调用 ConcatMessageStream，直接 GetMessage 会聚合；生产增量输出需消费/安全复制 message stream，不能把聚合后回放当实际 streaming 证明。
- 当前应用 `internal/ai/agent/loop.go:146-151` 自己限制 <=2；`internal/core/eino_model_runtime.go:527-532,551-572` 启用 deferred-stop 和 action-only 超限成功；这不是框架要求。
- 官方参考：https://github.com/cloudwego/eino/blob/v0.7.37/adk/chatmodel.go 。不复制修改框架源码，不依照当前 main 示例直接假定锁定版本签名。
