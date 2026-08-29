# 实施计划：白纸人格涌现与自我模型 v2

1. 固化领域词汇、`initializationMode` 与顶层身份字段的 `llm_defined`/`blank_slate` 初始化契约，以及所有人格共享的 blueprint/affect/memory/self-model/agency 边界。
2. 盘点现有事件、记忆、affect/drive、关系、主动消息和调试数据，绘制统一输入/输出矩阵。
3. 设计 appraisal、memory consolidation、self-model、agency 四组版本化 LLM 候选 schema 与 revision/CAS 规则；self-model 的长期特征使用结构化特征主张，摘要和人格坐标均从主张派生。
4. 先实现 interaction facts 到 LLM appraisal 再到既有 affect reducer 的最小闭环，再实现由 LLM 驱动、证据约束的记忆沉淀；禁止关键词/规则替代路径。
5. 实现 LLM 驱动的 self-model 候选、冲突和自动沉淀流程，并将摘要接入 prompt/debug；提供用户查看、修改、删除和回滚入口，以 user-authored revision 保留审计链。
6. 实现 LLM 驱动的 agency intention 资格输入、冻结、投递和拒绝/延迟状态；允许 LLM 自然生成解释，同时持久化 redacted 原因摘要、证据引用和门控结果供专用入口查看。服务端只做安全/权限/幂等/租约治理。
7. 为两种初始化模式补充初始化差异、长对话、重启恢复、冲突、回滚和安全边界测试，并验证共享运行时协议不分叉。
8. 完成全链路回放、SSE/浏览器回归、数据迁移检查和性能预算评估。

## Validation

- `npm test`
- `npm run typecheck`
- `npm run build`
- `node --check server/index.js`
- 事件回放与 SQLite 重启恢复 smoke test
- 白纸实例与预定义人格的对照 chat/proactive/debug 测试
