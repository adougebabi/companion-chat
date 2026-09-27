# 实施与验证记录（2026-09-28）

## 已实现

- `media.image.generate` 接受可选闭合 `capture` 对象（mode、framing、angle、camera、mirror、device_visibility），Prepare冻结并在重放时核对。持久化concept完整保留，旧仅有intent的任务可继续读取。
- Media Prompt Agent恢复取景先行、手持/镜前/第一人称/operator POV/外部摄影关系及未指定时的保守自摄默认；去掉固定年龄、体型、写真审美。
- Provider-facing媒体投影用已知body field `{field,value}`同构行、当前穿着、mood和有效drive标签，去除未知身体值、零意义drive模板、revision/时间/存储ID。当前appearance存在时，视觉身份仍保留结构化稳定面部/体型语义，历史头发/衣着叙述被过滤。
- 质量验收与retry的 `frozen_media_concept` 是结构对象。统一格式器只改多模态 `type=text`块，保留image_url及原生ToolResult JSON；两个Visual Identity多模态入口也使用同一路径。媒体同构数组启用TOON。
- `.trellis/spec/backend/media-prompt-contract.md`已记录新闭合输入、格式、错误与回归合同。

## 已验证

- 无数据库 `go -C apps/core-go test ./internal/core ./internal/ai/prompt -count=1`、`go vet`、`git diff --check`通过。
- 隔离PostgreSQL真实两轮 `TestFormalToolEinoAdapterE2E/media.image.generate`证实capture保留在`media_intents.prompt`；媒体状态/质量、当前外貌旧新权威、显式四种视角、TOON body rows、Visual Identity image block测试通过。
- 独立只读复核发现初版投影丢失稳定视觉身份、格式器改写ToolResult；均已修正并补回归。

## 联合验收限制

- 真实模型是否准确呈现四种拍摄物理关系，还需用户允许的真实Provider/图片输出验证；当前测试证明冻结字段、提示词契约、传输和质量输入一致，不声称已看过最终生成图像。
- 父任务最终统一跑完整数据库套件与媒体Prompt实际字符/token对比；本子任务尚未单独提交或归档。
