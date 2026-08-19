# 人格媒体意图、模板化与结果验收：技术设计

## 目标与边界

本设计把一项媒体交付拆为三个职责明确、可审计的阶段：

```text
人格模型在能力调用时决定 A
  → 持久化不可变的事实快照与 A 概念
  → B 将概念模板化为 provider prompt
  → provider 生成候选媒体
  → C 用同一人格模型验收候选媒体
  → 交付、一次受控重生成、拦截，或验收不可用时降级交付
```

- **A 是人格的创作/叙事决策。** 它决定要拍什么，不是 provider prompt。
- **B 是受约束的写作阶段。** 它只把 A 和服务器事实写入固定八段模板，不能重写故事。
- **C 是事实一致性门禁。** 它只判断候选结果是否符合 A，不做审美评分，也不重写 A。
- 服务器只拥有授权、权威事实、schema 校验、持久化、状态转换、固定模板渲染、provider 调用与安全诊断；服务器不以自然语言规则决定视觉语义。

## 术语与所有权

| 名称 | 拥有者 | 可变性 | 说明 |
| --- | --- | --- | --- |
| `PersonaMediaConceptV1` | AI 人格 | job 创建后不可变 | A 阶段；场景、动作、情绪、真人主体、非人对象、镜头与构图。 |
| `MediaConceptEnvelopeV1` | 服务器 | job 创建后不可变 | 身份、关系、当前投影状态、权威事件、临时外观、来源、kind/count 等事实。 |
| `MediaCapabilityCallV2` | AI 人格 → 服务器 | 仅在创建 job 前存在 | 人格识别出媒体意图时发出的结构化调用参数。 |
| `MediaPromptTemplateV1` | B 阶段 | 每个 provider 尝试可重建 | 固定八段模板；由 A、事实和可选 C 违例生成。 |
| `MediaAcceptanceV1` | C 阶段 | 每个候选媒体一份 | `pass` / `retry` / `reject` / `skipped` 的结构化验收记录。 |

## A：人格能力调用契约

当前 `<media-intent>` 是项目既有的文本承载方式；本次不引入浏览器侧 provider 调用，也不要求另起工具框架。它升级为人格模型的“图片/视频能力调用参数”，并在解析后立即校验。

```json
{
  "schemaVersion": 2,
  "kind": "image",
  "count": 1,
  "request": "可选的交付说明",
  "personaMediaConcept": {
    "schemaVersion": 1,
    "mediaKind": "image",
    "scene": "商场橱窗前",
    "action": "与闺蜜自然合影",
    "mood": "轻松愉快",
    "narrative": "…",
    "humanSubjects": [{"label": "人格本人", "role": "主体", "inFrame": true}],
    "nonHumanObjects": [{"label": "橱窗", "kind": "environment", "inFrame": true}],
    "capture": {
      "mode": "external_capture",
      "operator": "画外朋友",
      "deviceVisibility": "out_of_frame",
      "framingIntent": "自然竖屏中景"
    },
    "compositionIntent": "…"
  },
  "currentEvent": {"id": "可选的事件引用/摘要", "…": "…"},
  "temporaryAppearance": {"hair": "…", "outfit": "…"}
}
```

### 解析和事实权威

1. 聊天输出解析器和活动决策解析器保留该调用对象，而不是由 `normalizeMediaRequest()` 丢弃额外字段。
2. `personaMediaConcept` 通过现有 `normalizePersonaMediaConcept()` 校验，且 `mediaKind` 必须与 `kind` 相同。
3. `currentEvent` 和 `temporaryAppearance` 必须作为调用参数存在于可审计的调用记录中；服务器以 `mediaConceptEnvelopeFor()` 在 job 创建时读取的事件/状态快照为权威值。
4. 模型不能用调用参数覆盖事件 ID、人格身份、当前状态、关系或 provider；服务端构造的 envelope 仍是唯一的下游事实来源。
5. 聊天没有显式 life event 时，调用对象允许 `currentEvent: null`；它仍带有服务器投影的当前状态与临时外观。活动路径则使用创建该活动的权威事件。
6. `count > 1` 仍产生独立占位消息/job，但每一个 job 保存同一份已经校验的 A 概念和同一批事实快照；本任务不设计每张图片重新创作一个概念的变体策略。

### 三类生产者

| 生产者 | A 从哪里来 | 事实如何冻结 |
| --- | --- | --- |
| 聊天 | 流式人格输出的 `MediaCapabilityCallV2` | 创建消息/job 时由 `mediaConceptEnvelopeFor()` 读取当前投影状态；事件可为空。 |
| 活动 | `activity_decision` 人格输出内的媒体调用对象 | 决策输入必须含事件的外观变化；创建活动媒体 job 时由已存储 event 构造 envelope。 |
| 调试 | 显式提交同一结构化调用对象（用于 fixture/诊断） | 仍由服务器构造 envelope，不能注入身份、provider 或未验证事实。 |

`request` 继续只是人与模型可读的交付说明；它不是视觉推理兜底，也不得成为 provider prompt 的优先前缀。

## durable job 与迁移

新 source media job 的 payload 采用加法字段：

```json
{
  "envelope": {"schemaVersion": 1, "…": "authoritative facts"},
  "personaMediaConcept": {"schemaVersion": 1, "…": "frozen A"},
  "capabilityCall": {"schemaVersion": 2, "…": "auditable normalized call"},
  "kind": "image",
  "provider": "comfyui",
  "trigger": "model_capability_contract",
  "qualityRetryCount": 0,
  "maxQualityRetries": 1
}
```

- 不改变已应用的 SQLite migration；新字段放在既有 `payload_json` / `result_json` 中。
- job 创建事务仍原子地写入 chat 占位消息或 activity 的 `media_status` 和 source job。
- worker 进入 B 前重新解析 payload 中的 envelope 和 `personaMediaConcept`，只做 schema 检查，不调用 `generatePersonaMediaConcept()`。
- 缺少 `personaMediaConcept` 的旧 payload 是终止性迁移失败：不调用概念 LLM、不调用 B、不调用 provider；将原媒体目标标记失败，保留正文，并记录有界原因 `missing_frozen_media_concept` / 用户可见的重新生成提示。
- `result_json.personaConcept` 保留为冻结概念的副本，保持检查器兼容；其来源注明为 `capability_call` 而非 worker。

## B：固定模板化

`fillMediaPromptTemplate()` 继续是唯一的 Prompt Master 调用。输入固定为：

```text
authoritative envelope
+ frozen PersonaMediaConceptV1
+ optional prior C retry violations (only on the one quality retry)
```

要求：

- 模板输出仍是 `MediaPromptTemplateV1` 的固定八段，`renderMediaPromptTemplate()` 只按既有顺序连接段落。
- B 不获得任何“重新选择主体、场景、动作、事件、外观或镜头”的授权。C 的 `retryGuidance` 只陈述违反的冻结事实，例如“临时银灰短发缺失”或“错误新增第三个人”，不能给出新故事。
- 普通 B 故障沿用 durable provider/source-job 的现有重试语义；绝不落入服务器推导的视觉 fallback prompt。

## C：视觉一致性验收

### 输入和输出

C 复用 `lmCompletion()` 的当前模型/URL/API key；新增受限的多模态 message 内容构建，而不新增设置字段。它接收：

```text
frozen envelope + frozen PersonaMediaConceptV1
+ image candidate, or a bounded set of video keyframes
```

它必须只返回：

```json
{
  "schemaVersion": 1,
  "verdict": "pass | retry | reject",
  "violations": [
    {"code": "temporary_appearance_missing", "severity": "hard", "detail": "…"}
  ],
  "observedFacts": {
    "personaPresent": true,
    "sceneMatches": true,
    "captureMatches": true
  },
  "retryGuidance": "仅强调已冻结且未满足的事实。"
}
```

允许的硬检查：人格/临时外观连续性、地点/事件/动作、明确人类主体与非人对象、入镜与镜头关系、空白/损坏/明显畸形结果、安全问题。禁止审美、艺术风格好坏或“是否漂亮”评分。

### 候选字节与视频帧

- provider 需提供仅供服务端使用的、受限的 asset-byte 读取接口；C 不通过浏览器 route 回环，也不把 provider URL、路径或凭据发送到模型。
- 图片在 MIME、尺寸/总字节上限检查后转换为 data URL（或经当前兼容端点支持的等价私有输入）发送给模型。
- 视频先用 `ffprobe` 得到时长，再用 `ffmpeg` 在固定、有界的时间点提取少量 JPEG 关键帧。命令使用参数数组，限制帧数、像素、总字节、路径、超时和 stderr 长度；任何不可用/超限都是 C 基础设施不可用而非内容 `reject`。
- 不复用用户附件的 data URL 存储；该路径没有合适的服务端 MIME/大小/文件边界，不属于 C 的候选资产输入。

### C 状态机

```text
provider candidate
  └─ C invocation
       ├─ pass     → 持久化资产并将目标设为 ready
       ├─ skipped  → 记录安全诊断，持久化资产并设为 ready
       ├─ retry    → 若 qualityRetryCount = 0：记录验收，入队一个继任 source job
       │              （A/envelope 不变；B 接收违例；qualityRetryCount = 1）
       └─ reject   → 不持久化/关联候选资产；媒体目标 terminal failed/rejected，正文保留
```

- `retry` 只可触发一次新的 provider 生成。继任 job 使用同一 target（消息或活动）和同一 provider；不创建第二个可见占位项。
- 第二次候选的 `retry` 等价于终止性 `reject`。C 的明确 `reject` 也立即终止。
- 对于异步 provider，C 在 poll 成功、`mediaAssets()` / `updateMediaTarget(... ready ...)` 之前运行；对于同步 provider，C 在 submit 返回 files 后、同一可见状态转换之前运行。
- `MediaAcceptanceV1` 写入 source/poll 结果与 debug projection，至少含 verdict、有限诊断、违例、检查时间、输入种类/关键帧数及 `qualityRetryCount`。

### 用户已确认的 fail-open 边界

以下仅属于验收基础设施故障，均写入 `acceptance.status = "skipped"` 并交付已成功生成的媒体：模型调用失败/超时、模型不支持或拒绝视觉内容、模型 JSON 非法/无法判定、候选字节或视频关键帧无法安全读取/提取、受限资源检查失败。

模型给出明确的概念矛盾、明显生成失败或安全问题时，不得以 `skipped` 放行。

## 调试与用户可见状态

- `debugContextFor()` 对每项媒体保留并限量展示：规范化的 capability call、authoritative envelope、frozen persona concept、B template、final provider prompt、每次 C 验收和 provider/重试状态。
- 所有 debug 值经既有 redaction/bounding；不得向浏览器返回 API key、provider 路径、原始可执行命令、完整本地文件路径或候选 data URL。
- 聊天占位和活动状态复用现有 `queued` / `processing` / `ready` / `failed` 表现。质量拦截或旧 job 失败必须给出安全、可理解的重新生成说明，保留已有正文；不向普通 UI 暴露内部 prompt/违例 JSON。

## 回滚与兼容

- 代码回滚只影响未来 job；已经写入的 payload 是加法 JSON，因此不会要求数据库回滚。
- 新 worker 识别旧 job 并明确失败，确保部署后不回到 worker 临时构思画面。
- provider、media asset URL、ComfyUI prompt 替换、h3 受限路径和现有 polling 协议保持兼容；C 只改变“何时把候选媒体关联为 ready”。

