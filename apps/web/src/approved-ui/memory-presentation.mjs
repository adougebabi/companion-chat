const operationLabels = {
  create: "创建记录",
  revise: "纠正正文",
  forget: "遗忘记录",
  rollback: "历史补偿恢复",
};
/** Support refs may contain earlier Owner notes; never present those as this revision's reason. */
export function memoryRevisionReason(row) {
  const prefix =
    typeof row.actor_id === "string" && row.actor_id
      ? "owner:" + row.actor_id + ":"
      : null;
  if (typeof row.semantic_reason === "string" && row.semantic_reason) {
    if (prefix && row.semantic_reason.startsWith(prefix))
      return row.semantic_reason.slice(prefix.length);
    if (!row.semantic_reason.startsWith("owner:")) return row.semantic_reason;
  }
  const refs = Array.isArray(row.evidence_refs) ? row.evidence_refs : [];
  if (
    refs.length === 1 &&
    prefix &&
    typeof refs[0] === "string" &&
    refs[0].startsWith(prefix)
  )
    return refs[0].slice(prefix.length);
  return operationLabels[row.operation] || "历史修订";
}
