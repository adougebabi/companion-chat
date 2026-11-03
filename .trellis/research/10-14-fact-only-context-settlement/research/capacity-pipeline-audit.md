# 内部容量传递核验（主线程汇总探子已核实出处）

index: context_reference.go 的 add/Validate 原先分别拒绝128条、32KiB单快照、128KiB索引；add在重复检测前判断数量。intelligence.go/provider_context.go/reflection_runtime_v2.go另有按剩余ref数截断Active Memory，均需移除；provider_ref_codec.go没有容量上限，别名碰撞/unknown/作用域保留。

快照：core/capability_core.go 与 capability/capability.go各自的 Snapshot、BoundedSnapshotValue、NewSnapshotContextResolver、prepared envelope encode/decode/data存在128KiB丢弃/拒绝；core/tool_execution.go冻结快照复制随后也会报limit。CapabilityInvocation.Validate调用canonical prepared decoder，必须同步两份实现。保留单独64KiB模型Arguments/schema门禁，此次只处理服务端权威上下文与准备数据。

相邻引用：cognition.go resultingStateReferences有32KiB状态快照拒绝；action_outcome.go Validate把contextReferences总字节并入payload限制。仅去引用容量条件，保持其余业务合同/模型字段检查。

诊断：App.recordDiagnosticEvent在provider前可用且best effort；DB不可用现有通用fallback不带容量数据，因此本次额外始终输出安全metadata到slog，再尝试Owner事件存储。阶段包括context_projection/prompt_assembly/reflection_context，包含引用数、总字节、分类型、最大快照及阈值，不包含快照正文。

三个探子未修改代码/运行测试/访问DB网络；代码与最终验证由主线程负责。单独诊断核验报告见 capacity-diagnostics-audit.md。
