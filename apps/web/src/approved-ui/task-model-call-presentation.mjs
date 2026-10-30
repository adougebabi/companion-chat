// Read-only projections. Never infer a physical call's sequence from list order.
export function recordedCallValue(call, key) {
  return call[key] ?? call.metrics?.[key];
}

export function callValueLabel(value, missing) {
  if (value === undefined || value === null || value === '') return missing;
  return typeof value === 'string' ? value : JSON.stringify(value);
}

export function callDurationLabel(call) {
  const latency = recordedCallValue(call, 'latency_ms');
  if (typeof latency === 'number' && Number.isFinite(latency) && latency >= 0)
    return duration(latency) + '（记录耗时）';
  const instant = value => typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(value) ? Date.parse(value) : NaN;
  const start = instant(recordedCallValue(call, 'started_at'));
  const end = instant(recordedCallValue(call, 'completed_at'));
  if (Number.isFinite(start) && Number.isFinite(end) && end >= start)
    return duration(end - start) + '（起止时间）';
  return '耗时未记录';
}

function duration(ms) {
  return ms < 1000 ? `${ms} 毫秒` : `${ms / 1000} 秒`;
}

export function callStageLabel(call) {
  const stage = recordedCallValue(call, 'stage');
  const labels = {tool_request:'工具请求',tool_response:'工具响应',final_response:'最终响应',pending:'等待中',failed:'失败',cancelled:'已取消',timeout:'超时',unknown:'阶段未知'};
  return (typeof stage === 'string' && Object.hasOwn(labels, stage) ? labels[stage] : callValueLabel(stage, '阶段未记录'));
}

export function loadedCallSummary(records, hasMore) {
  const statuses = records.map(record => record.data?.status);
  const failed = statuses.filter(status => ['error','failed','timeout','dead_letter'].includes(status)).length;
  const ongoing = statuses.filter(status => ['queued','running','pending','started','in_progress'].includes(status)).length;
  return `已读取 ${records.length} 条模型调用记录 · 失败 ${failed} 条 · 进行中 ${ongoing} 条${hasMore ? '；还有记录未读取' : ''}`;
}
