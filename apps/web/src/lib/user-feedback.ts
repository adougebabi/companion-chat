const errorMessages: Record<string, string> = {
  agent_run_failed: "回复未能完成，可以重试。",
  provider_request_failed: "模型服务暂时无法连接，请检查模型服务或稍后重试。",
  request_failed: "服务暂时无法连接，请检查服务地址或稍后重试。",
  request_timeout: "等待服务响应超时，请稍后重试。",
  request_cancelled: "此次请求已取消。",
  agent_partial_committed: "部分操作已完成，当前回复不能直接重试。请查看诊断后再继续。",
  workflow_runtime_unavailable: "工作流服务暂时不可用，其他诊断仍可查看。",
  unavailable: "服务暂时不可用，请稍后重试。",
  current_facts_stale: "当前资料已变化，请读取最新版本后核对草稿。",
  actor_context_stale: "背景资料已变化，草稿已保留，请读取最新版本。",
  goal_evaluation_required_evidence_budget_exceeded: "本次评估所需资料超出处理预算，请查看诊断。",
};
export function userError(value: unknown, fallback = "操作未完成，请稍后重试。") {
  const message = value instanceof Error ? value.message : String(value ?? "");
  const code = value && typeof value === "object" && "code" in value ? String(value.code) : "";
  if (errorMessages[code]) return errorMessages[code];
  const known = Object.keys(errorMessages).find(key => new RegExp(`(^|[^a-z_])${key}([^a-z_]|$)`).test(message));
  if (known) return errorMessages[known];
  return /[\u3400-\u9fff]/.test(message) ? message : fallback;
}
const statusLabels: Record<string, string> = {
  active: "运行中", running: "执行中", paused: "已暂停", candidate: "候选", completed: "已完成", succeeded: "已完成", failed: "失败", cancelled: "已取消", abandoned: "已放弃", pending: "待处理", queued: "排队中", waiting: "等待中", timeout: "超时", unavailable: "不可用", available: "可用", unknown: "未知", apply: "应用规划", closed: "已关闭", no_change: "保留当前规划", original_used: "采用原方案", not_applied: "未应用", use_original: "保留原方案", ok: "成功", info: "信息", error: "错误", warning: "注意", retry_accepted: "已通过复核",
  terminated: "已终止", continued_as_new: "转入新运行", timed_out: "超时", original: "原方案", guarded: "置信度筛选", direct: "直接表达",
};
export function displayStatus(value: unknown) { const code = String(value ?? ""); return statusLabels[code] ?? (code || "未知"); }
export function shortId(value: unknown) { const text = String(value ?? ""); return text.length > 20 ? `${text.slice(0, 10)}…${text.slice(-6)}` : text; }
