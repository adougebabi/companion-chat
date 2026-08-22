# 新建摇光强制 MTPLX 结构化解析

## Goal

将新建摇光实例的自然语言描述改为 MTPLX 驱动的严格结构化分析。规则/正则只能用于协议边界和 JSON 解析，不能再从用户描述中猜姓名、角色或人格字段。

## Confirmed Code Facts

- `/api/companion/interviews/analyze` 当前委托顺序是 analyzer port -> repository port；默认只有 repository。
- `interviewRepository.analyze()` 使用姓名正则、默认角色和 foundation 截断，并且不创建 ready interview session，导致前端可能直接走 persona create。
- MTPLX raw provider 支持 OpenAI-compatible `stream: false`，但现有 chat completion adapter 强制流式且只认识 `companion.turn.v1`，不能直接当 persona extraction schema。
- 历史自然语言实现已经定义了无原文落库、provider failure 502、用户确认后 activate 的产品边界，可作为兼容基线。

## Requirements

- 新增独立 `InterviewAnalyzerPort` / infrastructure adapter，调用 MTPLX 非流式 JSON completion，拥有版本化 prompt 和 bounded timeout。
- analyzer 输出严格白名单结构，至少包含 `answers`、`inferredFields` 和可生成 preview 的 persona blueprint 字段；拒绝未知字段、错误类型、过长字段和无效 JSON。
- runtime 默认注入 analyzer；`interviewService.analyze()` 在生产配置缺失时 fail closed（501/502），不能回退到 repository 规则分析。
- 成功分析创建 ready interview session，返回 `interviewId`、`source: 'llm'`、preview 和字段来源；description 不入库、不在响应中回显。
- 分析失败不新增 session/persona；前端保留原文以便重试，但不得显示一个伪造的规则分析结果。
- 激活时以用户确认后的 preview 为准，保留现有生命周期/blueprint 初始化和旧 interview rows 兼容。

## Acceptance Criteria

- [x] mock MTPLX 返回合法 JSON 时，默认 runtime 调用 provider 一次，返回 ready preview 和 interviewId；spy repository.analyze 不被调用。
- [x] provider error、timeout、空 body、fenced JSON 中的未知字段和非法 shape 全部返回 bounded 502，数据库 session/persona 计数不变。
- [x] 姓名没有显式写成“叫/名为”格式时，仍能由 LLM 正确返回；regex 已从 repository 分析路径移除。
- [x] preview 编辑后 activate 的 persona 数据与编辑值一致，历史 interview session 仍可读取和激活。
- [x] 增加 analyzer、runtime composition、route integration 回归测试；全量 `npm test`、typecheck、build 通过。

## Out of Scope

- 不修改聊天 turn 的 `companion.turn.v1` 协议和已有 marker 兼容层。
- 不在本子任务生成图片；图片作业由兄弟任务负责。
