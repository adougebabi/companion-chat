# 生图后 LLM 质量验收与提示词修订：技术设计

## 目标与边界

当前 Go 媒体 Activity 在 Provider 候选下载后直接写入 `media_assets(status='ready')` 并发布。本设计在同一 durable media workflow 内增加 C 阶段质量闸门，同时保留已有 ComfyUI、MinIO、Provider job ID、消息/动态投影和 Temporal 重试边界。

```text
已冻结的媒体意图/上下文
  → media_prompt 生成 B provider prompt
  → ComfyUI 生成候选并下载
  → C 使用视觉 LLM 验收候选
       ├─ pass    → 写入 ready asset → 关联目标 → 发布
       ├─ skipped → 记录基础设施诊断 → 写入 ready asset → 发布（fail-open）
       ├─ retry   → 保存违例 → 同一目标再次调用 media_prompt → 重新生成（最多一次）
       └─ reject  → 不写入/关联候选 → 仅失败媒体目标，保留正文
```

本次 MVP 只实现图片候选。视频关键帧、图生图参考图、头像一致性和新的视觉模型配置不在本次交付范围。

## 术语与所有权

| 名称 | 所有者 | 可变性 | 说明 |
| --- | --- | --- | --- |
| `media_intents.prompt` | Go Core | 创建后保留 | 已冻结的媒体概念/请求输入，不由 C 改写。 |
| `media_intents.provider_prompt` | Go Worker | 每次 B 尝试更新 | 实际发送给 ComfyUI 的最终 provider prompt；用于 C 输入和诊断。 |
| `media_intents.quality_retry_count` | Go Core | 单调递增，最多 1 | 只统计 C 触发的质量重试，不统计 Provider/Temporal 传输重试。 |
| `media_intents.quality_retry_guidance` | C → B | 仅在一次 retry 中使用 | 受限的冻结事实违例，不是新故事或新媒体概念。 |
| `media_intents.quality_verdict` | C/Go Core | 当前候选状态 | 空值、`pass`、`skipped`、`retry`、`reject`。 |
| `MediaQualityAcceptanceV1` | C LLM | 每个候选一次 | 严格结构化 verdict、违例、观察事实和 retry guidance。 |

## 数据与状态设计

### `media_intents` 加法字段

在初始建表和兼容 SQL 中增加以下字段，不删除或改写既有事实：

```text
provider_prompt text NOT NULL DEFAULT ''
quality_retry_count integer NOT NULL DEFAULT 0
quality_retry_guidance text NOT NULL DEFAULT ''
quality_verdict varchar(16) NOT NULL DEFAULT ''
quality_candidate_sha256 varchar(128)
quality_checked_at timestamptz
```

`provider_job_id` 仍表示当前正在轮询的 ComfyUI job。首次 C 返回 `retry` 时，事务将 `quality_retry_count` 置为 1、保存 guidance、记录当前候选摘要，并清空 `provider_job_id` 与 `quality_verdict`，使下一次执行从 B 重新开始。原始概念、消息/动态目标、Provider request ID 和 Workflow ID 不变。

`quality_candidate_sha256` 用于崩溃恢复：C 已返回 `pass`/`skipped` 但对象写入或 ready 事务尚未完成时，worker 重新下载同一 Provider job，若摘要相同则复用已持久化 verdict，不重复调用 C；摘要不同则重新验收。

### 状态转换

```text
pending + no provider_job_id
  → B 成功 + provider submit → running
running + provider candidate
  → C pass/skipped → quality_verdict persisted → asset ready/publish → completed
  → C retry (count=0) → quality_retry_count=1, provider_job_id=NULL, status=pending
  → C retry (count>=1) → status=failed, quality_verdict=reject
  → C reject → status=failed, quality_verdict=reject
```

C 基础设施错误（Provider 不可用、视觉输入不支持、超时、非法 JSON、候选 MIME/大小/读取不满足 C 输入边界）不进入 `failed` 或 `retry`，而是记录 `skipped` 后继续现有上传/发布路径；用户/Workflow 主动取消则保留取消语义，不得因 fail-open 继续发送。

## Provider 与模型契约

### B：现有 `media_prompt`

- 首次生成继续使用 `mediaPromptInstruction`。
- 质量 retry 时，`Provider.Text(ctx, "media_prompt", ...)` 的 user 内容改为有界结构化对象：冻结的原始媒体输入、上一次 provider prompt 和 C 的违例/guidance。
- B 只能修复被指出的表达问题；系统提示明确禁止新增/删除人物、场景、动作、镜头关系或把违例 guidance 改写成新故事。
- B 成功后先持久化 `provider_prompt`，再提交 ComfyUI；Provider request ID 仍由现有 media intent 复用/持久化。

### C：`media_quality_acceptance_response`

复用当前 `media_prompt` Provider role 和其模型/endpoint/API key，不新增设置项；使用 `Provider.StructuredWithSchema` 请求严格 JSON Schema。

系统提示要求：只做事实一致性检查，不评分审美；只返回 schema 允许的 JSON；`retryGuidance` 只能描述冻结概念/事实的缺失或冲突。

```json
{
  "schema_version": 1,
  "verdict": "pass | retry | reject",
  "violations": [
    {"code": "capture_mismatch", "severity": "hard", "detail": "bounded detail"}
  ],
  "observed_facts": {
    "subject_matches": true,
    "appearance_matches": true,
    "scene_matches": true,
    "capture_matches": false,
    "framing_matches": false
  },
  "retry_guidance": "bounded frozen-fact correction"
}
```

Go normalizer 只接受 `pass`、`retry`、`reject`；限制 schema version、违例数量/字段长度、severity 枚举和 guidance 长度。`retry` 没有可执行 guidance 时按明确不合格处理，不触发无意义的第二次生成。

### C 多模态输入

C user message 的 `content` 使用 OpenAI-compatible 多模态数组：一个有界文本块（冻结概念、上下文摘要、provider prompt）和一个 `image_url` data URL。候选图片在内存中完成 MIME allow-list、总字节上限和非空校验后再编码；不写入公开 URL，不经过 BFF 路由，不把 Provider 路径或凭据发送给模型。

Provider diagnostics 会记录同一 messages。更新现有 `redactDiagnostic`：`image_url.url` 中的 data URL、过长二进制字符串和候选内容必须替换为 `[REDACTED_IMAGE_DATA]`/摘要，不能把 base64 写入 `diagnostic_model_runs`。

## Media Worker 流程

将 `ProcessMediaIntent` 保持为唯一媒体 Activity，但把内部流程拆成可测试的阶段 helper：

1. 读取 intent 和质量状态；ready asset replay 继续只做幂等投影。
2. 若没有 `provider_job_id`，调用 B（必要时携带一次 retry guidance），持久化 `provider_prompt`，提交 ComfyUI 并立即持久化新 job ID。
3. 轮询现有 ComfyUI history 并下载候选；保持 heartbeat 和当前 Provider retry 语义。
4. 计算候选 SHA-256；若同一候选已经有 `pass`/`skipped` verdict，跳过 C，否则调用 C。
5. C `pass`/`skipped`：在同一状态边界写入 quality verdict，再上传对象、创建 ready asset；复用现有 `publishMediaAsset` 和 `markMediaIntentCompleted`。
6. C 首次 `retry`：记录验收诊断，在事务中保存 guidance、递增 quality count、清空当前 job/verdict、将 intent 置回 pending；不上传候选，不创建新可见消息。Activity 返回 `{status:"quality_retry"}`，同一个 Media workflow 看到该结果后再执行一次同一 intent 的 B→Provider→C 链路；Temporal history 与持久化质量状态共同保证 Worker 重启可恢复。
7. C `reject` 或第二次 `retry`：记录诊断，将 intent 标记 failed，不上传/关联候选；通过终态结果结束 Temporal Activity，避免把内容拒绝误当成 Provider 异常而重复执行。

### Temporal 终态处理

定义一个 Core→Workflow 可识别的质量结果：

- 首次内容 `retry`：`ProcessMediaActivity` 返回 `{status:"quality_retry", quality_retry_count:1}` 和 nil error；`MediaWorkflow` 最多再执行一次 Activity。
- 内容 `reject`/耗尽：`ProcessMediaActivity` 返回 `{status:"failed", quality_verdict:"reject"}` 和 nil error，Temporal 不再按 Activity retry；媒体 intent 已是 failed。
- Provider、数据库、对象存储等基础设施错误：继续返回 error，让现有 Activity retry/失败投影生效。
- C `skipped`：视为成功交付，Activity 返回 completed 结果，但 diagnostic 中保留 skipped 原因。

`MediaWorkflow` 在收到 `quality_retry` 后调用 `control.waitUntilResumed`，再执行一次 `ProcessMediaActivity`；第二次只能得到 completed/failed 或基础设施错误，不能再循环。实现不得用内存计数器代替 `quality_retry_count` 持久化字段。

## 幂等、崩溃和并发

- `provider_job_id` 持久化仍是外部提交边界；同一 job 的 Activity retry 只 poll，不二次 submit。
- C retry 不创建第二个聊天/动态占位；目标引用只在 pass/skipped 的唯一 ready asset 上更新。
- 质量重试执行的稳定身份由原 intent ID + `quality_retry_count=1` 派生；重复 workflow/activity replay 发现已有 count/guidance 时复用而非再创建。
- C verdict 写入、quality count 更新和 provider job 清空使用 `WHERE id=$1 AND provider_job_id=$2 AND status='running'` 等条件更新；stale worker 不能清除新一轮状态。
- 上传前的候选不进入对象存储；reject/retry 不产生需要清理的用户可见资产。上传后 DB 失败沿用现有 stable object key/result replay。
- ready asset 仍是最终 replay boundary；重复完成只重新应用现有 `publishMediaAsset` 幂等投影。

## 诊断与用户可见状态

- 每个 C verdict 写 `media.quality.acceptance` diagnostic event，包含 intent/target 关联、候选摘要、verdict、quality retry count、有限 violations、检查时间和 `skipped` reason；不保存图片 data URL。
- 继续使用现有 `diagnostic_model_runs` 记录 B/C Provider 调用；通过 redaction 过滤多模态二进制。
- 普通 UI 只看到现有 queued/processing/ready/failed/媒体占位状态；reject/retry exhausted 使用安全失败文案，保留原消息/动态正文，不显示内部 guidance JSON。
- Debug inspector 若已有 persona-scoped diagnostic projection，只扩展其 bounded media summary；禁止返回完整候选、Provider URL、文件路径或凭据。

## 兼容与回滚

- 新字段均为加法且有默认值；旧 intent 在没有 quality 状态时从 `quality_retry_count=0`、空 verdict 开始。
- 已有 provider job 的 retry 仍跳过 B；质量闸门只在候选下载后生效。
- 回滚代码不会要求数据库回滚；旧 worker 看到新增列默认值仍可完成原有生成，但不会理解新的 C 诊断。部署前应优先完成 Worker 版本切换和历史 workflow replay 检查。

## 主要风险

| 风险 | 防护 |
| --- | --- |
| C 视觉模型不可用导致媒体全部阻塞 | 用户已确认 fail-open；所有 C 基础设施错误转 `skipped` 后交付。 |
| C 把审美偏好当成硬违例 | 系统提示和 schema 明确只允许事实/安全检查；测试禁止审美 verdict。 |
| retry 重新创作了新故事 | A/上下文不可变；B 只接收有限 guidance；回归测试比较冻结概念摘要。 |
| Worker 崩溃后二次提交 Provider | provider job ID 和 quality state 在外部边界前后持久化；stale 条件更新。 |
| 诊断泄露候选图片 | data URL redaction、候选只在内存传给 C、诊断只存摘要。 |
| reject 触发 Temporal 重试并重复生成 | 内容终态与基础设施 error 分离；workflow 对终态返回 nil。 |
