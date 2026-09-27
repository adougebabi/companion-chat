# 执行计划

1. 复核媒体 Tool schema、frozen concept、最终 Provider wire和质量重试；补四类视角、未知 appearance/零压力 drive、双层 JSON 的失败 fixture。
2. 扩 bounded capture input并保持旧 intent兼容；恢复 Media Prompt Agent framing-first、镜头/设备物理规则，去除“不是普通自拍”的冲突默认。
3. 实施 media-specific context projector及 TOON-capable格式；只压缩 Provider-facing copy，不修改持久冻结事实。
4. 用统一多模态 text-part formatter处理 media quality/retry与 Visual Identity同类入口，断言 image part逐字节不变。
5. 运行 Go prompt/core测试、受控媒体模型 fixture、质量重试/失败测试；核对 token/字符对比，更新 media prompt contract与相关 provider 文档。

回滚点：capture schema保持 optional/additive，格式切换与投影减重分开验证；旧持久 media intents可按原冻结内容重放。

验证命令：`go -C apps/core-go test ./internal/ai/prompt ./internal/core`；受控多模态Provider fixture核对最终 wire text/image parts及质量重试。
