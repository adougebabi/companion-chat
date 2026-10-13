export function configObject(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? JSON.parse(JSON.stringify(value)) as Record<string, unknown> : {};
}

export function parseWorkflowTemplate(raw: string): Record<string, unknown> {
  const parse = (source: string) => {
    const value: unknown = JSON.parse(source);
    if (!value || typeof value !== "object" || Array.isArray(value) || !Object.keys(value).length) throw new Error("工作流必须是非空 JSON 对象。");
    return value as Record<string, unknown>;
  };
  try { return parse(raw.trim()); }
  catch (cause) {
    const normalized = raw.trim().replace(/([:\[,\s]*)(\{\{[A-Za-z0-9_.-]+\}\})(?=\s*[,}\]])/g, '$1"$2"');
    if (normalized === raw.trim()) throw cause;
    return parse(normalized);
  }
}

export function mediaSettingsPatch(current: unknown, baseUrl: string, workflow: Record<string, unknown>, visualIdentity?: Record<string, unknown>, clearVisual = false): Record<string, unknown> {
  const result: Record<string, unknown> = { ...configObject(current), baseUrl: baseUrl.trim(), workflow };
  if (visualIdentity !== undefined) result.visual_identity_workflow = visualIdentity;
  if (clearVisual) delete result.visual_identity_workflow;
  return result;
}

export function autonomySettingsPatch(current: unknown, input: { mode: string; actions: string[]; budget: number; topicSuppression?: number }): Record<string, unknown> {
  return {
    ...configObject(current), mode: input.mode, allowed_actions: [...input.actions], budget_remaining: input.budget,
    ...(input.topicSuppression === undefined ? {} : { topic_suppression_seconds: input.topicSuppression }),
  };
}

export function workflowSummary(value: unknown): { nodes: number; placeholders: string[] } {
  let nodes = 0;
  const visit = (entry: unknown) => {
    if (!entry || typeof entry !== "object") return;
    if (!Array.isArray(entry) && typeof (entry as Record<string, unknown>).class_type === "string") nodes++;
    Object.values(entry).forEach(visit);
  };
  visit(value);
  return { nodes, placeholders: [...new Set((JSON.stringify(value) ?? "").match(/\{\{[A-Za-z0-9_.-]+\}\}/g) ?? [])] };
}
