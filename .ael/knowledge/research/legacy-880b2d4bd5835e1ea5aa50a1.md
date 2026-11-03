---
{"id":"legacy-880b2d4bd5835e1ea5aa50a1","title":"spec-loading.md","kind":"research","status":"historical","paths":[],"source":".trellis/research/10-03-yaoguang-goal-life-consistency/research/spec-loading.md#sha256=5009fd9eae8715695963a0c0aba2572bd43157e2d00009f9c754a5d04a590b70","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# 大型规范读取契约

以下两个真实规范超过 native context injection 单文件 32768 bytes，不能把注入后的截断文本视作完整规范：

- `.trellis/spec/backend/persona-layer-contract.md`：人格/Working Persona/Developing Self 的权威、证据与演进。
- `.trellis/spec/backend/fluctlight-provider-contract.md`：Provider capabilities、原生 Eino/ADK、正式 Agent/Tool、每次物理请求预算与失败语义。

实现前由主代理使用文件工具分段完整读取涉及的规范章节，并确认当前有效/被替代条款；规范属于判断地基，不能只依赖研究压缩。只读核验子代理按核验范围自行拉取原文件，不从截断注入推断否定结论。本研究入口替代超限 manifest 直接注入，仅改变加载方式，不取消规范。

若历史 capability architecture/README 的两轮/deferred 描述与当前 formal Tool runtime 或最新规范冲突，先核对最新正式实现与 09-22 原始方案，再在本任务 spec-sync 修复相矛盾的有效描述，禁止另建兼容双轨。

