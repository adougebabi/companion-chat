# 实施计划：白纸人格涌现与自我模型 v2

1. 固化领域词汇、白纸实例初始化契约和现有 blueprint/affect/memory/agency 的边界。
2. 盘点现有事件、记忆、affect/drive、关系、主动消息和调试数据，绘制统一输入/输出矩阵。
3. 设计 appraisal、memory consolidation、self-model、agency 四组版本化候选 schema 与 revision/CAS 规则。
4. 先实现 interaction facts 到 appraisal/affect 的最小闭环，再实现证据驱动的记忆沉淀。
5. 实现 self-model 的候选、冲突和用户纠正流程，并将摘要接入 prompt/debug。
6. 实现 agency intention 的资格门控、冻结、投递和拒绝/延迟状态。
7. 为白纸实例和预定义人格分别补充初始化、长对话、重启恢复、冲突、回滚和安全边界测试。
8. 完成全链路回放、SSE/浏览器回归、数据迁移检查和性能预算评估。

## Validation

- `npm test`
- `npm run typecheck`
- `npm run build`
- `node --check server/index.js`
- 事件回放与 SQLite 重启恢复 smoke test
- 白纸实例与预定义人格的对照 chat/proactive/debug 测试
