# 设计：沿既有请求链路完成预算闭环

## 边界和数据流

保留 `BuildContextProjectionFor` 的权威状态读取和 `assembleProjectionPromptForSurface` 的 Slot 选择。先用面向 Agent 的精简投影构造 Prompt；摘要、原文、检索记忆使用现有 source refs 和覆盖关系选择。Eino v0.7.37 的 `ChatModelAgent`/Runner 继续拥有 Tool Loop。每次 Generate/Stream 前，现有 `runtimeContextRefresh.prepare` 在复制的消息上替换 System/Runtime，再用同一预算策略核验完整消息、Schema 与输出约束；Provider 边界最后复核，不新造并行裁剪器。

## 预算合同

模型角色配置提供窗口、最大输入、输出预留和安全余量。条件是 `input + effective_generation_reserve + safety_margin <= context_window`。普通 16K 配置以 11776 硬限和约 8000 首轮目标回归。有匹配 tokenizer/模板的本地计数走精确模式；不能证明时标记 `estimated`、加保守余量，边界拒绝。实发 Eino `MaxCompletionTokens` 与同一请求预留匹配；显式冲突报错。成功请求的 Provider usage 与本地计数并列记录。

## 收敛与真实性

依次消除重复协议/Slot、精简领域投影、检索去重、工具结果分页、摘要承接较早完整交互。保护当前用户输入、关键身份/权限、当前有效事实和正在消费的工具单元。工具视图只改变模型可见投影，不改变原始收据；分页引用必须能经现有授权 Tool 读取。摘要失败保留原文；必需内容仍放不下时返回分类预算错误。

## 兼容和风险

现有未提交修改包括引用编码、时间格式和人格协议；逐处对照并保留。保留外部接口与独立 Tool 能力。不把估算说成 mlx-serve 精确模板计数，也不把 token 下降说成 GPU 内存保证。回滚以单个新增改动为界，不还原用户既有修改。
