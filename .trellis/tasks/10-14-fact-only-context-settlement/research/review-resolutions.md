# 独立核验发现的处理

- P1 Actor 键形状：已沿 buildActorProjection 的 actor_id 修正关系与 Goal target 筛选；fixture v2 也改为正式形状。TestFactOnlyProductionActorShapeKeepsRecipientRelationship 验证当前收件人关系保留、其他收件人排除。
- P2 purpose 跨版本抑制：publication 新增仅 Core 可设置的 FactOnlyTopicSuppression；同 topic/window/inbound 下仅借现有消息 reply_mode 元数据桥接旧 Fact-only/fallback，用例包含正反顺序和 legacy 负例。P 用例已补但本机 SKIP，SQL/契约由主线程复核，不冒充数据库已执行。
- 同时修正 duplicate-suppressed Tool 回填：读取已存在真实消息 text，不能将新候选当已发送。
- 最终线缆门禁发现记忆 provenance 元数据超过原 512-token 分区，已移除候选上重复长说明，保留 epistemic/provenance/time/subject。原语义警告在协议中保留，实际最终输入断言重新通过。
- 独立研究/核验仅定位与报告，代码修正及最终测试均由主线程执行。原报告保留于 view-code-review.md / runtime-code-review.md。
