# 技术设计

## 权威与边界

复用当前表和写入服务。Foundation 仅为稳定人格与一次性初始化来源；`fluctlight_personality_runtime` 与已生效 overlay 确定激活人格，`working_personas` 是唯一生效人格文本；Effective Life 持有身体、库存、穿着和 profile habits；schedule/life event 持有生活位置和活动；goals/intentions/activity/outcomes 分别持有意愿、执行和结果；Raw History 与 Memory 只描述历史/认识，不能回填当前事实。

`BuildContextProjectionFor` 形成统一读取视图，暴露各权威版本和来源。相关领域写入、来源语义修订或删除在事务内推进 `fluctlight_context_generations`；读前/读后核对同一 Fluctlight 代际，Tool 收据记录代际推进，结算锁代际行再校验。缺少当前态基线是显式未初始化/错误，不回退 Foundation 文本。成功虚拟动作和确认事件使用现有事务与幂等收据生效。

## Context Projection

保留原始 Eino 消息序列及诊断。任务级投影组件只读选择、格式化、预算和来源跟踪。普通对话、WakeUp、Reflection、初始化及相关正式任务走一致的预算/诊断边界，按 surface 给不同部分。System 指令来自可信配置；状态、记忆、历史及反思进入数据层并有来源标识。原生工具轮次由 Eino 保持完整，压缩只针对已消费且证明可替代的查询结果；不改变工具协议或回执。每次后续请求在已提交工具变更后读取新状态/更新投影，并保留 Eino 工具结果。

预算使用现有 model role 总上限与 Prompt/Working Memory 分区。必需的人格、当前输入、待消费回执失败则诊断性报错；可选片段按来源/重要性裁剪。记录版本、估算 token、选中/丢弃原因和来源映射。估算值不得宣称精确 tokenizer 结果。原生 Tool 适配器把该次送模投影生成的 Core 冻结上下文传入独立 `ExecuteTool`，以便修订 Tool 解析模型实际看到的 opaque 引用；宿主校验调用与快照身份，独立直接调用仍自行读取授权作用域。

## Memory Provenance 与三层

Raw History 继续为原始依据。Episode 采用可重建、来源绑定的经历片段；Long-term 仍由现有 durable Memory authority 管理；Resident 为短小、可重建、有代际/来源的常驻投影，包含未完成事项与关键关系，不复制当前状态或 Working Persona。Active Memory 继续表示短期活动事实，不与 Resident 混同。已有 `memory.recall` 查询扩展有效层，不新增同义工具。

来源关系保存多条原始或派生引用及版本/指纹；发生时间与记录时间独立。派生发布校验来源仍有效和作用域正确，阻止循环和失效摘要回流；纠正/删除事务中立即使依赖产物不可见，再按已有任务重建。后台结果冻结来源指纹，过期时拒绝发布。历史来源无法验证则为 legacy/unknown，不生成假引用。索引检索始终以数据库有效状态过滤。

## 兼容、迁移与回滚

新增迁移顺接 `0036_effective_life`，只做现有库的增量列/表/索引和可审计 backfill；重复执行不改变记录。数据迁移与服务代码同步部署后，正式调用方仅使用新投影/记忆读路径。发生异常时错误显式上报，不悄悄回退旧来源。回滚使用迁移前数据库备份和对应代码版本，避免向旧代码暴露不兼容的写入；不对生产数据执行迁移验收。

## 风险与验证

最大风险是不同表的快照时刻、工具轮次中的刷新、纠正与后台重建竞态、legacy 来源无法恢复、模型实际行为受配置限制。对这些分别设并发/版本测试、原生 Eino wire 测试、迁移与依赖失效测试、真实多轮验证。报告明确区分环境 BLOCKED 与通过。
