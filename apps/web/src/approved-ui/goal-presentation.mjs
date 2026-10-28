// Core supplies newest-first evaluations. Goal/criteria identity owns every verdict.
export function goalAssessment(goal) {
  const version =
    Number.isSafeInteger(goal.criteria_version) && goal.criteria_version > 0
      ? goal.criteria_version
      : null;
  const evaluations = Array.isArray(goal.evaluations) ? goal.evaluations : [];
  let evaluation =
    version === null
      ? null
      : evaluations.find(
          (row) =>
            row.goal_id === goal.id &&
            row.criteria_version === version &&
            row.stage_id == null &&
            row.commitment_id == null,
        ) || null;
  const own = goal.execution?.last_evaluation;
  if (
    !evaluation &&
    version !== null &&
    own?.criteria_version === version &&
    own.evaluation_id &&
    (own.goal_id == null || own.goal_id === goal.id)
  )
    evaluation = own;
  const texts = Array.isArray(goal.success_criteria)
    ? goal.success_criteria
    : [];
  const ids = Array.isArray(goal.criterion_ids) ? goal.criterion_ids : [];
  const mappingKnown =
    ids.length === texts.length &&
    ids.every((id) => typeof id === "string" && id) &&
    new Set(ids).size === ids.length;
  const judgments = Array.isArray(evaluation?.judgments)
    ? evaluation.judgments
    : [];
  const criteria = texts.map((text, index) => {
    const id = typeof ids[index] === "string" && ids[index] ? ids[index] : null;
    const judgment =
      mappingKnown && id
        ? judgments.find((row) => row.criterion_id === id)
        : null;
    const verdict = ["satisfied", "not_satisfied", "unknown"].includes(
      judgment?.verdict,
    )
      ? judgment.verdict
      : "unknown";
    return {
      id,
      text: typeof text === "string" ? text : "条件内容尚未完整读到",
      verdict,
      reason: typeof judgment?.reason === "string" ? judgment.reason : "",
      evidenceRefs: Array.isArray(judgment?.evidence_refs)
        ? judgment.evidence_refs
        : [],
    };
  });
  const progress =
    typeof goal.progress === "number" &&
    Number.isFinite(goal.progress) &&
    goal.progress >= 0 &&
    goal.progress <= 1
      ? goal.progress
      : null;
  return {
    evaluation,
    criteria,
    progress,
    verifiedCount:
      evaluation && mappingKnown
        ? criteria.filter((row) => row.verdict === "satisfied").length
        : null,
  };
}
export const goalLabels = {
  candidate: "候选",
  active: "进行中",
  paused: "已暂停",
  completed: "已完成",
  cancelled: "已取消",
  abandoned: "已放下",
};
export const goalScopeLabel = (value) =>
  value === "relationship"
    ? "关系目标"
    : value === "general"
      ? "一般目标"
      : "目标范围尚未提供";
