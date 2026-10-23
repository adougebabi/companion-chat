import {
  domainName,
  errorMessage,
  stateLabel,
} from "../v3/domain-presentation.mjs";
export const joined = (...values) =>
  values.filter((value) => value != null && String(value).trim()).join(" · ");
export function scheduleProjection(item, schedule, format) {
  const activity =
      typeof item.activity === "string" ? item.activity.trim() : "",
    unspecified = !activity || activity === "未安排时间";
  return {
    time: unspecified ? "" : format(item.start_at, schedule?.timezone),
    text: unspecified ? "尚未安排具体活动" : activity,
    editable:
      !!item.id && !unspecified && Number.isFinite(Date.parse(item.start_at)),
    _source: item,
    _schedule: schedule,
  };
}
export function readableSummary(
  value,
  fallback = "结构化记录，详情中可查看完整内容",
  depth = 0,
) {
  if (depth > 5 || value == null) return fallback;
  if (typeof value === "string") {
    const text = value.trim();
    if (!text) return fallback;
    if (/^[\[{]/.test(text)) {
      try {
        return readableSummary(JSON.parse(text), fallback, depth + 1);
      } catch {
        return fallback;
      }
    }
    return text.replace(/\s+/g, " ").slice(0, 180);
  }
  if (Array.isArray(value)) {
    const text = value
      .slice(-4)
      .map((v) => readableSummary(v, "", depth + 1))
      .filter(Boolean)
      .join("；");
    return text || fallback;
  }
  if (typeof value === "object") {
    for (const key of [
      "summary",
      "text",
      "message",
      "content",
      "messages",
      "items",
      "result",
    ])
      if (value[key] != null) {
        const text = readableSummary(value[key], "", depth + 1);
        if (text) return text;
      }
  }
  return fallback;
}
export function diagnosticProjection(
  record,
  index,
  characters,
  now = Date.now(),
) {
  const d = record.data || {},
    fid = d.fluctlight_id || d.metrics?.fluctlight_id,
    character =
      characters.find((c) => c.id === fid)?.name ||
      (fid ? "角色名称未读取" : "系统调用"),
    start = Date.parse(d.started_at || d.queued_at || record.occurred_at) || 0,
    queued = Date.parse(d.queued_at) || start,
    completed = Date.parse(d.completed_at || ""),
    pending = ["queued", "running", "pending", "started"].includes(d.status),
    status = ["failed", "cancelled", "timeout", "dead_letter"].includes(
      d.status,
    )
      ? "error"
      : pending
        ? "pending"
        : ["completed", "ready", "succeeded"].includes(d.status)
          ? "success"
          : "unknown",
    label = domainName(
      d.scenario || d.binding_role || d.role,
      index ? "媒体生成" : "模型调用",
    );
  const rawInput =
      typeof d.prompt === "string"
        ? d.prompt
        : JSON.stringify(d.prompt || {}, null, 2),
    rawResponse =
      typeof d.response === "string"
        ? d.response
        : JSON.stringify(d.response || {}, null, 2),
    latency = Number(d.latency_ms ?? d.metrics?.latency_ms),
    duration =
      Number.isFinite(latency) && latency >= 0
        ? Math.round(latency / 1000)
        : Number.isFinite(completed) && start
          ? Math.max(0, Math.round((completed - start) / 1000))
          : null,
    elapsed = queued ? Math.max(0, Math.floor((now - queued) / 1000)) : null;
  return {
    id: record.ref,
    kind: index ? "媒体调用" : "模型调用",
    label,
    character,
    status,
    start,
    duration,
    durationLabel: pending
      ? elapsed == null
        ? "等待时间待确认"
        : `已等待 ${elapsed} 秒`
      : duration == null
        ? "耗时未记录"
        : `${duration} 秒`,
    input: rawInput,
    response: rawResponse,
    summary:
      status === "error"
        ? errorMessage(
            { code: d.error_code, message: d.error },
            "本次调用未完成，可查看原记录。",
          )
        : pending
          ? joined(
              stateLabel(d.status),
              elapsed == null ? "" : `已等待 ${elapsed} 秒`,
            )
          : readableSummary(d.response, label + "已完成，详情中可查看结果"),
    reason: d.error_code
      ? errorMessage({ code: d.error_code }, "本次调用未完成，可查看原记录。")
      : "",
    category: d.error_code ? "执行失败" : "",
    code: d.error_code || "",
    correlationId: d.correlation_id || record.ref,
    model: d.model_id || "",
    _record: record,
    media: index
      ? {
          type:
            { image: "图片", audio: "语音", video: "视频" }[d.kind] || "图片",
          workflow: d.workflow_id || "",
          prompt:
            typeof d.provider_prompt === "string"
              ? d.provider_prompt
              : JSON.stringify(d.provider_prompt || d.prompt || {}, null, 2),
          negativePrompt: d.negative_prompt || "",
          params: d.parameters || {},
          output: readableSummary(d.result, "尚未记录可读输出摘要"),
          assetId: d.result?.asset_id,
        }
      : null,
  };
}
export function pendingIssues(rows, characters) {
  const result = new Map();
  for (const row of rows) {
    const kind = row.intent_type || row.title || row.kind,
      key =
        kind === "wardrobe.initialize"
          ? kind + ":" + row.fluctlight_id
          : row.source_id || row.id,
      previous = result.get(key);
    if (
      previous &&
      Date.parse(previous._pending.created_at || 0) >
        Date.parse(row.created_at || 0)
    )
      continue;
    const name = characters.find((c) => c.id === row.fluctlight_id)?.name;
    result.set(key, {
      id: row.id,
      label: joined(name, domainName(kind, "待处理事项")),
      reason: joined(
        stateLabel(row.status),
        row.status === "failed" || row.status === "dead_letter"
          ? "请查看原任务并恢复"
          : "等待原任务继续处理",
      ),
      _pending: row,
    });
  }
  return [...result.values()];
}
