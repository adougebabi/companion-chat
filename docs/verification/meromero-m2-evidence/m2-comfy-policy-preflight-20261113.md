# M2 提供的 ComfyUI 工作流：媒体策略预检

受检源码为干净导出的 `2d202e8f475358159527f9dba7b8c836448e423e`。
使用用户提供的工作流的内存副本，将唯一的渲染节点 `prompt` 替换成
`{{prompt}}`；未更改原文件。工作流直接在真实 ComfyUI 出图的独立证据见
`m2-comfy-direct-smoke-20261113.md`。

`Krea2LoRALoader` 在 `lora_1`、`lora_2` 和 `lora_3` 输入中包含配置；
现有 `validateMediaWorkflowPolicy` 对 LoRA 节点只读取 `lora_name`。
隔离的无人物风景场景使用既有 `TestMediaNormalizationProbe`、受控模型和
无实际生图模式检查两种配置：

1. 没有声明风格 LoRA：返回 `media_workflow_unclassified_lora_forbidden`。
2. 为定位字段兼容性，仅在测试副本中把节点非空 LoRA 字符串全部假设性列入
   `style_loras`：仍返回同一错误。此假设不代表这些 LoRA 的真实分类。

两次测试均执行至现有策略检查，退出码均为 1，未 SKIP，未访问数据库、
ComfyUI 或对象存储，未产生新图片。初次尝试因 Go 默认构建缓存受沙箱
限制，在测试执行前失败；更换为临时构建缓存后获得上述结果。命令及完整
脱敏输出见 `m2-comfy-policy-preflight-20261113.log`。

此结果说明上述隔离配置中的正式媒体策略无法通过该多输入 LoRA 节点，
不代表已检查用户的生产配置，也不代表其 LoRA 均为风格或身份。不得以
空字符串分类、关闭身份保护或其他方式绕过当前限制。该兼容问题登记在
AEL `image-krea2-multi-lora-policy-compatibility`，本次未修复。正式私聊到真实
渲染和附件交付的端到端验收依然未执行；M2 未完成。
