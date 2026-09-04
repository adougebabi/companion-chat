# 生图后 LLM 质量验收与提示词修订工作流

## Goal

在媒体 Provider 已经生成候选图片后，先让具备视觉输入能力的 LLM 对候选结果进行事实一致性验收；模型明确判定合格时发送，验收基础设施不可用时按已确认的 fail-open 策略直接发送并记录 `skipped` 诊断。若候选图片明确不符合冻结的媒体意图，则让下一次提示词生成只针对已发现的违例进行修订，再有限次数地重生成，避免把错误图片直接交付或无限重试。

本任务的 MVP 处理图片生成；视频关键帧验收沿用历史设计但暂不作为本次实现的必需交付，除非后续确认扩大范围。

## 已确认的仓库事实

- 当前 Go Worker 在 `apps/core-go/internal/core/media.go:29-155` 读取 `media_intents`，调用 `media_prompt` 生成 provider prompt，提交/轮询 ComfyUI，下载候选字节，写入 `media_assets(status='ready')`，发布媒体并完成 intent；候选在发布前没有 LLM 质量闸门。
- `apps/core-go/internal/core/media.go:64` 是当前 `media_prompt` 调用边界；`apps/core-go/internal/core/media.go:144-155` 是候选写入 ready、发布和完成的关键交付边界。
- 当前 PostgreSQL 表 `media_intents`、`media_assets`、`media_references` 已提供 durable intent、provider job、资产和目标关联；不应为质量验收引入浏览器到 Provider 的旁路。
- `.trellis/spec/backend/media-prompt-contract.md` 与归档任务 `08-19-persona-media-intent-pipeline` 已记录 A/B/C 分层、`MediaAcceptanceV1`、一次质量重试、同一目标复用和旧作业兼容等设计。本任务负责把其中 C 阶段接入当前 Go 媒体执行路径，并以当前 Go 代码为准校正历史 Node 示例。
- `.trellis/spec/guides/cross-layer-thinking-guide.md` 要求媒体候选经过 C-stage visual acceptance；C 只能由模型判断事实一致性，服务端不能用关键词或启发式替代。

## Requirements

### R1. 交付前质量闸门

Provider 成功得到候选图片后，必须先执行 C 阶段 LLM 验收；在 C 返回允许交付的结论前，不得创建/关联 `ready` 资产，不得替换消息或动态占位，不得向用户发送图片。Provider 成功不等于媒体交付成功。

### R2. 验收输入与输出

C 阶段接收同一次媒体任务冻结的权威事实、人格媒体概念、最终 provider prompt 和候选图片内容。图片通过服务端受限的 MIME、尺寸、总字节检查后，以受限的视觉输入传给 LLM；不得调用公开浏览器媒体路由回环，也不得把 Provider 凭据、内部 URL 或本地路径发送给模型。

C 只返回严格结构化结果：

```json
{
  "schemaVersion": 1,
  "verdict": "pass | retry | reject",
  "violations": [{"code": "capture_mismatch", "severity": "hard", "detail": "…"}],
  "observedFacts": {"subjectMatches": true, "framingMatches": false, "captureMatches": false},
  "retryGuidance": "只描述冻结事实中未满足的部分"
}
```

`retryGuidance` 只能指出冻结概念/事实未被满足的内容，不得创作新人物、场景、动作或拍摄故事。

### R3. 验收标准

C 只检查可验证的硬一致性：真人主体/非人对象、身份和临时外观、地点/事件/动作、前摄/后摄/镜子/第三方拍摄关系、构图范围、明显空白/损坏/畸形结果和安全问题。不得因审美、艺术风格、漂亮程度、主观偏好或“像不像大片”而拒绝图片。

### R4. 不合格后的提示词修订与重生成

首次 `retry` 时：

- 保持 A 阶段媒体概念、权威上下文、目标消息/动态、Provider 和原始占位不变；
- 将 C 的结构化违例作为受限输入交给 `media_prompt`，只修订导致违例的提示词表达；
- 由同一个 MediaWorkflow 继续一次确定性、可幂等的质量重试执行，不创建第二个可见占位项；
- 第二次候选仍返回 `retry` 时，按终止性 `reject` 处理。

质量重试最多 1 次，且必须与 Provider 传输重试、轮询重试分开计数。

### R5. 合格、拒绝与失败状态

- `pass`：持久化候选资产、创建/关联引用、替换精确的消息/动态目标并完成媒体 intent。
- `skipped`：仅当 C 阶段基础设施不可用（模型调用失败/超时、不支持视觉输入、非法 JSON、无法安全读取候选图片等）时使用；记录有界诊断后，按 fail-open 直接持久化、关联并发送 Provider 成功的候选图片。`skipped` 不能用于掩盖模型明确发现的内容违例。
- `reject` 或质量重试耗尽：不持久化或关联该候选资产；仅将媒体目标标记为失败/拦截，保留原聊天文本或动态正文，并给出安全、可理解的失败状态。
- 任何结论都必须写入有界、脱敏的媒体诊断，能够区分 Provider 失败、C 验收结论和质量重试。

### R6. 验收逻辑必须由 LLM 执行

人物是否存在、镜头是否符合、设备是否入镜、场景和动作是否匹配等视觉判断必须由 C 阶段 LLM 完成。Go Worker 只负责候选读取边界、调用模型、校验结构化结果、状态转换和幂等；不得添加关键词、正则、图像 OCR/颜色启发式或服务端“自动判定合格”规则。

### R7. 当前 MVP 范围

本任务先实现图片候选的 C 阶段验收和一次质量重试。视频关键帧抽取、视频 C 验收、图生图参考图和头像一致性不作为本次完成条件，避免在图片闸门尚未稳定时扩大交付面。

## Acceptance Criteria

- [ ] Provider 生成成功后，图片在 C 阶段返回允许交付结论前不会进入 `ready`、不会创建媒体引用、不会替换消息/动态目标。
- [ ] C 调用接收冻结的媒体概念/权威事实、最终 provider prompt 和受限候选图片输入，不经过浏览器路由，不泄露凭据、Provider URL 或本地路径。
- [ ] C 只接受严格结构化的 `pass`、`retry`、`reject` 结果；非法 JSON、缺字段、越界诊断和任意自由文本不会被当成合格。
- [ ] C 的检查范围仅包含身份/外观、主体与非人对象、场景/动作、镜头/构图、明显生成失败和安全问题，不包含主观审美评分。
- [ ] 首次 `retry` 只触发一次确定性的质量重试执行，复用同一目标、A 概念、权威事实和 Provider，不创建第二个可见占位项。
- [ ] 质量重试把结构化违例传回 `media_prompt`，只调整提示词表达，不重新创作人物、场景、动作或拍摄关系。
- [ ] 第二次候选仍为 `retry` 或任意明确 `reject` 时，不关联候选资产；媒体目标失败且原正文保留。
- [ ] `pass` 才会持久化/关联 ready 资产并发布媒体；Provider 成功但 C 未通过时不会提前发送。
- [ ] Provider 传输重试与 C 质量重试分别计数，重复 poll、Worker 重启和 stale lease 不会绕过 C 或重复交付。
- [ ] C 结果、违例、重试次数和失败原因进入 persona-scoped、bounded、脱敏诊断；普通 UI 不暴露内部 prompt 或验收 JSON。
- [ ] 回归测试覆盖单人/多人主体、构图与前后摄/镜子/第三方关系、临时外观遗漏、明显损坏图片、`pass`、一次 `retry`、`reject` 和重试耗尽。

## Out of Scope

- 以主观审美、艺术风格或“是否漂亮”作为自动拦截条件。
- 无限自动重试或在重试中改写冻结的人格媒体概念。
- 视频关键帧验收、图生图参考图、头像一致性和新的视觉模型配置。

## Notes

- 已确认 C 阶段基础设施故障采用 fail-open：记录 `skipped` 后直接发送 Provider 成功的图片。质量重试最多 1 次；这是跨层复杂任务，需要先完成 `design.md` 和 `implement.md`，再进入实现。
